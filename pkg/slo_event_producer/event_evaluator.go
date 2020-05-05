//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/prometheus_ingester"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"strconv"
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
		rules:        []*evaluationRule{},
		rulesOptions: config.Rules,
		logger:       logger,
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
	rules        []*evaluationRule
	rulesOptions []ruleOptions
	logger       logrus.FieldLogger
}

func (re *EventEvaluator) getMetricsFromRuleOptions() (*prometheus.GaugeVec, error) {
	possibleLabels := stringmap.StringMap{}

	metricsLabels := []stringmap.StringMap{}
	metricsValues := []float64{}

	// get possible labels and labels->value mappings
	for _, ruleConfig := range re.rulesOptions {
		if !ruleConfig.ExposeAsMetric {
			continue
		}
		labels := stringmap.StringMap{}
		for _, matchCond := range ruleConfig.MetadataMatcherConditionsOptions {
			if matchCond.Operator == equalToOperatorName {
				possibleLabels.AddKeys(matchCond.Key)
				labels[matchCond.Key] = matchCond.Value
			}
		}
		if len(labels) == 0 {
			return nil, fmt.Errorf("rule marked as to be exposed as Prometheus metrics does not contain any metadata_matcher with '%s' operator: %v", equalToOperatorName, ruleConfig)
		}

		possibleLabels.AddKeys(ruleConfig.AdditionalMetadata.Keys()...)
		labels = labels.Merge(ruleConfig.AdditionalMetadata)

		possibleLabels.AddKeys("operator")
		var found bool
		for _, failCond := range ruleConfig.FailureConditionsOptions {
			if failCond.Key != prometheus_ingester.MetadataValueKey {
				continue
			}
			found = true
			metricsLabels = append(metricsLabels, labels.Merge(stringmap.StringMap{"operator": failCond.Operator}))
			failCondValue, err := strconv.ParseFloat(failCond.Value, 64)
			if err != nil {
				return nil, fmt.Errorf("unable to parse failure_condition value as a float: %v", failCond)
			}
			metricsValues = append(metricsValues, failCondValue)
		}
		if !found {
			return nil, fmt.Errorf("rule marked as to be exposed as Prometheus metric does not contain any failure_condition which matches Prometheus query result key ('%s'): %v", prometheus_ingester.MetadataValueKey, ruleConfig)
		}
	}
	// create metrics
	thresholdsGaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        metricFromRulesName,
			Help:        "Threshold exposed based on information from slo_event_producer's slo_rules configuration",
			ConstLabels: prometheus.Labels{},
		},
		possibleLabels.Keys())
	for i, value := range metricsValues {
		m, err := thresholdsGaugeVec.GetMetricWith(prometheus.Labels(possibleLabels.Merge(metricsLabels[i])))
		if err != nil {
			return nil, err
		}
		m.Set(value)
	}
	return thresholdsGaugeVec, nil
}

func (re *EventEvaluator) registerMetrics(wrappedRegistry prometheus.Registerer) error {
	metric, err := re.getMetricsFromRuleOptions()
	if err != nil {
		return err
	}
	err = wrappedRegistry.Register(metric)
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
