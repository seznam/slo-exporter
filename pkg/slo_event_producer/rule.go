//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
)

func getOperators(operatorOpts []exposableOperatorOptions) ([]operator, error) {
	var operators = make([]operator, len(operatorOpts))
	for i, operatorOpts := range operatorOpts {
		operator, err := newOperator(operatorOpts)
		if err != nil {
			return nil, err
		}
		operators[i] = operator
	}
	return operators, nil
}

func newEvaluationRule(opts ruleOptions, logger logrus.FieldLogger) (*evaluationRule, error) {
	var (
		matcherConditions            []operator
		failureConditions            []operator
		metadataMatchersOperatorOpts []exposableOperatorOptions
		err                          error
	)
	for _, op := range opts.MetadataMatcherConditionsOptions {
		metadataMatchersOperatorOpts = append(
			metadataMatchersOperatorOpts,
			exposableOperatorOptions{
				operatorOptions: op,
				ExposeAsMetric:  false,
			},
		)
	}
	if matcherConditions, err = getOperators(metadataMatchersOperatorOpts); err != nil {
		return nil, err
	}
	if failureConditions, err = getOperators(opts.FailureConditionsOptions); err != nil {
		return nil, err
	}
	return &evaluationRule{
		sloMatcher: event.SloClassification{
			Domain: opts.SloMatcher.Domain,
			App:    opts.SloMatcher.App,
			Class:  opts.SloMatcher.Class,
		},
		metadataMatcher:    matcherConditions,
		failureConditions:  failureConditions,
		additionalMetadata: opts.AdditionalMetadata,
		honorSloResult:     opts.HonorSloResult,
		logger:             logger,
	}, nil
}

type evaluationRule struct {
	sloMatcher         event.SloClassification
	metadataMatcher    []operator
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
			er.logger.WithError(err).WithField("event", newEvent).Warnf("failed to evaluate operator %v", operator)
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
	var (
		matches bool
		err     error
	)
	for _, matcherOperator := range er.metadataMatcher {
		matches, err = matcherOperator.Evaluate(newEvent)
		if err != nil {
			er.logger.WithError(err).WithField("event", newEvent).Warnf("failed to evaluate metadataMatcher operator %v", matcherOperator)
			return nil, false
		}
		if !matches {
			return nil, false
		}
	}

	failed := er.evaluateEvent(newEvent)

	newSloEvent := &event.Slo{
		Key:      newEvent.EventKey(),
		Occurred: newEvent.GetTimeOccurred(),
		Domain:   eventSloClassification.Domain,
		Class:    eventSloClassification.Class,
		App:      eventSloClassification.App,
		Metadata: er.additionalMetadata,
		Quantity: newEvent.Quantity,
	}
	er.markEventResult(failed, newSloEvent)
	er.logger.WithField("event", newSloEvent).Debug("generated new slo event")
	return newSloEvent, true

}
