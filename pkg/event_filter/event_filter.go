package event_filter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
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
	}, []string{"matcher"})
)

func init() {
	log = logrus.WithFields(logrus.Fields{"component": component})
	prometheus.MustRegister(filteredEventsTotal)
}

type eventFilterConfig struct {
	FilteredHttpStatusCodes []int
	FilteredHttpHeaders     map[string]string
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

func NewFromViper(viperConfig *viper.Viper) (*RequestEventFilter, error) {
	var config eventFilterConfig
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return New(config), nil
}

func New(config eventFilterConfig) *RequestEventFilter {
	return &RequestEventFilter{
		config.FilteredHttpStatusCodes,
		headersToLowercase(config.FilteredHttpHeaders),
	}
}

func (ef *RequestEventFilter) matches(event *producer.RequestEvent) bool {
	if ef.statusMatch(event.StatusCode) {
		filteredEventsTotal.WithLabelValues("status_code").Inc()
		log.WithField("event", event).Debugf("matched event because of status code")
		return true
	}
	if ef.headersMatch(event.Headers) {
		filteredEventsTotal.WithLabelValues("http_header").Inc()
		log.WithField("event", event).Debugf("matched event because of status code")
		return true
	}
	return false
}

func (ef *RequestEventFilter) statusMatch(testedStatus int) bool {
	for _, status := range ef.statuses {
		if status == testedStatus {

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
			return true
		}
	}
	return false
}

func (ef *RequestEventFilter) Run(in <-chan *producer.RequestEvent, out chan<- *producer.RequestEvent) {
	go func() {
		defer close(out)

		for event := range in {
			if !ef.matches(event) {
				out <- event
			}
		}
		log.Info("input channel closed, finishing")
	}()
}
