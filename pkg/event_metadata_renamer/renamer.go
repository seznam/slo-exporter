package event_metadata_renamer

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"

	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/pipeline"

	"github.com/sirupsen/logrus"
)

var renamingCollisionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "renaming_collisions_total",
	Help: "Total number of collision occurred while attempting to rename a metadata key.",
}, []string{"Source", "Destination"})

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*EventMetadataRenamerManager, error) {
	var config []renamerConfig
	marshalledConfig, err := yaml.Marshal(viperConfig.Get("eventMetadataRenamerConfigs"))
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	if err := yaml.UnmarshalStrict(marshalledConfig, &config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return NewFromConfig(config, logger)
}

// New returns requestNormalizer which allows to add Key to RequestEvent.
func NewFromConfig(config []renamerConfig, logger logrus.FieldLogger) (*EventMetadataRenamerManager, error) {
	relabelManager := EventMetadataRenamerManager{
		renamerConfig: config,
		outputChannel: make(chan *event.Raw),
		logger:        logger,
	}
	return &relabelManager, nil
}

type renamerConfig struct {
	Source, Destination string
}

type EventMetadataRenamerManager struct {
	renamerConfig []renamerConfig
	observer      pipeline.EventProcessingDurationObserver
	inputChannel  chan *event.Raw
	outputChannel chan *event.Raw
	done          bool
	logger        logrus.FieldLogger
}

func (r *EventMetadataRenamerManager) String() string {
	return "eventMetadataRenamer"
}

func (r *EventMetadataRenamerManager) Done() bool {
	return r.done
}

func (r *EventMetadataRenamerManager) RegisterMetrics(_, wrappedRegistry prometheus.Registerer) error {
	return wrappedRegistry.Register(renamingCollisionsTotal)
}

func (r *EventMetadataRenamerManager) SetInputChannel(channel chan *event.Raw) {
	r.inputChannel = channel
}

func (r *EventMetadataRenamerManager) OutputChannel() chan *event.Raw {
	return r.outputChannel
}

func (r *EventMetadataRenamerManager) Stop() {}

func (r *EventMetadataRenamerManager) RegisterEventProcessingDurationObserver(observer pipeline.EventProcessingDurationObserver) {
	r.observer = observer
}

func (r *EventMetadataRenamerManager) observeDuration(start time.Time) {
	if r.observer != nil {
		r.observer.Observe(time.Since(start).Seconds())
	}
}

// renameEventMetadata applies the relabel configs on the event metadata.
func (r *EventMetadataRenamerManager) renameEventMetadata(e *event.Raw) *event.Raw {
	for _, renameConfig := range r.renamerConfig {
		if _, ok := e.Metadata[renameConfig.Source]; !ok {
			continue
		}
		if _, ok := e.Metadata[renameConfig.Destination]; ok {
			r.logger.Warnf("refusing to override metadata's %s:%s with %s:%s", renameConfig.Destination, e.Metadata[renameConfig.Destination], renameConfig.Source, e.Metadata[renameConfig.Source])
			renamingCollisionsTotal.WithLabelValues(renameConfig.Source, renameConfig.Destination).Inc()
			continue
		}
		e.Metadata[renameConfig.Destination] = e.Metadata[renameConfig.Source]
		delete(e.Metadata, renameConfig.Source)
	}
	return e
}

// Run event replacer receiving events and filling their Key if not already filled.
func (r *EventMetadataRenamerManager) Run() {
	go func() {
		defer func() {
			close(r.outputChannel)
			r.done = true
		}()
		for newEvent := range r.inputChannel {
			start := time.Now()
			processedEvent := r.renameEventMetadata(newEvent)
			r.logger.WithField("event", newEvent).Debug("processed event")
			r.outputChannel <- processedEvent
			r.observeDuration(start)
		}
		r.logger.Info("input channel closed, finishing")
	}()
}
