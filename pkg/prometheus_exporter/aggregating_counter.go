package prometheus_exporter

import (
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/seznam/slo-exporter/pkg/stringmap"
	"github.com/sirupsen/logrus"
)

func aggregatedMetricName(metricName string, aggregatedLabels ...string) string {
	return strings.Join(aggregatedLabels, "_") + ":" + metricName
}

// newAggregatedCounterVector creates set of counters which will create set of cascade aggregation level metrics by dropping aggregation labels.
func newAggregatedCounterVectorSet(metricName, metricHelp string, aggregationLabels []string, logger logrus.FieldLogger, exemplarMetadataKeys []string) *aggregatedCounterVectorSet {
	vectorSet := aggregatedCounterVectorSet{
		aggregatedMetrics: []*aggregatedCounterVector{},
	}
	// Generate new metric vector for every label aggregation level.
	for i := 1; i <= len(aggregationLabels); i++ {
		vectorSet.aggregatedMetrics = append(vectorSet.aggregatedMetrics, &aggregatedCounterVector{
			vector:       newCounterVector(aggregatedMetricName(metricName, aggregationLabels[:i]...), metricHelp, logger, exemplarMetadataKeys),
			labelsToDrop: aggregationLabels[i:],
		})
	}
	return &vectorSet
}

type aggregatedCounterVector struct {
	vector       *counterVector
	labelsToDrop []string
}

func (v *aggregatedCounterVector) register(registry prometheus.Registerer) error {
	if err := registry.Register(v.vector); err != nil {
		return err
	}
	return nil
}

func (v *aggregatedCounterVector) addWithExemplar(value float64, labels, exemplarLabels stringmap.StringMap) {
	v.vector.addWithExemplar(value, labels.Without(v.labelsToDrop), exemplarLabels)
}

type aggregatedCounterVectorSet struct {
	aggregatedMetrics []*aggregatedCounterVector
}

func (s *aggregatedCounterVectorSet) register(registry prometheus.Registerer) error {
	for _, metric := range s.aggregatedMetrics {
		if err := metric.register(registry); err != nil {
			return err
		}
	}
	return nil
}

func (s *aggregatedCounterVectorSet) inc(labels stringmap.StringMap) {
	s.add(1, labels)
}

func (s *aggregatedCounterVectorSet) add(value float64, labels stringmap.StringMap) {
	s.addWithExemplar(value, labels, stringmap.StringMap{})
}

func (s *aggregatedCounterVectorSet) addWithExemplar(value float64, labels, exemplarLabels stringmap.StringMap) {
	for _, metric := range s.aggregatedMetrics {
		metric.addWithExemplar(value, labels, exemplarLabels)
	}
}

func newCounterVector(name, help string, logger logrus.FieldLogger, _ []string) *counterVector {
	newVector := counterVector{
		name:     name,
		help:     help,
		counters: map[string]*counter{},
		mtx:      sync.RWMutex{},
		logger:   logger,
	}
	return &newVector
}

type counter struct {
	value       float64
	labelNames  []string
	labelValues []string
	exemplar    *dto.Exemplar
}

func (e *counter) addWithExemplar(value float64, exemplarLabels stringmap.StringMap) {
	e.value += value
	if len(exemplarLabels) > 0 {
		exemplar, err := newExemplar(value, time.Now(), prometheus.Labels(exemplarLabels))
		if err != nil {
			return
		}
		e.exemplar = exemplar
	}
}

type counterVector struct {
	name     string
	help     string
	counters map[string]*counter
	mtx      sync.RWMutex
	logger   logrus.FieldLogger
}

func (e *counterVector) addWithExemplar(value float64, labels, exemplarLabels stringmap.StringMap) {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	labelsString := labels.String()
	newCounter, ok := e.counters[labelsString]
	if !ok {
		labelNames := labels.SortedKeys()
		newCounter = &counter{
			value:       0,
			labelNames:  labelNames,
			labelValues: labels.ValuesByKeys(labelNames),
		}
		e.counters[labelsString] = newCounter
	}
	newCounter.addWithExemplar(value, exemplarLabels)
}

// We do not know the labels beforehand, so we disable registration time checks by not sending any result to channel.
func (e *counterVector) Describe(chan<- *prometheus.Desc) {}

func (e *counterVector) Collect(ch chan<- prometheus.Metric) {
	e.mtx.RLock()
	defer e.mtx.RUnlock()
	for _, c := range e.counters {
		newMetric, err := NewConstCounterWithExemplar(
			prometheus.NewDesc(e.name, e.help, c.labelNames, nil),
			prometheus.CounterValue,
			c.value,
			c.labelValues...,
		)
		if err != nil {
			e.logger.Errorf("failed to initialize new const metric: %v", err)
			ch <- prometheus.NewInvalidMetric(prometheus.NewDesc(e.name, e.help, c.labelNames, nil), err)
			continue
		}
		if c.exemplar != nil {
			newMetric.AddExemplar(c.exemplar)
		}
		ch <- newMetric
	}
}
