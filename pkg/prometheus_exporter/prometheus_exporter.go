package prometheus_exporter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/shutdown_handler"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"time"
)

const (
	component  = "prometheus_exporter"
	metricHelp = "Total number of SLO events exported with it's result and metadata."
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
)

func init() {
	log = logrus.WithField("component", component)
}

type labelsNamesConfig struct {
	Result    string
	SloDomain string
	SloClass  string
	SloApp    string
	EventKey  string
}

func (c labelsNamesConfig) keys() []string {
	return []string{c.EventKey, c.Result, c.SloApp, c.SloDomain, c.SloClass}
}

type prometheusExporterConfig struct {
	MetricName                  string
	LabelNames                  labelsNamesConfig
	MaximumUniqueEventKeys      int
	ExceededKeyLimitPlaceholder string
}

type PrometheusSloEventExporter struct {
	aggregatedMetricsSet        *aggregatedCounterSet
	knownLabels                 []string
	validEventResults           []event.Result
	metricName                  string
	labelNames                  labelsNamesConfig
	eventKeyLimit               int
	exceededKeyLimitPlaceholder string
	eventKeyCache               map[string]int
	observer                    prometheus.Observer
}

type InvalidSloEventResult struct {
	result       string
	validResults []event.Result
}

func (e *InvalidSloEventResult) Error() string {
	return fmt.Sprintf("result '%s' is not valid. Expected one of: %+v", e.result, e.validResults)
}

func NewFromViper(metricRegistry prometheus.Registerer, possibleLabels []string, possibleResults []event.Result, viperConfig *viper.Viper) (*PrometheusSloEventExporter, error) {
	config := prometheusExporterConfig{}
	viperConfig.SetDefault("MetricName", "slo_events_total")
	viperConfig.SetDefault("LabelNames.Result", "result")
	viperConfig.SetDefault("LabelNames.SloDomain", "slo_domain")
	viperConfig.SetDefault("LabelNames.SloClass", "slo_class")
	viperConfig.SetDefault("LabelNames.SloApp", "app")
	viperConfig.SetDefault("LabelNames.EventKey", "event_key")
	viperConfig.SetDefault("exceededKeyLimitPlaceholder", "cardinalityLimitExceeded")
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return New(metricRegistry, possibleLabels, possibleResults, config)
}

func New(metricRegistry prometheus.Registerer, possibleLabels []string, possibleResults []event.Result, config prometheusExporterConfig) (*PrometheusSloEventExporter, error) {
	knownLabels := append(possibleLabels, config.LabelNames.keys()...)

	// initialize and register Prometheus metrics
	eventKeyCardinalityLimit.Set(float64(config.MaximumUniqueEventKeys))
	metricRegistry.MustRegister(eventKeyCardinalityLimit, errorsTotal, eventKeys)

	newAggregatedMetricsSet, err := newAggregatedCounterSet(metricRegistry, config.MetricName, knownLabels, config.LabelNames)
	if err != nil {
		return nil, err
	}

	return &PrometheusSloEventExporter{
		aggregatedMetricsSet:        newAggregatedMetricsSet,
		knownLabels:                 knownLabels,
		validEventResults:           possibleResults,
		metricName:                  config.MetricName,
		labelNames:                  config.LabelNames,
		eventKeyLimit:               config.MaximumUniqueEventKeys,
		exceededKeyLimitPlaceholder: config.ExceededKeyLimitPlaceholder,
		eventKeyCache:               map[string]int{},
		observer:                    nil,
	}, nil
}

func (e *PrometheusSloEventExporter) Run(shutdownHandler *shutdown_handler.GracefulShutdownHandler, input <-chan *event.Slo) {
	go func() {
		for newEvent := range input {
			start := time.Now()
			err := e.processEvent(newEvent)
			if err != nil {
				log.Errorf("unable to process slo event: %+v", err)
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
		shutdownHandler.Done()
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
func (e *PrometheusSloEventExporter) normalizeEventMetadata(eventMetadata stringmap.StringMap) stringmap.StringMap {
	normalized := stringmap.StringMap{}
	for _, k := range e.knownLabels {
		normalized[k] = ""
	}
	return normalized.Merge(eventMetadata.Select(e.knownLabels))
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

func (e *PrometheusSloEventExporter) isValidResult(result event.Result) bool {
	for _, validEventResult := range e.validEventResults {
		if validEventResult == result {
			return true
		}
	}
	return false
}

// for given ev metadata, initialize exposed metric for all possible result label values
func (e *PrometheusSloEventExporter) initializeMetricForGivenMetadata(metadata stringmap.StringMap) {
	for _, result := range e.validEventResults {
		metadata[e.labelNames.Result] = string(result)
		e.aggregatedMetricsSet.add(0, metadata)
	}
}

func (e *PrometheusSloEventExporter) labelsFromEvent(sloEvent *event.Slo) stringmap.StringMap {
	return sloEvent.Metadata.Merge(stringmap.StringMap{
		e.labelNames.Result:    string(sloEvent.Result),
		e.labelNames.SloDomain: sloEvent.Domain,
		e.labelNames.SloClass:  sloEvent.Class,
		e.labelNames.SloApp:    sloEvent.App,
		e.labelNames.EventKey:  sloEvent.Key,
	})
}

func (e *PrometheusSloEventExporter) processEvent(newEvent *event.Slo) error {
	if !e.isValidResult(newEvent.Result) {
		return &InvalidSloEventResult{string(newEvent.Result), e.validEventResults}
	}

	labels := e.labelsFromEvent(newEvent)
	// Drop all unexpected labels
	normalizedLabels := e.normalizeEventMetadata(labels)

	if e.isCardinalityExceeded(newEvent.Key) {
		log.Warnf("ev key '%s' exceeded limit '%d', masked as '%s'", newEvent.Key, e.eventKeyLimit, e.exceededKeyLimitPlaceholder)
		normalizedLabels[e.labelNames.EventKey] = e.exceededKeyLimitPlaceholder
	}

	e.initializeMetricForGivenMetadata(normalizedLabels)

	// add result to metadata
	normalizedLabels[e.labelNames.Result] = string(newEvent.Result)
	e.aggregatedMetricsSet.inc(normalizedLabels)
	return nil
}
