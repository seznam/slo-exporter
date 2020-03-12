//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/prometheus/client_golang/prometheus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
)

var (
	unclassifiedEventsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "slo_exporter",
		Subsystem: "slo_event_producer",
		Name:      "unclassified_events_total",
		Help:      "Total number of dropped events without classification.",
	})
)

func init() {
	prometheus.MustRegister(unclassifiedEventsTotal)
}

func newEvaluationRule(opts ruleOptions) (*evaluationRule, error) {
	var failureCriteria []criterium
	for _, criteriumOpts := range opts.FailureCriteriaOptions {
		criterium, err := newCriterium(criteriumOpts)
		if err != nil {
			return nil, err
		}
		failureCriteria = append(failureCriteria, criterium)
	}
	return &evaluationRule{
		sloMatcher: event.SloClassification{
			Domain: opts.SloMatcher.Domain,
			App:    opts.SloMatcher.App,
			Class:  opts.SloMatcher.Class,
		},
		failureCriteria:    failureCriteria,
		additionalMetadata: opts.AdditionalMetadata,
		honorSloResult:     opts.HonorSloResult,
	}, nil
}

type evaluationRule struct {
	sloMatcher         event.SloClassification
	failureCriteria    []criterium
	additionalMetadata stringmap.StringMap
	honorSloResult     bool
}

func (er *evaluationRule) PossibleMetadataKeys() []string {
	return er.additionalMetadata.Keys()
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
	for _, criterium := range er.failureCriteria {
		log.Tracef("evaluating criterium %+v", criterium)
		if criterium.Evaluate(newEvent) {
			failed = true
			break
		}
	}
	return failed
}

func (er *evaluationRule) proccessEvent(newEvent *event.HttpRequest) (*event.Slo, bool) {
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
		Key:      newEvent.GetEventKey(),
		Occurred: newEvent.GetTimeOccurred(),
		Domain:   eventSloClassification.Domain,
		Class:    eventSloClassification.Class,
		App:      eventSloClassification.App,
		Metadata: er.additionalMetadata,
	}
	er.markEventResult(failed, newSloEvent)
	log.Debugf("generated SLO newEvent: %+v", newSloEvent)
	return newSloEvent, true

}
