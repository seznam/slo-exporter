package prometheus_exporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"strings"
)

func aggregatedMetricName(metricName string, aggregatedLabels... string) string {
	return strings.Join(aggregatedLabels, "_")+":"+metricName
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
