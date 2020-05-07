//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
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
		//possibleLabels.AddKeys("operator")
		for _, failCond := range ruleConfig.FailureConditionsOptions {
			if !failCond.ExposeAsMetric {
				continue
			}
			ruleMetricLabels := stringmap.StringMap{}
			ruleMetricLabels = ruleMetricLabels.Merge(stringmap.StringMap{"operator": failCond.Operator})

			for _, matchCond := range ruleConfig.MetadataMatcherConditionsOptions {
				op, err := newOperator(exposableOperatorOptions{matchCond, false})
				if err != nil {
					return nil, fmt.Errorf("unable to determine whether given operator '%v' is of equality type: %v", matchCond, err)
				}
				if op.IsEqualityOperator() {
					ruleMetricLabels[matchCond.Key] = matchCond.Value
				}
			}
			ruleMetricLabels = ruleMetricLabels.Merge(ruleConfig.AdditionalMetadata)

			failCondValue, err := strconv.ParseFloat(failCond.Value, 64)
			if err != nil {
				return nil, fmt.Errorf("unable to parse failure_condition value as a float: %v", failCond)
			}
			metricsValues = append(metricsValues, failCondValue)
			metricsLabels = append(metricsLabels, ruleMetricLabels)

			possibleLabels.AddKeys(ruleMetricLabels.Keys()...)
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
