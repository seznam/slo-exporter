//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
)

func newEvaluationRule(opts ruleOptions, logger logrus.FieldLogger) (*evaluationRule, error) {
	var failureConditions []operator
	for _, operatorOpts := range opts.FailureConditionsOptions {
		operator, err := newOperator(operatorOpts)
		if err != nil {
			return nil, err
		}
		failureConditions = append(failureConditions, operator)
	}
	return &evaluationRule{
		sloMatcher: event.SloClassification{
			Domain: opts.SloMatcher.Domain,
			App:    opts.SloMatcher.App,
			Class:  opts.SloMatcher.Class,
		},
		failureConditions:  failureConditions,
		additionalMetadata: opts.AdditionalMetadata,
		honorSloResult:     opts.HonorSloResult,
		logger:             logger,
	}, nil
}

type evaluationRule struct {
	sloMatcher         event.SloClassification
	failureConditions  []operator
	additionalMetadata stringmap.StringMap
	honorSloResult     bool
	logger             logrus.FieldLogger
}

func (er *evaluationRule) markEventResult(failed bool, newEvent *event.Slo) {
	if failed {
		newEvent.Result = event.Fail
	} else {
		newEvent.Result = event.Success
	}
}

// evaluateEvent and return bool on whether it is to be considered as failed
func (er *evaluationRule) evaluateEvent(newEvent *event.HttpRequest) bool {
	if er.honorSloResult && newEvent.SloResult != "" {
		return newEvent.SloResult != string(event.Success)
	}
	failed := false
	// Evaluate all criteria and if matches any, mark it as failed.
	for _, operator := range er.failureConditions {
		result, err := operator.Evaluate(newEvent)
		if err != nil {
			er.logger.Warnf("failed to evaluate operator %v on event %v: %v", operator, newEvent, err)
		}
		if result {
			failed = true
			break
		}
	}
	return failed
}

func (er *evaluationRule) processEvent(newEvent *event.HttpRequest) (*event.Slo, bool) {
	eventSloClassification := newEvent.GetSloClassification()
	if !newEvent.IsClassified() || eventSloClassification == nil {
		return nil, false
	}
	// Check if rule matches the newEvent
	if !er.sloMatcher.Matches(*eventSloClassification) {
		return nil, false
	}
	failed := er.evaluateEvent(newEvent)

	newSloEvent := &event.Slo{
		Key:      newEvent.EventKey,
		Occurred: newEvent.GetTimeOccurred(),
		Domain:   eventSloClassification.Domain,
		Class:    eventSloClassification.Class,
		App:      eventSloClassification.App,
		Metadata: er.additionalMetadata,
		Quantity: newEvent.Quantity,
	}
	er.markEventResult(failed, newSloEvent)
	er.logger.Debugf("generated SLO newEvent: %+v", newSloEvent)
	return newSloEvent, true

}
