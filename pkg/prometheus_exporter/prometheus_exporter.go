package prometheus_exporter

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/slo_event_producer"
)

var (
	component           string
	log                 *logrus.Entry
	sloEventResultLabel = "result"
	metricName          = "slo_events_total"
)

func init() {
	const component = "prometheus_exporter"
	log = logrus.WithField("component", component)
}

type PrometheusSloEventExporter struct {
	counterVec  *prometheus.CounterVec
	knownLabels []string
}

func New(labels []string) *PrometheusSloEventExporter {
	return &PrometheusSloEventExporter{
		prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        metricName,
			Help:        "Total number of SLO events exported with it's result and metadata.",
			ConstLabels: nil,
		}, append(labels, sloEventResultLabel)),
		append(labels, sloEventResultLabel),
	}
}

func (e *PrometheusSloEventExporter) Run(ctx context.Context, input <-chan *slo_event_producer.SloEvent) {
	prometheus.MustRegister(e.counterVec)

	go func() {
		defer log.Info("stopping...")
		for {
			select {
			case event, ok := <-input:
				if !ok {
					log.Info("input channel closed, finishing")
				}
				e.processEvent(event)
			}
		}
	}()
}

// make sure that eventMetadata contains exactly the expected set, so that it passed Prometheus library sanity checks
func normalizeEventMetadata(knownMetadata []string, eventMetadata map[string]string) map[string]string {
	normalized := make(map[string]string)
	for _, k := range knownMetadata {
		v, _ := eventMetadata[k]
		normalized[k] = v
	}
	return normalized
}

func (e *PrometheusSloEventExporter) processEvent(event *slo_event_producer.SloEvent) {
	normalizedMetadata := normalizeEventMetadata(e.knownLabels, event.SloMetadata)

	// Make sure that for given eventMetadata, all possible cases are properly initialized
	for _, possibleResult := range slo_event_producer.EventResults {
		normalizedMetadata[sloEventResultLabel] = string(possibleResult)
		e.counterVec.With(prometheus.Labels(normalizedMetadata)).Add(0)
	}
	normalizedMetadata[sloEventResultLabel] = string(event.Result)
	e.counterVec.With(prometheus.Labels(normalizedMetadata)).Inc()
}
