package event_filter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"time"
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
	FilteredHttpHeaders     stringmap.StringMap
}

type RequestEventFilter struct {
	statuses []int
	headers  stringmap.StringMap
	observer prometheus.Observer
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
		statuses: config.FilteredHttpStatusCodes,
		headers:  config.FilteredHttpHeaders.Lowercase(),
	}
}

func (ef *RequestEventFilter) matches(event *event.HttpRequest) bool {
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

func (ef *RequestEventFilter) headersMatch(testedHeaders stringmap.StringMap) bool {
	lowerTestedHeaders := testedHeaders.Lowercase()
	for k, v := range ef.headers {
		value, ok := lowerTestedHeaders[k]
		if ok && value == v {
			return true
		}
	}
	return false
}

func (ef *RequestEventFilter) SetPrometheusObserver(observer prometheus.Observer) {
	ef.observer = observer
}

func (ef *RequestEventFilter) observeDuration(start time.Time) {
	if ef.observer != nil {
		ef.observer.Observe(time.Since(start).Seconds())
	}
}

func (ef *RequestEventFilter) Run(in <-chan *event.HttpRequest, out chan<- *event.HttpRequest) {
	go func() {
		defer close(out)
		for event := range in {
			start := time.Now()
			if !ef.matches(event) {
				out <- event
			}
			ef.observeDuration(start)
		}
		log.Info("input channel closed, finishing")
	}()
}
