package event_filter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"regexp"
	"strconv"
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
	FilteredHttpStatusCodeMatchers []string
	FilteredHttpHeaderMatchers     stringmap.StringMap
}

type httpHeaderMatcher struct {
	nameRegexp  *regexp.Regexp
	valueRegexp *regexp.Regexp
}

type RequestEventFilter struct {
	statusMatchers []*regexp.Regexp
	headerMatchers []httpHeaderMatcher
	observer       prometheus.Observer
}

func NewFromViper(viperConfig *viper.Viper) (*RequestEventFilter, error) {
	var config eventFilterConfig
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return NewFromConfig(config)
}

func NewFromConfig(config eventFilterConfig) (*RequestEventFilter, error) {
	filter := RequestEventFilter{
		statusMatchers: []*regexp.Regexp{},
		headerMatchers: []httpHeaderMatcher{},
	}
	// Load status code matchers
	for _, statusMatcher := range config.FilteredHttpStatusCodeMatchers {
		regexpMatcher, err := regexp.Compile(statusMatcher)
		if err != nil {
			return nil, fmt.Errorf("invalid status code matcher regular expression: %v", err)
		}
		filter.statusMatchers = append(filter.statusMatchers, regexpMatcher)
	}
	// Load HTTP header matchers
	for nameMatcher, valueMatcher := range config.FilteredHttpHeaderMatchers {
		nameRegexpMatcher, err := regexp.Compile(nameMatcher)
		if err != nil {
			return nil, fmt.Errorf("invalid HTTP header name matcher regular expression: %v", err)
		}
		valueRegexpMatcher, err := regexp.Compile(valueMatcher)
		if err != nil {
			return nil, fmt.Errorf("invalid HTTP header value matcher regular expression: %v", err)
		}
		filter.headerMatchers = append(filter.headerMatchers, httpHeaderMatcher{nameRegexp: nameRegexpMatcher, valueRegexp: valueRegexpMatcher})
	}
	return &filter, nil
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
	for _, matcher := range ef.statusMatchers {
		if matcher.MatchString(strconv.Itoa(testedStatus)) {
			return true
		}
	}
	return false
}

func (ef *RequestEventFilter) headersMatch(testedHeaders stringmap.StringMap) bool {
	for _, matcher := range ef.headerMatchers {
		for headerName, headerValue := range testedHeaders {
			if matcher.nameRegexp.MatchString(headerName) && matcher.valueRegexp.MatchString(headerValue) {
				return true
			}
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
