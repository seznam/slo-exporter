package event_filter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"regexp"
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
		Help:      "Total number of filtered events by metadata_key.",
	}, []string{"metadata_key"})
)

func init() {
	log = logrus.WithFields(logrus.Fields{"component": component})
	prometheus.MustRegister(filteredEventsTotal)
}

type eventFilterConfig struct {
	MetadataFilter stringmap.StringMap
}

type metadataMatcher struct {
	nameRegexp  *regexp.Regexp
	valueRegexp *regexp.Regexp
}

type EventFilter struct {
	metadataMatchers []metadataMatcher
	observer         prometheus.Observer
}

func NewFromViper(viperConfig *viper.Viper) (*EventFilter, error) {
	var config eventFilterConfig
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return NewFromConfig(config)
}

func NewFromConfig(config eventFilterConfig) (*EventFilter, error) {
	filter := EventFilter{
		metadataMatchers: []metadataMatcher{},
	}
	for nameMatcher, valueMatcher := range config.MetadataFilter {
		nameRegexpMatcher, err := regexp.Compile(nameMatcher)
		if err != nil {
			return nil, fmt.Errorf("invalid event metadata name matcher regular expression: %v", err)
		}
		valueRegexpMatcher, err := regexp.Compile(valueMatcher)
		if err != nil {
			return nil, fmt.Errorf("invalid event metadata value matcher regular expression: %v", err)
		}
		filter.metadataMatchers = append(filter.metadataMatchers, metadataMatcher{nameRegexp: nameRegexpMatcher, valueRegexp: valueRegexpMatcher})
	}
	return &filter, nil
}

func (ef *EventFilter) matches(event *event.HttpRequest) bool {
	matches, matchedKey := ef.metadataMatch(event.Metadata)
	if matches {
		filteredEventsTotal.WithLabelValues(matchedKey).Inc()
		log.WithField("event", event).Debugf("dropping event because of matching event metadata key %s", matchedKey)
		return true
	}
	return false
}

func (ef *EventFilter) metadataMatch(testedMetadata stringmap.StringMap) (bool, string) {
	for _, matcher := range ef.metadataMatchers {
		for metadataName, metadataValue := range testedMetadata {
			if matcher.nameRegexp.MatchString(metadataName) && matcher.valueRegexp.MatchString(metadataValue) {
				return true, metadataName
			}
		}
	}
	return false, ""
}

func (ef *EventFilter) SetPrometheusObserver(observer prometheus.Observer) {
	ef.observer = observer
}

func (ef *EventFilter) observeDuration(start time.Time) {
	if ef.observer != nil {
		ef.observer.Observe(time.Since(start).Seconds())
	}
}

func (ef *EventFilter) Run(in <-chan *event.HttpRequest, out chan<- *event.HttpRequest) {
	go func() {
		defer close(out)
		for newEvent := range in {
			start := time.Now()
			if !ef.matches(newEvent) {
				out <- newEvent
			}
			ef.observeDuration(start)
		}
		log.Info("input channel closed, finishing")
	}()
}
