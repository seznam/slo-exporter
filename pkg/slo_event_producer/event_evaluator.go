//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
)

const (
	metricFromRulesName = "slo_rules_threshold"
)

func configFromFile(path string) (*rulesConfig, error) {
	var config rulesConfig
	if _, err := config.loadFromFile(path); err != nil {
		return nil, err
	}
	return &config, nil
}

func NewEventEvaluatorFromConfigFiles(paths []string, logger logrus.FieldLogger) (*EventEvaluator, error) {
	var config rulesConfig
	for _, path := range paths {
		tmpConfig, err := configFromFile(path)
		if err != nil {
			return nil, err
		}
		config.Rules = append(config.Rules, tmpConfig.Rules...)
	}
	evaluator, err := NewEventEvaluatorFromConfig(&config, logger)
	if err != nil {
		return nil, err
	}
	return evaluator, nil
}

func NewEventEvaluatorFromConfig(config *rulesConfig, logger logrus.FieldLogger) (*EventEvaluator, error) {
	var configurationErrors error
	evaluator := EventEvaluator{
		rules:  []*evaluationRule{},
		logger: logger,
	}
	for _, ruleOpts := range config.Rules {
		rule, err := newEvaluationRule(ruleOpts, logger)
		if err != nil {
			configurationErrors = multierror.Append(configurationErrors, err)
			continue
		}
		evaluator.AddEvaluationRule(rule)
	}
	return &evaluator, configurationErrors
}

type EventEvaluator struct {
	rules  []*evaluationRule
	logger logrus.FieldLogger
}

func (re *EventEvaluator) getMetricsFromRuleOptions() (metrics []metric, possibleLabels []string, err error) {
	metrics = []metric{}
	possibleLabelsMap := stringmap.StringMap{}

	for _, rule := range re.rules {
		for _, failureCondition := range rule.failureConditions {
			exposableFailureCondition, ok := failureCondition.(exposableOperator)
			if !ok || !exposableFailureCondition.Expose() {
				continue
			}
			metricFromFailureCondition := exposableFailureCondition.Metric()
			metricFromFailureCondition.Labels = metricFromFailureCondition.Labels.Merge(metricFromFailureCondition.Labels)

			metricFromFailureCondition.Labels = metricFromFailureCondition.Labels.Merge(rule.additionalMetadata)

			for _, metadataMatchingCondition := range rule.metadataMatcher {
				metadataMatchingCondition, ok := metadataMatchingCondition.(labelsExposableOperator)
				if !ok {
					continue
				}
				metricFromFailureCondition.Labels = metricFromFailureCondition.Labels.Merge(metadataMatchingCondition.Labels())
			}
			metrics = append(metrics, metricFromFailureCondition)
			possibleLabelsMap.AddKeys(metricFromFailureCondition.Labels.Keys()...)
		}
	}

	return metrics, possibleLabelsMap.Keys(), nil
}

func (re *EventEvaluator) registerMetrics(wrappedRegistry prometheus.Registerer) error {
	metrics, possibleLabels, err := re.getMetricsFromRuleOptions()
	if err != nil {
		return err
	}

	thresholdsGaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        metricFromRulesName,
			Help:        "Threshold exposed based on information from slo_event_producer's slo_rules configuration",
			ConstLabels: prometheus.Labels{},
		},
		possibleLabels)

	for _, metric := range metrics {
		labels := stringmap.StringMap{}
		labels.AddKeys(possibleLabels...)
		m, err := thresholdsGaugeVec.GetMetricWith(prometheus.Labels(labels.Merge(metric.Labels)))
		if err != nil {
			return err
		}
		m.Set(metric.Value)
	}

	err = wrappedRegistry.Register(thresholdsGaugeVec)
	if err != nil {
		return err
	}
	return nil
}

func (re *EventEvaluator) AddEvaluationRule(rule *evaluationRule) {
	re.rules = append(re.rules, rule)
}

func (re *EventEvaluator) Evaluate(newEvent *event.HttpRequest, outChan chan<- *event.Slo) {
	if !newEvent.IsClassified() {
		unclassifiedEventsTotal.Inc()
		re.logger.Warnf("dropping event %s with no classification", newEvent)
		return
	}
	timer := prometheus.NewTimer(evaluationDurationSeconds)
	defer timer.ObserveDuration()
	matchedRulesCount := 0
	for _, rule := range re.rules {
		newSloEvent, matched := rule.processEvent(newEvent)
		if !matched {
			continue
		}
		matchedRulesCount++
		outChan <- newSloEvent
	}
	if matchedRulesCount == 0 {
		re.logger.Warnf("event %+v did not match any SLO rule", newEvent)
		didNotMatchAnyRule.Inc()
	}
}
