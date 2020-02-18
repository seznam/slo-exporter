package prometheus_exporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"strings"
)

func newAggregatedCounterSet(registry prometheus.Registerer, metricName string, possibleLabels []string, labelNames labelsNamesConfig) (*aggregatedCounterSet, error) {
	// Will have all labels domain, class, app, event key.
	perEndpointCounter, err := newAggregatedCounter(registry, metricName, metricHelp, possibleLabels, []string{labelNames.SloDomain, labelNames.SloClass, labelNames.SloApp, labelNames.EventKey}, []string{})
	if err != nil {
		return nil, err
	}
	// Will have only labels domain, class, app.
	perAppCounter, err := newAggregatedCounter(registry, metricName, metricHelp, possibleLabels, []string{labelNames.SloDomain, labelNames.SloClass, labelNames.SloApp}, []string{labelNames.EventKey})
	if err != nil {
		return nil, err
	}
	// Will have all labels domain, class.
	perClassCounter, err := newAggregatedCounter(registry, metricName, metricHelp, possibleLabels, []string{labelNames.SloDomain, labelNames.SloClass}, []string{labelNames.SloApp, labelNames.EventKey})
	if err != nil {
		return nil, err
	}
	// Will have all labels domain.
	perDomainCounter, err := newAggregatedCounter(registry, metricName, metricHelp, possibleLabels, []string{labelNames.SloDomain}, []string{labelNames.SloClass, labelNames.SloApp, labelNames.EventKey})
	if err != nil {
		return nil, err
	}
	return &aggregatedCounterSet{aggregatedMetrics: []*aggregatedCounter{perEndpointCounter, perAppCounter, perClassCounter, perDomainCounter}}, nil
}

type aggregatedCounterSet struct {
	aggregatedMetrics []*aggregatedCounter
}

func (s *aggregatedCounterSet) inc(labels stringmap.StringMap) {
	for _, metric := range s.aggregatedMetrics {
		metric.inc(labels)
	}
}

func (s *aggregatedCounterSet) add(value float64, labels stringmap.StringMap) {
	for _, metric := range s.aggregatedMetrics {
		metric.add(value, labels)
	}
}

func aggregatedMetricName(metricName string, aggregatedLabels ...string) string {
	return strings.Join(aggregatedLabels, "_") + ":" + metricName
}

func newAggregatedCounter(registry prometheus.Registerer, metricName, metricHelp string, possibleLabels []string, aggregatedLabels []string, labelsToDrop []string) (*aggregatedCounter, error) {
	newAggregatedCounter := aggregatedCounter{
		labelsToDrop: stringmap.NewFromKeys(labelsToDrop),
		counter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: aggregatedMetricName(metricName, aggregatedLabels...),
			Help: metricHelp,
		}, possibleLabels),
	}
	if err := registry.Register(newAggregatedCounter.counter); err != nil {
		return nil, err
	}
	return &newAggregatedCounter, nil
}

type aggregatedCounter struct {
	labelsToDrop stringmap.StringMap
	counter      *prometheus.CounterVec
}

func (c *aggregatedCounter) inc(labels stringmap.StringMap) {
	c.add(1, labels)
}

func (c *aggregatedCounter) add(value float64, labels stringmap.StringMap) {
	c.counter.With(prometheus.Labels(labels.Merge(c.labelsToDrop))).Add(value)
}
