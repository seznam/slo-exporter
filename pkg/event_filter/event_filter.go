package event_filter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/pipeline"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"regexp"
	"time"
)

var (
	filteredEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "filtered_events_total",
		Help: "Total number of filtered events by metadata_key.",
	}, []string{"metadata_key"})
)

type eventFilterConfig struct {
	MetadataFilter stringmap.StringMap
}

type metadataMatcher struct {
	nameRegexp  *regexp.Regexp
	valueRegexp *regexp.Regexp
}

type EventFilter struct {
	metadataMatchers []metadataMatcher
	observer         pipeline.EventProcessingDurationObserver
	logger           logrus.FieldLogger
	inputChannel     chan *event.HttpRequest
	outputChannel    chan *event.HttpRequest
	done             bool
}

func (ef *EventFilter) RegisterMetrics(_ prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	return wrappedRegistry.Register(filteredEventsTotal)
}

func (ef *EventFilter) String() string {
	return "eventFilter"
}

func (ef *EventFilter) Done() bool {
	return ef.done
}

func (ef *EventFilter) Stop() {
	return
}

func (ef *EventFilter) SetInputChannel(channel chan *event.HttpRequest) {
	ef.inputChannel = channel
}

func (ef *EventFilter) OutputChannel() chan *event.HttpRequest {
	return ef.outputChannel
}

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*EventFilter, error) {
	var config eventFilterConfig
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return NewFromConfig(config, logger)
}

func NewFromConfig(config eventFilterConfig, logger logrus.FieldLogger) (*EventFilter, error) {
	filter := EventFilter{
		metadataMatchers: []metadataMatcher{},
		outputChannel:    make(chan *event.HttpRequest),
		inputChannel:     make(chan *event.HttpRequest),
		done:             false,
		logger:           logger,
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
		ef.logger.WithField("event", event).Debugf("dropping event because of matching event metadata key %s", matchedKey)
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

func (ef *EventFilter) RegisterEventProcessingDurationObserver(observer pipeline.EventProcessingDurationObserver) {
	ef.observer = observer
}

func (ef *EventFilter) observeDuration(start time.Time) {
	if ef.observer != nil {
		ef.observer.Observe(time.Since(start).Seconds())
	}
}

func (ef *EventFilter) Run() {
	go func() {
		defer func() {
			close(ef.outputChannel)
			ef.done = true
		}()
		for newEvent := range ef.inputChannel {
			start := time.Now()
			if !ef.matches(newEvent) {
				ef.outputChannel <- newEvent
			}
			ef.observeDuration(start)
		}
		ef.logger.Info("input channel closed, finishing")
	}()
}
