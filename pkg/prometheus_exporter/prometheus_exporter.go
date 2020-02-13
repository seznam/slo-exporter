package prometheus_exporter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/slo_event_producer"
	"time"
)

const (
	// used to replace the event's eventKey (for all previously unknown) in case that eventKeyLimit is exceeded
	eventKeyCardinalityLimitReplacement = "cardinalityLimitExceeded"
)

const (
	component = "prometheus_exporter"
)

var (
	log         *logrus.Entry
	errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "slo_exporter",
			Subsystem:   component,
			Name:        "errors_total",
			Help:        "Errors occurred during application runtime",
			ConstLabels: prometheus.Labels{"app": "slo_exporter", "module": component},
		},
		[]string{"type"})
	eventKeys = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   "slo_exporter",
			Subsystem:   component,
			Name:        "event_keys",
			Help:        "Number of known unique event keys",
			ConstLabels: prometheus.Labels{"app": "slo_exporter", "module": component},
		})
	eventKeyCardinalityLimit = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   "slo_exporter",
		Subsystem:   component,
		Name:        "event_keys_limit",
		Help:        "Event keys cardinality limit",
		ConstLabels: prometheus.Labels{"app": "slo_exporter", "module": component},
	})
	sloEventResultLabel = "result"
	metricName          = "slo_events_total"
)

func init() {
	log = logrus.WithField("component", component)
}

type PrometheusSloEventExporter struct {
	eventsCount       *prometheus.CounterVec
	knownLabels       []string
	validEventResults []slo_event_producer.SloEventResult
	eventKeyLabel     string
	eventKeyLimit     int
	eventKeyCache     map[string]int
	observer          prometheus.Observer
}

type InvalidSloEventResult struct {
	result       string
	validResults []slo_event_producer.SloEventResult
}

func (e *InvalidSloEventResult) Error() string {
	return fmt.Sprintf("result '%s' is not valid. Expected one of: %v", e.result, e.validResults)
}

func New(metricRegistry prometheus.Registerer, labels []string, results []slo_event_producer.SloEventResult, eventKeyLabel string, eventKeyLimit int) *PrometheusSloEventExporter {
	// initialize and register Prometheus metrics
	eventKeyCardinalityLimit.Set(float64(eventKeyLimit))
	eventsCount := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        metricName,
		Help:        "Total number of SLO events exported with it's result and metadata.",
		ConstLabels: nil,
	}, append(labels, sloEventResultLabel))
	metricRegistry.MustRegister(eventKeyCardinalityLimit, errorsTotal, eventKeys, eventsCount)

	return &PrometheusSloEventExporter{
		eventsCount:       eventsCount,
		knownLabels:       append(labels, sloEventResultLabel),
		validEventResults: results,
		eventKeyLabel:     eventKeyLabel,
		eventKeyLimit:     eventKeyLimit,
		eventKeyCache:     map[string]int{},
	}
}

func (e *PrometheusSloEventExporter) Run(input <-chan *slo_event_producer.SloEvent) {
	go func() {
		for event := range input {
			start := time.Now()
			err := e.processEvent(event)
			if err != nil {
				log.Errorf("unable to process slo event: %v", err)
				switch err.(type) {
				case *InvalidSloEventResult:
					errorsTotal.With(prometheus.Labels{"type": "InvalidResult"}).Inc()
				default:
					errorsTotal.With(prometheus.Labels{"type": "Unknown"}).Inc()
				}
			}
			e.observeDuration(start)
		}
		log.Info("input channel closed, finishing")
	}()
}

func (e *PrometheusSloEventExporter) SetPrometheusObserver(observer prometheus.Observer) {
	e.observer = observer
}

func (e *PrometheusSloEventExporter) observeDuration(start time.Time) {
	if e.observer != nil {
		e.observer.Observe(time.Since(start).Seconds())
	}
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

func (e *PrometheusSloEventExporter) isCardinalityExceeded(eventKey string) bool {
	if e.eventKeyLimit == 0 {
		// unlimited
		return false
	}

	_, ok := e.eventKeyCache[eventKey]
	if !ok && len(e.eventKeyCache)+1 > e.eventKeyLimit {
		return true
	} else {
		e.eventKeyCache[eventKey]++
		eventKeys.Set(float64(len(e.eventKeyCache)))
		return false
	}
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

	if e.isCardinalityExceeded(normalizedMetadata[e.eventKeyLabel]) {
		log.Warnf("event key '%s' exceeded limit '%d', masked as '%s'", normalizedMetadata[e.eventKeyLabel], e.eventKeyLimit, eventKeyCardinalityLimitReplacement)
		normalizedMetadata[e.eventKeyLabel] = eventKeyCardinalityLimitReplacement
	}

	e.initializeMetricForGivenMetadata(normalizedMetadata)

	// add result to metadata
	normalizedMetadata[sloEventResultLabel] = string(event.Result)
	e.eventsCount.With(prometheus.Labels(normalizedMetadata)).Inc()
	return nil
}
