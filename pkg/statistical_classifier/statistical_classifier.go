//revive:disable:var-naming
package statistical_classifier

//revive:enable:var-naming

import (
	"context"
	"fmt"
	"github.com/seznam/slo-exporter/pkg/pipeline"
	"github.com/seznam/slo-exporter/pkg/storage"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/sampleuv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/spf13/viper"
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

	classificationWeightsMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "classification_weight",
			Help: "Current weight for given classification.",
		},
		[]string{"slo_domain", "slo_class"},
	)
)

type weightClassification struct {
	SloDomain string
	SloClass  string
}

type defaultClassificationWeight struct {
	Weight         float64
	Classification weightClassification
}

type classifierConfig struct {
	HistoryWindowSize           time.Duration
	HistoryWeightUpdateInterval time.Duration
	DefaultWeights              []defaultClassificationWeight
}

// StatisticalClassifier is classifier based on cache and regexp matches
type StatisticalClassifier struct {
	archiver      *storage.PeriodicalAggregatingArchiver
	observer      pipeline.EventProcessingDurationObserver
	logger        logrus.FieldLogger
	inputChannel  chan *event.Raw
	outputChannel chan *event.Raw
	done          bool
}

// NewFromViper create new instance of StatisticalClassifier based on viper config
func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*StatisticalClassifier, error) {
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

func defaultWeightsSetFromConfig(conf classifierConfig) classificationWeights {
	defaultWeights := newClassificationWeights()
	if len(conf.DefaultWeights) < 1 {
		return defaultWeights
	}
	for _, initialWeight := range conf.DefaultWeights {
		defaultWeights.inc(event.SloClassification{
			Domain: initialWeight.Classification.SloDomain,
			Class:  initialWeight.Classification.SloClass,
			App:    guessedLabelPlaceholder,
		}, initialWeight.Weight)
	}
	return defaultWeights
}

func aggregateClassificationStatistics(items []interface{}) (interface{}, error) {
	newAggregatedWeights := newClassificationWeights()
	for _, item := range items {
		weights, ok := item.(classificationWeights)
		if !ok {
			return nil, fmt.Errorf("failed to cast '%+v' to 'classificationMapping'", item)
		}
		newAggregatedWeights.merge(weights)
	}
	for _, item := range newAggregatedWeights.listClassificationWeights() {
		classificationWeightsMetric.WithLabelValues(item.classification.Domain, item.classification.Class).Set(item.weight)
	}
	return newAggregatedWeights, nil
}

// New returns new instance of StatisticalClassifier
func New(conf classifierConfig, logger logrus.FieldLogger) (*StatisticalClassifier, error) {
	newArchiver := storage.NewPeriodicalAggregatingArchiver(
		logger.WithField("submodule", "PeriodicalAggregatingArchiver"),
		storage.NewInMemoryCappedContainer(int(conf.HistoryWindowSize/conf.HistoryWeightUpdateInterval)),
		newClassificationWeights(),
		aggregateClassificationStatistics,
		storage.NewTicker(conf.HistoryWeightUpdateInterval),
	)
	newArchiver.SetCurrent(defaultWeightsSetFromConfig(conf))
	return &StatisticalClassifier{
		archiver:      newArchiver,
		logger:        logger,
		inputChannel:  make(chan *event.Raw),
		outputChannel: make(chan *event.Raw),
		done:          false,
	}, nil
}

func (sc *StatisticalClassifier) OutputChannel() chan *event.Raw {
	return sc.outputChannel
}

func (sc *StatisticalClassifier) SetInputChannel(channel chan *event.Raw) {
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
	toRegister := []prometheus.Collector{eventsTotal, errorsTotal}
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

func (sc *StatisticalClassifier) currentWeights() (*classificationWeights, error) {
	currentWeights, ok := sc.archiver.Current().(classificationWeights)
	if !ok {
		return nil, fmt.Errorf("failed to cast '%+v' to 'classificationWeights'", sc.archiver.Current())
	}
	return &currentWeights, nil
}

func (sc *StatisticalClassifier) increaseWeight(classification event.SloClassification, value float64) error {
	weights, err := sc.currentWeights()
	if err != nil {
		return err
	}
	weights.inc(classification, value)
	sc.archiver.SetCurrent(weights)
	return nil
}

func (sc *StatisticalClassifier) guessClass() (*event.SloClassification, error) {
	currentWeights, err := sc.currentWeights()
	if err != nil {
		return nil, err
	}
	if currentWeights.len() < 1 {
		return nil, fmt.Errorf("not enough data to guess")
	}
	w := sampleuv.NewWeighted(currentWeights.sortedWeights(), rand.New(rand.NewSource(uint64(int64(time.Now().UnixNano())))))
	i, ok := w.Take()
	if !ok {
		return nil, fmt.Errorf("not enough data to guess")
	}
	classificationWeight, err := currentWeights.index(i)
	if err != nil {
		return nil, fmt.Errorf("not enough data to guess")
	}
	return &classificationWeight.classification, nil
}

// Classify classifies event. Classification is guessed based on frequency of observed classifications over history window.
func (sc *StatisticalClassifier) Classify(event *event.Raw) error {
	if !event.IsClassified() {
		classification, err := sc.guessClass()
		if err != nil {
			eventsTotal.WithLabelValues("unclassified").Inc()
			return err
		}
		event.UpdateSLOClassification(classification)
		eventsTotal.WithLabelValues("classified").Inc()
	} else {
		if err := sc.increaseWeight(sc.sanitizeGuessedClassification(event.GetSloClassification()), 1); err != nil {
			return err
		}
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

		sc.archiver.Run(ctx)
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
