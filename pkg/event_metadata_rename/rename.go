package event_metadata_rename

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

var (
	renamingCollisionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "renaming_collisions_total",
		Help: "Total number of collision occurred while attempting to rename a metadata key.",
	}, []string{"Source", "Destination"})
)

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*eventMetadataRenameManager, error) {
	var renameConfig []renameConfig
	marshalledConfig, err := yaml.Marshal(viperConfig.Get("eventMetadataRenameConfigs"))
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	if err := yaml.UnmarshalStrict(marshalledConfig, &renameConfig); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return NewFromConfig(renameConfig, logger)
}

// New returns requestNormalizer which allows to add Key to RequestEvent
func NewFromConfig(renameConfig []renameConfig, logger logrus.FieldLogger) (*eventMetadataRenameManager, error) {
	relabelManager := eventMetadataRenameManager{
		renameConfig: renameConfig,
		outputChannel: make(chan *event.Raw),
		logger:        logger,
	}
	return &relabelManager, nil
}
type renameConfig struct {
	Source, Destination string
}

type eventMetadataRenameManager struct {
	renameConfig []renameConfig
	observer      pipeline.EventProcessingDurationObserver
	inputChannel  chan *event.Raw
	outputChannel chan *event.Raw
	done          bool
	logger        logrus.FieldLogger
}

func (r *eventMetadataRenameManager) String() string {
	return "eventMetadataRename"
}

func (r *eventMetadataRenameManager) Done() bool {
	return r.done
}

func (r *eventMetadataRenameManager) RegisterMetrics(_ prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	return wrappedRegistry.Register(renamingCollisionsTotal)
}

func (r *eventMetadataRenameManager) SetInputChannel(channel chan *event.Raw) {
	r.inputChannel = channel
}

func (r *eventMetadataRenameManager) OutputChannel() chan *event.Raw {
	return r.outputChannel
}

func (r *eventMetadataRenameManager) Stop() {
	return
}

func (r *eventMetadataRenameManager) RegisterEventProcessingDurationObserver(observer pipeline.EventProcessingDurationObserver) {
	r.observer = observer
}

func (r *eventMetadataRenameManager) observeDuration(start time.Time) {
	if r.observer != nil {
		r.observer.Observe(time.Since(start).Seconds())
	}
}

// renameEventMetadata applies the relabel configs on the event metadata.
func (r *eventMetadataRenameManager) renameEventMetadata(e *event.Raw) *event.Raw {
	// TODO
	for _, renameConfig := range r.renameConfig {
		if _, ok := e.Metadata[renameConfig.Destination]; ok {
			r.logger.Warnf("refusing to override metadata's %s:%s with %s:%s", renameConfig.Destination, e.Metadata[renameConfig.Destination], renameConfig.Source, e.Metadata[renameConfig.Source])
			renamingCollisionsTotal.WithLabelValues(renameConfig.Source, renameConfig.Destination).Inc()
			continue
		}
		if _, ok := e.Metadata[renameConfig.Source]; !ok {
			continue
		}
		e.Metadata[renameConfig.Destination] = e.Metadata[renameConfig.Source]
		delete(e.Metadata, renameConfig.Source)
	}
	return e
}

// Run event replacer receiving events and filling their Key if not already filled.
func (r *eventMetadataRenameManager) Run() {
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
