package prometheus_exporter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"strings"
	"sync"
)

func aggregatedMetricName(metricName string, aggregatedLabels ...string) string {
	return strings.Join(aggregatedLabels, "_") + ":" + metricName
}

// newAggregatedCounterVector creates set of counters which will create set of cascade aggregation level metrics by dropping aggregation labels.
func newAggregatedCounterVectorSet(metricName, metricHelp string, aggregationLabels []string) *aggregatedCounterVectorSet {
	vectorSet := aggregatedCounterVectorSet{
		aggregatedMetrics: []aggregatedCounterVector{},
	}
	// Generate new metric vector for every label aggregation level.
	for i := 1; i <= len(aggregationLabels); i++ {
		vectorSet.aggregatedMetrics = append(vectorSet.aggregatedMetrics, aggregatedCounterVector{
			vector:       newCounterVector(aggregatedMetricName(metricName, aggregationLabels[:i]...), metricHelp),
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

func (v *aggregatedCounterVector) inc(labels stringmap.StringMap) {
	v.add(1, labels)
}

func (v *aggregatedCounterVector) add(value float64, labels stringmap.StringMap) {
	v.vector.add(value, labels.Without(v.labelsToDrop))
}

type aggregatedCounterVectorSet struct {
	aggregatedMetrics []aggregatedCounterVector
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
	for _, metric := range s.aggregatedMetrics {
		metric.add(value, labels)
	}
}

func newCounterVector(name, help string) *counterVector {
	newVector := counterVector{
		name:     name,
		help:     help,
		counters: map[string]*counter{},
		mtx:      sync.RWMutex{},
	}
	return &newVector
}

type counter struct {
	value       float64
	labelNames  []string
	labelValues []string
}

func (e *counter) add(value float64) {
	e.value += value
}

func (e *counter) inc() {
	e.add(1)
}

type counterVector struct {
	name     string
	help     string
	counters map[string]*counter
	mtx      sync.RWMutex
}

func (e *counterVector) add(value float64, labels stringmap.StringMap) {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	newCounter, ok := e.counters[labels.String()]
	if !ok {
		labelNames := labels.SortedKeys()
		newCounter = &counter{
			value:       0,
			labelNames:  labelNames,
			labelValues: labels.ValuesByKeys(labelNames),
		}
		e.counters[labels.String()] = newCounter
	}
	newCounter.add(value)
}

func (e *counterVector) inc(labels stringmap.StringMap) {
	e.add(1, labels)
}

func (e counterVector) Describe(chan<- *prometheus.Desc) {
	// We do not know the labels beforehand, so we disable registration time checks by not sending any result to channel.
	return
}

func (e counterVector) Collect(ch chan<- prometheus.Metric) {
	e.mtx.RLock()
	defer e.mtx.RUnlock()
	for _, c := range e.counters {
		newMetric, err := prometheus.NewConstMetric(
			prometheus.NewDesc(e.name, e.help, c.labelNames, nil),
			prometheus.CounterValue,
			c.value,
			c.labelValues...
		)
		if err != nil {
			fmt.Println(err)
		}
		ch <- newMetric
	}
}
