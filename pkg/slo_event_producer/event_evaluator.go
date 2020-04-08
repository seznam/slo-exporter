//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
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
