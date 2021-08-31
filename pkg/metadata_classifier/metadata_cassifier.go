package metadata_classifier

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/pipeline"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"time"
)

var (
	processedEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "processed_events_total",
		Help: "Total number of processed events by operation.",
	}, []string{"operation"})
)

type metadataClassifierConfig struct {
	SloDomainMetadataKey   string
	SloClassMetadataKey    string
	SloAppMetadataKey      string
	OverrideExistingValues bool
}

type metadataClassifier struct {
	overrideExistingValues bool
	domainKey              string
	classKey               string
	appKey                 string
	observer               pipeline.EventProcessingDurationObserver
	logger                 logrus.FieldLogger
	inputChannel           chan event.Raw
	outputChannel          chan event.Raw
	done                   bool
}

func (e *metadataClassifier) RegisterMetrics(_ prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	return wrappedRegistry.Register(processedEventsTotal)
}

func (e *metadataClassifier) String() string {
	return "metadataClassifier"
}

func (e *metadataClassifier) Done() bool {
	return e.done
}

func (e *metadataClassifier) Stop() {
	return
}

func (e *metadataClassifier) SetInputChannel(channel chan event.Raw) {
	e.inputChannel = channel
}

func (e *metadataClassifier) OutputChannel() chan event.Raw {
	return e.outputChannel
}

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*metadataClassifier, error) {
	var config metadataClassifierConfig
	viperConfig.SetDefault("OverrideExistingValues", true)
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return NewFromConfig(config, logger)
}

func NewFromConfig(config metadataClassifierConfig, logger logrus.FieldLogger) (*metadataClassifier, error) {
	filter := metadataClassifier{
		overrideExistingValues: config.OverrideExistingValues,
		domainKey:              config.SloDomainMetadataKey,
		classKey:               config.SloClassMetadataKey,
		appKey:                 config.SloAppMetadataKey,
		outputChannel:          make(chan event.Raw),
		inputChannel:           make(chan event.Raw),
		done:                   false,
		logger:                 logger,
	}
	return &filter, nil
}

func (e *metadataClassifier) RegisterEventProcessingDurationObserver(observer pipeline.EventProcessingDurationObserver) {
	e.observer = observer
}

func (e *metadataClassifier) observeDuration(start time.Time) {
	if e.observer != nil {
		e.observer.Observe(time.Since(start).Seconds())
	}
}

func (e *metadataClassifier) generateSloClassification(toBeClassified event.Raw) event.SloClassification {
	newClassification := event.SloClassification{}
	if toBeClassified.IsClassified() {
		classification := toBeClassified.SloClassification()
		newClassification = classification.Copy()
	}
	metadataDomain, ok := toBeClassified.Metadata()[e.domainKey]
	if ok && (e.overrideExistingValues || newClassification.Domain == "") {
		newClassification.Domain = metadataDomain
	}
	metadataClass, ok := toBeClassified.Metadata()[e.classKey]
	if ok && (e.overrideExistingValues || newClassification.Class == "") {
		newClassification.Class = metadataClass
	}
	metadataApp, ok := toBeClassified.Metadata()[e.appKey]
	if ok && (e.overrideExistingValues || newClassification.App == "") {
		newClassification.App = metadataApp
	}
	return newClassification
}

func (e *metadataClassifier) Run() {
	go func() {
		defer func() {
			close(e.outputChannel)
			e.done = true
		}()
		for newEvent := range e.inputChannel {
			start := time.Now()
			if !e.overrideExistingValues && newEvent.IsClassified() {
				processedEventsTotal.WithLabelValues("skipped").Inc()
			} else {
				newClassification := e.generateSloClassification(newEvent)
				newEvent.SetSLOClassification(newClassification)
				processedEventsTotal.WithLabelValues("generated-slo-classification").Inc()
				e.logger.WithField("event", newEvent).WithField("slo-classification", newClassification).Debug("classified new event")
			}
			e.outputChannel <- newEvent
			e.observeDuration(start)
		}
		e.logger.Info("input channel closed, finishing")
	}()
}
