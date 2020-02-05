package event_filter

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"strings"
)

const (
	component = "event_filter"
)

var (
	log                 *logrus.Entry
	filteredEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "slo_exporter",
		Subsystem: component,
		Name:      "filtered_events_total",
		Help:      "Total number of filtered events by reason.",
	}, []string{"matcher", "filtered_value"})
)

func init() {
	log = logrus.WithFields(logrus.Fields{"component": component})
	prometheus.MustRegister(filteredEventsTotal)
}

type RequestEventFilter struct {
	statuses []int
	headers  map[string]string
}

func headersToLowercase(headers map[string]string) map[string]string {
	lowercaseHeaders := map[string]string{}
	for k, v := range headers {
		lowercaseHeaders[strings.ToLower(k)] = strings.ToLower(v)
	}
	return lowercaseHeaders
}

func New(dropStatuses []int, dropHeaders map[string]string) *RequestEventFilter {
	return &RequestEventFilter{
		dropStatuses,
		headersToLowercase(dropHeaders),
	}
}

func (ef *RequestEventFilter) matches(event *producer.RequestEvent) bool {
	if ef.statusMatch(event.StatusCode) {
		log.WithField("event", event).Debugf("matched event because of status code")
		return true
	}
	if ef.headersMatch(event.Headers) {
		log.WithField("event", event).Debugf("matched event because of status code")
		return true
	}
	return false
}

func (ef *RequestEventFilter) statusMatch(testedStatus int) bool {
	for _, status := range ef.statuses {
		if status == testedStatus {
			filteredEventsTotal.WithLabelValues("status_code", string(testedStatus)).Inc()
			return true
		}
	}
	return false
}

func (ef *RequestEventFilter) headersMatch(testedHeaders map[string]string) bool {
	lowerTestedHeaders := headersToLowercase(testedHeaders)
	for k, v := range ef.headers {
		value, ok := lowerTestedHeaders[k]
		if ok && value == v {
			filteredEventsTotal.WithLabelValues("http_header", k+"="+v).Inc()
			return true
		}
	}
	return false
}

func (ef *RequestEventFilter) Run(in <-chan *producer.RequestEvent, out chan<- *producer.RequestEvent) {
	go func() {
		defer log.Info("stopping...")
		defer close(out)
		for {
			select {
			case event, ok := <-in:
				if !ok {
					log.Info("input channel closed, finishing")
					return
				}
				if !ef.matches(event) {
					out <- event
				}
			}
		}
	}()
}
