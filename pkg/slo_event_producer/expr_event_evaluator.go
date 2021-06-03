package slo_event_producer

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/sirupsen/logrus"
)

func NewExprEventEvaluatorFromConfigFiles(paths []string, logger logrus.FieldLogger) (*ExprEventEvaluator, error) {
	return &ExprEventEvaluator{}, nil
}

func (re *ExprEventEvaluator) registerMetrics(wrappedRegistry prometheus.Registerer) error {
	return nil
}

func (re *ExprEventEvaluator) Evaluate(newEvent *event.Raw, outChan chan<- *event.Slo) {
	//	if !newEvent.IsClassified() {
	//		unclassifiedEventsTotal.Inc()
	//		re.logger.Warnf("dropping event %s with no classification", newEvent)
	//		return
	//	}
	//
	//	timer := prometheus.NewTimer(evaluationDurationSeconds)
	//	defer timer.ObserveDuration()
	//	matchedRulesCount := 0
	//
	//	for _, ruleGroup := range re.ruleGroups {
	//		newMatchedCount, err := ruleGroup.Evaluate(newEvent, outChan)
	//		matchedRulesCount += newMatchedCount
	//	}
	//
	//	if matchedRulesCount == 0 {
	//		re.logger.Warnf("event %+v did not match any SLO rule", newEvent)
	//		didNotMatchAnyRule.Inc()
	//	}
}
