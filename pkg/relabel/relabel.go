package relabel

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/pipeline"
	"github.com/seznam/slo-exporter/pkg/stringmap"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"

	"github.com/prometheus/prometheus/pkg/relabel"
	"github.com/sirupsen/logrus"
)

var (
	droppedEventsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dropped_events_total",
		Help: "Total number of dropped events.",
	})
)

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*eventRelabelManager, error) {
	// Viper unmarshal the nested structure to nested structure of interface{} types.
	// Prometheus relabel uses classic YAML unmarshalling so we marshall the structure to YAML again and then let
	// Prometheus code validate it and unmarshall it.
	var relabelConf []relabel.Config
	marshalledConfig, err := yaml.Marshal(viperConfig.Get("EventRelabelConfigs"))
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	if err := yaml.UnmarshalStrict(marshalledConfig, &relabelConf); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return NewFromConfig(relabelConf, logger)
}

// New returns requestNormalizer which allows to add Key to RequestEvent
func NewFromConfig(relabelConfig []relabel.Config, logger logrus.FieldLogger) (*eventRelabelManager, error) {
	relabelManager := eventRelabelManager{
		relabelConfig: relabelConfig,
		outputChannel: make(chan *event.Raw),
		logger:        logger,
	}
	return &relabelManager, nil
}

type eventRelabelManager struct {
	relabelConfig []relabel.Config
	observer      pipeline.EventProcessingDurationObserver
	inputChannel  chan *event.Raw
	outputChannel chan *event.Raw
	done          bool
	logger        logrus.FieldLogger
}

func (r *eventRelabelManager) String() string {
	return "relabel"
}

func (r *eventRelabelManager) Done() bool {
	return r.done
}

func (r *eventRelabelManager) RegisterMetrics(_ prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	return wrappedRegistry.Register(droppedEventsTotal)
}

func (r *eventRelabelManager) SetInputChannel(channel chan *event.Raw) {
	r.inputChannel = channel
}

func (r *eventRelabelManager) OutputChannel() chan *event.Raw {
	return r.outputChannel
}

func (r *eventRelabelManager) Stop() {
	return
}

func (r *eventRelabelManager) RegisterEventProcessingDurationObserver(observer pipeline.EventProcessingDurationObserver) {
	r.observer = observer
}

func (r *eventRelabelManager) observeDuration(start time.Time) {
	if r.observer != nil {
		r.observer.Observe(time.Since(start).Seconds())
	}
}

// relabelEvent applies the relabel configs on the event metadata.
// If event is about to be dropped, nil is returned.
func (r *eventRelabelManager) relabelEvent(event *event.Raw) *event.Raw {
	newLabels := event.Metadata.AsPrometheusLabels()
	for _, relabelConfigRule := range r.relabelConfig {
		newLabels = relabel.Process(newLabels, &relabelConfigRule)
		if newLabels == nil {
			return nil
		}
	}
	event.Metadata = stringmap.NewFromLabels(newLabels)
	return event
}

// Run event replacer receiving events and filling their Key if not already filled.
func (r *eventRelabelManager) Run() {
	go func() {
		defer func() {
			close(r.outputChannel)
			r.done = true
		}()
		for newEvent := range r.inputChannel {
			start := time.Now()
			relabeledEvent := r.relabelEvent(newEvent)
			if relabeledEvent == nil {
				r.logger.WithField("event", newEvent).Debug("dropping event")
				droppedEventsTotal.Inc()
				continue
			}
			r.logger.WithField("event", newEvent).Debug("relabeled event")
			r.outputChannel <- relabeledEvent
			r.observeDuration(start)
		}
		r.logger.Info("input channel closed, finishing")
	}()
}
