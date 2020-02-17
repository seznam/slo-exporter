//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/prometheus/client_golang/prometheus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
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
		sloMatcher: producer.SloClassification{
			Domain: opts.SloMatcher.Domain,
			App:    opts.SloMatcher.App,
			Class:  opts.SloMatcher.Class,
		},
		failureCriteria:    failureCriteria,
		additionalMetadata: opts.AdditionalMetadata,
	}, nil
}

type evaluationRule struct {
	sloMatcher         producer.SloClassification
	failureCriteria    []criterium
	additionalMetadata stringmap.StringMap
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

func (er *evaluationRule) evaluateEvent(newEvent *producer.RequestEvent) (*event.Slo, bool) {
	eventSloClassification := newEvent.GetSloClassification()
	if !newEvent.IsClassified() || eventSloClassification == nil {
		unclassifiedEventsTotal.Inc()
		log.Warnf("dropping event %+v with no classification", newEvent)
		return nil, false
	}
	// Check if rule matches the newEvent
	if !er.sloMatcher.Matches(*eventSloClassification) {
		return nil, false
	}
	// Evaluate all criteria and if matches any, mark it as failed.
	failed := false
	for _, criterium := range er.failureCriteria {
		log.Tracef("evaluating criterium %+v", criterium)
		if criterium.Evaluate(newEvent) {
			failed = true
			break
		}
	}

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
