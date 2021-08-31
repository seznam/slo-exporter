//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"fmt"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/stringmap"
	"github.com/sirupsen/logrus"
	"regexp"
)

type sloClassificationMatcher struct {
	domainRegexp *regexp.Regexp
	classRegexp  *regexp.Regexp
	appRegexp    *regexp.Regexp
}

func (s *sloClassificationMatcher) matchesSloClassification(c event.SloClassification) bool {
	if s.domainRegexp != nil && !s.domainRegexp.MatchString(c.Domain) {
		return false
	}
	if s.classRegexp != nil && !s.classRegexp.MatchString(c.Class) {
		return false
	}
	if s.appRegexp != nil && !s.appRegexp.MatchString(c.App) {
		return false
	}
	return true
}

func getOperators(operatorOpts []operatorOptions) ([]operator, error) {
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
		matcherConditions []operator
		failureConditions []operator
		err               error
	)
	if matcherConditions, err = getOperators(opts.MetadataMatcherConditionsOptions); err != nil {
		return nil, err
	}
	if failureConditions, err = getOperators(opts.FailureConditionsOptions); err != nil {
		return nil, err
	}
	sloMatcher := sloClassificationMatcher{}
	if opts.SloMatcher.DomainRegexp != "" {
		r, err := regexp.Compile(opts.SloMatcher.DomainRegexp)
		if err != nil {
			return nil, fmt.Errorf("invalid domain matcher regexp: %w", err)
		}
		sloMatcher.domainRegexp = r
	}
	if opts.SloMatcher.ClassRegexp != "" {
		r, err := regexp.Compile(opts.SloMatcher.ClassRegexp)
		if err != nil {
			return nil, fmt.Errorf("invalid class matcher regexp: %w", err)
		}
		sloMatcher.classRegexp = r
	}
	if opts.SloMatcher.AppRegexp != "" {
		r, err := regexp.Compile(opts.SloMatcher.AppRegexp)
		if err != nil {
			return nil, fmt.Errorf("invalid app matcher regexp: %w", err)
		}
		sloMatcher.appRegexp = r
	}
	return &evaluationRule{
		sloMatcher:         sloMatcher,
		metadataMatcher:    matcherConditions,
		failureConditions:  failureConditions,
		additionalMetadata: opts.AdditionalMetadata,
		logger:             logger,
	}, nil
}

type evaluationRule struct {
	sloMatcher         sloClassificationMatcher
	metadataMatcher    []operator
	failureConditions  []operator
	additionalMetadata stringmap.StringMap
	logger             logrus.FieldLogger
}

// evaluateEvent and return bool on whether it is to be considered as failed
func (er *evaluationRule) evaluateEvent(newEvent event.Raw) event.Result {
	result := event.Success
	// Evaluate all criteria and if matches any, mark it as failed.
	for _, operator := range er.failureConditions {
		matches, err := operator.Evaluate(newEvent)
		if err != nil {
			er.logger.WithError(err).WithField("event", newEvent).Warnf("failed to evaluate operator %v", operator)
		}
		if matches {
			result = event.Fail
			break
		}
	}
	return result
}

func (er *evaluationRule) processEvent(newEvent event.Raw) (event.Slo, bool) {
	eventSloClassification := newEvent.SloClassification()
	if !newEvent.IsClassified() {
		return nil, false
	}
	// Check if rule matches the newEvent
	if !er.sloMatcher.matchesSloClassification(eventSloClassification) {
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

	result := er.evaluateEvent(newEvent)

	newSloEvent := event.NewSlo(newEvent.EventKey(), newEvent.Quantity(), result, eventSloClassification, er.additionalMetadata)
	er.logger.WithField("event", newSloEvent).Debug("generated new slo event")
	return newSloEvent, true

}
