package statistical_classifier

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/seznam/slo-exporter/pkg/storage"
	"github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/seznam/slo-exporter/pkg/event"
	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/sampleuv"
)

var classificationWeightsMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "classification_weight",
		Help: "Current weight for given classification.",
	},
	[]string{"slo_domain", "slo_class"},
)

type classificationWeight struct {
	classification *event.SloClassification
	weight         float64
}

type classificationMapping map[string]*classificationWeight

func (c classificationMapping) inc(classification *event.SloClassification, weight float64) {
	key := classification.String()
	if _, ok := c[key]; !ok {
		c[key] = &classificationWeight{
			classification: classification,
			weight:         0,
		}
	}
	c[key].weight += weight
}

func (c classificationMapping) merge(other classificationMapping) {
	for _, classificationWeight := range other {
		c.inc(classificationWeight.classification, classificationWeight.weight)
	}
}

func newWeightedClassificationSet() *weightedClassificationSet {
	return &weightedClassificationSet{
		mtx:                       sync.RWMutex{},
		enumeratedClassifications: []classificationWeight{},
		classificationWeights:     []float64{},
	}
}

func newWeightedClassificationSetFromClassifications(classifications *classificationMapping) *weightedClassificationSet {
	newSet := newWeightedClassificationSet()
	weights := make([]classificationWeight, 0, len(*classifications))
	for _, weight := range *classifications {
		weights = append(weights, *weight)
	}
	newSet.setWeights(weights)
	return newSet
}

// weightedClassificationSet serves as indexed mapping between classifications and its weights.
type weightedClassificationSet struct {
	mtx                       sync.RWMutex
	enumeratedClassifications []classificationWeight
	classificationWeights     []float64
}

func (w *weightedClassificationSet) index(i int) *event.SloClassification {
	return w.enumeratedClassifications[i].classification
}

func (w *weightedClassificationSet) setWeights(weights []classificationWeight) {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	w.enumeratedClassifications = weights
	w.classificationWeights = []float64{}
	for _, classificationWeight := range w.enumeratedClassifications {
		w.classificationWeights = append(w.classificationWeights, classificationWeight.weight)
		classificationWeightsMetric.WithLabelValues(
			classificationWeight.classification.Domain,
			classificationWeight.classification.Class,
		).Set(classificationWeight.weight)
	}
}

func (w *weightedClassificationSet) weights() []float64 {
	w.mtx.RLock()
	defer w.mtx.RUnlock()
	return w.classificationWeights
}

type weightedClassifier struct {
	history                 storage.CappedContainer
	totalWeightsOverHistory *weightedClassificationSet
	defaultWeights          *weightedClassificationSet
	recentWeights           classificationMapping
	lock                    sync.RWMutex
	historyUpdateInterval   time.Duration
	logger                  logrus.FieldLogger
}

func newWeightedClassifier(windowSize, historyUpdateInterval time.Duration, logger logrus.FieldLogger) (*weightedClassifier, error) {
	if historyUpdateInterval == 0 {
		return nil, fmt.Errorf("history update interval cannot be zero")
	}
	historyItemsLimit := windowSize / historyUpdateInterval
	return &weightedClassifier{
		history:                 storage.NewInMemoryCappedContainer(int(historyItemsLimit)),
		totalWeightsOverHistory: newWeightedClassificationSet(),
		recentWeights:           classificationMapping{},
		lock:                    sync.RWMutex{},
		historyUpdateInterval:   historyUpdateInterval,
		logger:                  logger,
	}, nil
}

// increaseWeight increases classification weight in the recent data.
func (s *weightedClassifier) increaseWeight(classification event.SloClassification, weight float64) {
	s.recentWeights.inc(&classification, weight)
}

func (s *weightedClassifier) setDefaultWeights(defaultWeights *weightedClassificationSet) {
	s.defaultWeights = defaultWeights
}

// archive puts most recent weights to history queue, drops old expired data from it and recalculates the weights from the updated history.
func (s *weightedClassifier) archive() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.history.Add(s.recentWeights)
	s.recentWeights = classificationMapping{}
	if err := s.reweight(); err != nil {
		return fmt.Errorf("failed to reweight classifier from historical data: %w", err)
	}
	return nil
}

// reweight recalculates the weights over whole history.
func (s *weightedClassifier) reweight() error {
	totalClassificationsWeights := classificationMapping{}
	for item := range s.history.Stream() {
		itemClassificationsWeights, ok := item.(classificationMapping)
		if !ok {
			return fmt.Errorf("failed to cast '%+v' to 'classificationMapping'", item)
		}
		totalClassificationsWeights.merge(itemClassificationsWeights)
	}
	s.totalWeightsOverHistory = newWeightedClassificationSetFromClassifications(&totalClassificationsWeights)
	return nil
}

// guessClass returns classification for event. Its based on wights calculated over history window or the default ones configured.
// If there is no weights yet, error is returned.
func (s *weightedClassifier) guessClass() (*event.SloClassification, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	classificationWeights := s.totalWeightsOverHistory
	if len(classificationWeights.weights()) < 1 {
		if s.defaultWeights != nil && len(s.defaultWeights.weights()) > 0 {
			classificationWeights = s.defaultWeights
		} else {
			return nil, fmt.Errorf("not enough data to guess")
		}
	}

	w := sampleuv.NewWeighted(classificationWeights.weights(), rand.New(rand.NewSource(uint64(time.Now().UnixNano()))))
	i, ok := w.Take()
	if !ok {
		return nil, fmt.Errorf("not enough data to guess")
	}
	return classificationWeights.index(i), nil
}

// Run runs statistic refresher - archive recentWeights classifications and recount weightedClassifier.
func (s *weightedClassifier) Run(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.historyUpdateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.archive(); err != nil {
					s.logger.Errorf("failed to update historical data: %v", err)
					errorsTotal.WithLabelValues("failedToUpdateHistoricalData").Inc()
				}
			}
		}
	}()
}
