//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/prometheus/client_golang/prometheus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

var (
	didNotMatchAnyRule = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "slo_exporter",
		Subsystem: "slo_event_producer",
		Name:      "events_not_matching_any_rule",
		Help:      "Total number of events not matching any SLO rule.",
	})

	evaluationDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "slo_exporter",
		Subsystem: "slo_event_producer",
		Name:      "evaluation_duration_seconds",
		Help:      "Histogram of event evaluation duration.",
		Buckets:   prometheus.ExponentialBuckets(0.0001, 5, 7),
	})
)

func init() {
	prometheus.MustRegister(didNotMatchAnyRule, evaluationDurationSeconds)
}

type requestEventEvaluator struct {
	rules []*evaluationRule
}

func (re *requestEventEvaluator) AddEvaluationRule(rule *evaluationRule) {
	re.rules = append(re.rules, rule)
}

func (re *requestEventEvaluator) Evaluate(event *producer.RequestEvent, outChan chan<- *SloEvent) {
	timer := prometheus.NewTimer(evaluationDurationSeconds)
	defer timer.ObserveDuration()
	matchedRulesCount := 0
	for _, rule := range re.rules {
		newSloEvent, matched := rule.evaluateEvent(event)
		if !matched {
			continue
		}
		matchedRulesCount++
		outChan <- newSloEvent
	}
	if matchedRulesCount == 0 {
		log.Warnf("event %v did not match any SLO rule", event)
		didNotMatchAnyRule.Inc()
	}
}
