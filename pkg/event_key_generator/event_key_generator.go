package event_key_generator

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/pipeline"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"time"
)

var (
	processedEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "processed_events_total",
		Help: "Total number of processed events by operation.",
	}, []string{"operation"})
)

type eventKeyGeneratorConfig struct {
	FiledSeparator           string
	OverrideExistingEventKey bool
	MetadataKeys             []string
}

type EventKeyGenerator struct {
	separator           string
	overrideExistingKey bool
	metadataKeys        []string
	observer            pipeline.EventProcessingDurationObserver
	logger              logrus.FieldLogger
	inputChannel        chan *event.Raw
	outputChannel       chan *event.Raw
	done                bool
}

func (e *EventKeyGenerator) RegisterMetrics(_ prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	return wrappedRegistry.Register(processedEventsTotal)
}

func (e *EventKeyGenerator) String() string {
	return "eventKeyGenerator"
}

func (e *EventKeyGenerator) Done() bool {
	return e.done
}

func (e *EventKeyGenerator) Stop() {
	return
}

func (e *EventKeyGenerator) SetInputChannel(channel chan *event.Raw) {
	e.inputChannel = channel
}

func (e *EventKeyGenerator) OutputChannel() chan *event.Raw {
	return e.outputChannel
}

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*EventKeyGenerator, error) {
	var config eventKeyGeneratorConfig
	viperConfig.SetDefault("OverrideExistingEventKey", true)
	viperConfig.SetDefault("FiledSeparator", ":")
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return NewFromConfig(config, logger)
}

func NewFromConfig(config eventKeyGeneratorConfig, logger logrus.FieldLogger) (*EventKeyGenerator, error) {
	filter := EventKeyGenerator{
		separator:           config.FiledSeparator,
		overrideExistingKey: config.OverrideExistingEventKey,
		metadataKeys:        config.MetadataKeys,
		outputChannel:       make(chan *event.Raw),
		inputChannel:        make(chan *event.Raw),
		done:                false,
		logger:              logger,
	}
	return &filter, nil
}

func (e *EventKeyGenerator) RegisterEventProcessingDurationObserver(observer pipeline.EventProcessingDurationObserver) {
	e.observer = observer
}

func (e *EventKeyGenerator) observeDuration(start time.Time) {
	if e.observer != nil {
		e.observer.Observe(time.Since(start).Seconds())
	}
}

func (e *EventKeyGenerator) generateEventKey(metadata stringmap.StringMap) string {
	first := true
	eventKey := ""
	for _, key := range e.metadataKeys {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		if first {
			eventKey = value
			first = false
		} else {
			eventKey += e.separator + value
		}
	}
	return eventKey
}

func (e *EventKeyGenerator) Run() {
	go func() {
		defer func() {
			close(e.outputChannel)
			e.done = true
		}()
		for newEvent := range e.inputChannel {
			start := time.Now()
			if newEvent.EventKey() == "" || e.overrideExistingKey {
				newKey := e.generateEventKey(newEvent.Metadata)
				newEvent.SetEventKey(newKey)
				processedEventsTotal.WithLabelValues("generated-event-key").Inc()
				e.logger.WithField("event", newEvent).WithField("event-key", newKey).Debug("generated new event key for event")
			} else {
				e.logger.WithField("event", newEvent).Debug("skipped generating of eventKey because it is already set")
				processedEventsTotal.WithLabelValues("skipped").Inc()
			}
			e.outputChannel <- newEvent
			e.observeDuration(start)
		}
		e.logger.Info("input channel closed, finishing")
	}()
}
