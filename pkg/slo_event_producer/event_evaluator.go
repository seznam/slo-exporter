//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/hashicorp/go-multierror"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
)

type EventEvaluator interface {
	Evaluate(event *event.HttpRequest, outChan chan<- *event.Slo)
	AddEvaluationRule(*evaluationRule)
	PossibleMetadataKeys() []string
}

func configFromFile(path string) (*rulesConfig, error) {
	var config rulesConfig
	if _, err := config.loadFromFile(path); err != nil {
		return nil, err
	}
	return &config, nil
}

func NewEventEvaluatorFromConfigFiles(paths []string) (EventEvaluator, error) {
	var config rulesConfig
	for _, path := range paths {
		tmpConfig, err := configFromFile(path)
		if err != nil {
			return nil, err
		}
		config.Rules = append(config.Rules, tmpConfig.Rules...)
	}
	evaluator, err := NewEventEvaluatorFromConfig(&config)
	if err != nil {
		return nil, err
	}
	return evaluator, nil
}

func NewEventEvaluatorFromConfig(config *rulesConfig) (EventEvaluator, error) {
	var configurationErrors error
	evaluator := requestEventEvaluator{}
	for _, ruleOpts := range config.Rules {
		rule, err := newEvaluationRule(ruleOpts)
		if err != nil {
			log.Errorf("invalid rule configuration: %v", err)
			configurationErrors = multierror.Append(configurationErrors, err)
			continue
		}
		evaluator.AddEvaluationRule(rule)
	}
	return &evaluator, configurationErrors
}
