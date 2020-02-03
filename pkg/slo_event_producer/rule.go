//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/prometheus/client_golang/prometheus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

const (
	eventKeyMetadataKey = "event_key"
)

type eventMetadata map[string]string

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

func (e *eventMetadata) matches(otherMetadata eventMetadata) bool {
	for k, v := range *e {
		otherV, ok := otherMetadata[k]
		if !ok {
			return false
		}
		if otherV != v {
			return false
		}
	}
	return true
}

func mergeMetadata(a, b map[string]string) map[string]string {
	newMetadata := map[string]string{}
	for k, v := range a {
		newMetadata[k] = v
	}
	for k, v := range b {
		newMetadata[k] = v
	}
	return newMetadata
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
		matcher:            opts.Matcher,
		failureCriteria:    failureCriteria,
		additionalMetadata: opts.AdditionalMetadata,
	}, nil
}

type evaluationRule struct {
	matcher            eventMetadata
	failureCriteria    []criterium
	additionalMetadata eventMetadata
}

func (er *evaluationRule) PossibleMetadataKeys() []string {
	sloClassification := producer.SloClassification{}
	resultingMetadata := mergeMetadata(sloClassification.GetMap(), er.additionalMetadata)
	resultingMetadata = mergeMetadata(resultingMetadata, map[string]string{eventKeyMetadataKey: ""})
	var keys []string
	for k := range resultingMetadata {
		keys = append(keys, k)
	}
	return keys
}

func (er *evaluationRule) markEventResult(failed bool, event *SloEvent) {
	if failed {
		event.Result = SloEventResultFail
	} else {
		event.Result = SloEventResultSuccess
	}
}

func (er *evaluationRule) setEventKey(event *producer.RequestEvent, newEvent *SloEvent) {
	newEvent.SloMetadata[eventKeyMetadataKey] = event.GetEventKey()
}

func (er *evaluationRule) evaluateEvent(event *producer.RequestEvent) (*SloEvent, bool) {
	eventMetadata := event.GetSloMetadata()
	if !event.IsClassified() || eventMetadata == nil {
		unclassifiedEventsTotal.Inc()
		log.Warnf("dropping event %v with no classification", event)
		return nil, false
	}
	// Check if rule matches the event
	if er.matcher != nil {
		if !er.matcher.matches(*eventMetadata) {
			return nil, false
		}
	}
	// Evaluate all criteria and if matches any, mark it as failed.
	failed := false
	for _, criterium := range er.failureCriteria {
		log.Tracef("evaluating criterium %v", criterium)
		if criterium.Evaluate(event) {
			failed = true
			break
		}
	}
	finalMetadata := map[string]string{}
	if er.additionalMetadata != nil {
		finalMetadata = mergeMetadata(er.additionalMetadata, *eventMetadata)
	} else {
		finalMetadata = *eventMetadata
	}

	newSloEvent := &SloEvent{TimeOccurred: event.GetTimeOccurred(), SloMetadata: finalMetadata}
	er.markEventResult(failed, newSloEvent)
	er.setEventKey(event, newSloEvent)
	log.Debugf("generated SLO event: %v", newSloEvent)
	return newSloEvent, true

}
