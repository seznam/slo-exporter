package prometheus_exporter

import (
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/slo_event_producer"
)

var (
	component   string
	log         *logrus.Entry
	errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "slo_exporter",
			Subsystem:   component,
			Name:        "errors_total",
			Help:        "",
			ConstLabels: prometheus.Labels{"app": "slo_exporter", "subsystem": component},
		},
		[]string{"type"})
	sloEventResultLabel = "result"
	metricName          = "slo_events_total"
)

func init() {
	const component = "prometheus_exporter"
	log = logrus.WithField("component", component)
	prometheus.MustRegister(errorsTotal)

}

type PrometheusSloEventExporter struct {
	eventsCount       *prometheus.CounterVec
	knownLabels       []string
	validEventResults []slo_event_producer.SloEventResult
}

type InvalidSloEventResult struct {
	result       string
	validResults []slo_event_producer.SloEventResult
}

func (e *InvalidSloEventResult) Error() string {
	return fmt.Sprintf("result '%s' is not valid. Expected one of: %v", e.result, e.validResults)
}

func New(labels []string, results []slo_event_producer.SloEventResult) *PrometheusSloEventExporter {
	return &PrometheusSloEventExporter{
		prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        metricName,
			Help:        "Total number of SLO events exported with it's result and metadata.",
			ConstLabels: nil,
		}, append(labels, sloEventResultLabel)),
		append(labels, sloEventResultLabel),
		results,
	}
}

func (e *PrometheusSloEventExporter) Run(input <-chan *slo_event_producer.SloEvent) {
	prometheus.MustRegister(e.eventsCount)

	go func() {
		defer log.Info("stopping...")
		for {
			select {
			case event, ok := <-input:
				if !ok {
					log.Info("input channel closed, finishing")
					return
				}
				err := e.processEvent(event)
				if err != nil {
					log.Errorf("unable to process slo event: %v", err)
					if errors.Is(err, &InvalidSloEventResult{}) {
						errorsTotal.With(prometheus.Labels{"type": "InvalidResult"}).Inc()
					} else {
						errorsTotal.With(prometheus.Labels{"type": "Unknown"}).Inc()
					}
				}
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

func (e *PrometheusSloEventExporter) isValidResult(result slo_event_producer.SloEventResult) bool {
	for _, validEventResult := range e.validEventResults {
		if validEventResult == result {
			return true
		}
	}
	return false
}

// for given event metadata, initialize exposed metric for all possible result label values
func (e *PrometheusSloEventExporter) initializeMetricForGivenMetadata(metadata map[string]string) {
	// do not edit the original metadata map
	metadataCopy := map[string]string{}
	for k, v := range metadata {
		metadataCopy[k] = v
	}
	for _, result := range e.validEventResults {
		metadataCopy[sloEventResultLabel] = string(result)
		e.eventsCount.With(prometheus.Labels(metadataCopy)).Add(0)
	}
}

func (e *PrometheusSloEventExporter) processEvent(event *slo_event_producer.SloEvent) error {
	normalizedMetadata := normalizeEventMetadata(e.knownLabels, event.SloMetadata)

	if !e.isValidResult(event.Result) {
		return &InvalidSloEventResult{string(event.Result), e.validEventResults}
	}
	e.initializeMetricForGivenMetadata(normalizedMetadata)

	// add result to metadata
	normalizedMetadata[sloEventResultLabel] = string(event.Result)
	e.eventsCount.With(prometheus.Labels(normalizedMetadata)).Inc()
	return nil
}
