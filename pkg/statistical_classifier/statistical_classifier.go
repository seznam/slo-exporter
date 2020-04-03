//revive:disable:var-naming
package statistical_classifier

//revive:enable:var-naming

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/pipeline"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
)

const (
	guessedLabelPlaceholder = "statistically-guessed"
)

var (
	eventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "events_processed_total",
			Help: "Total number of processed events by result.",
		},
		[]string{"result"},
	)

	errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "errors_total",
			Help: "Total number of errors.",
		},
		[]string{"type"},
	)
)

type classifierConfig struct {
	HistoryWindowSize  time.Duration
	HistoryWeightUpdateInterval time.Duration
}

// StatisticalClassifier is classifier based on cache and regexp matches
type StatisticalClassifier struct {
	classifier    *weightedClassifier
	observer      pipeline.EventProcessingDurationObserver
	logger        *logrus.Entry
	inputChannel  chan *event.HttpRequest
	outputChannel chan *event.HttpRequest
	done          bool
}

// NewFromViper create new instance of StatisticalClassifier based on viper config
func NewFromViper(viperConfig *viper.Viper, logger *logrus.Entry) (*StatisticalClassifier, error) {
	var config classifierConfig
	defaultWindowSize, err := time.ParseDuration("30m")
	if err != nil {
		return nil, fmt.Errorf("invalid default historyWindowSize vaule: %w", err)
	}
	viperConfig.SetDefault("historyWindowSize", defaultWindowSize)
	defaultUpdateInterval, err := time.ParseDuration("1m")
	if err != nil {
		return nil, fmt.Errorf("invalid default historyWeightUpdateInterval vaule: %w", err)
	}
	viperConfig.SetDefault("historyWeightUpdateInterval", defaultUpdateInterval)

	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return New(config, logger)
}

// New returns new instance of StatisticalClassifier
func New(conf classifierConfig, logger *logrus.Entry) (*StatisticalClassifier, error) {
	newClassifier, err := newWeightedClassifier(conf.HistoryWindowSize, conf.HistoryWeightUpdateInterval, logger)
	if err != nil {
		return nil, err
	}
	return &StatisticalClassifier{
		classifier:    newClassifier,
		logger:        logger,
		inputChannel:  make(chan *event.HttpRequest),
		outputChannel: make(chan *event.HttpRequest),
		done:          false,
	}, nil
}

func (sc *StatisticalClassifier) OutputChannel() chan *event.HttpRequest {
	return sc.outputChannel
}

func (sc *StatisticalClassifier) SetInputChannel(channel chan *event.HttpRequest) {
	sc.inputChannel = channel
}

func (sc *StatisticalClassifier) RegisterEventProcessingDurationObserver(observer pipeline.EventProcessingDurationObserver) {
	sc.observer = observer
}

func (sc *StatisticalClassifier) Stop() {
	return
}

func (sc *StatisticalClassifier) Done() bool {
	return sc.done
}

func (sc *StatisticalClassifier) RegisterMetrics(_ prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	toRegister := []prometheus.Collector{eventsTotal, errorsTotal, classificationWeightsMetric}
	for _, metric := range toRegister {
		if err := wrappedRegistry.Register(metric); err != nil {
			return err
		}
	}
	return nil
}

func (sc *StatisticalClassifier) observeDuration(start time.Time) {
	if sc.observer != nil {
		sc.observer.Observe(time.Since(start).Seconds())
	}
}

// To be able to distinguish statistically guessed events from regular ones, some of the classification parts are replaced with placeholder.
func (sc *StatisticalClassifier) sanitizeGuessedClassification(classification *event.SloClassification) event.SloClassification {
	newClassification := classification.Copy()
	newClassification.App = guessedLabelPlaceholder
	return newClassification
}

// Classify classifies event. Classification is guessed based on frequency of observed classifications over history window.
func (sc *StatisticalClassifier) Classify(event *event.HttpRequest) error {
	if !event.IsClassified() {
		classification, err := sc.classifier.guessClass()
		if err != nil {
			eventsTotal.WithLabelValues("unclassified").Inc()
			return err
		}
		event.UpdateSLOClassification(classification)
		eventsTotal.WithLabelValues("classified").Inc()
	} else {
		sc.classifier.increaseWeight(sc.sanitizeGuessedClassification(event.GetSloClassification()), 1)
		eventsTotal.WithLabelValues("increased-weight").Inc()
	}
	return nil
}

// Run statistic classifier receiving events and trying to classify event.
func (sc *StatisticalClassifier) Run() {
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer func() {
			cancel()
			close(sc.outputChannel)
			sc.done = true
		}()

		sc.classifier.Run(ctx)
		for newEvent := range sc.inputChannel {
			start := time.Now()
			if err := sc.Classify(newEvent); err != nil {
				sc.logger.WithField("event", newEvent).Error(err)
				errorsTotal.WithLabelValues("failedToClassify").Inc()
			} else {
				sc.logger.WithField("event", newEvent).Debug("processed event")
				sc.outputChannel <- newEvent
			}
			sc.observeDuration(start)
		}
		sc.logger.Info("input channel closed, finishing")
	}()
}
