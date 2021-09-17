package prometheus_exporter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/pipeline"
	"github.com/seznam/slo-exporter/pkg/stringmap"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"time"
)

const (
	metricHelp = "Total number of SLO events exported with it's result and metadata."
)

var (
	errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "errors_total",
			Help:        "Errors occurred during application runtime",
			ConstLabels: prometheus.Labels{"app": "slo_exporter"},
		},
		[]string{"type"})
	eventKeys = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name:        "event_keys",
			Help:        "Number of known unique event keys",
			ConstLabels: prometheus.Labels{"app": "slo_exporter"},
		})
	eventKeyCardinalityLimit = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "event_keys_limit",
		Help:        "Event keys cardinality limit",
		ConstLabels: prometheus.Labels{"app": "slo_exporter"},
	})
)

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
	ExemplarMetadataKeys        []string
}

type PrometheusSloEventExporter struct {
	aggregatedMetricsSet        *aggregatedCounterVectorSet
	metricName                  string
	labelNames                  labelsNamesConfig
	eventKeyLimit               int
	exceededKeyLimitPlaceholder string
	exemplarMetadataKeys        []string
	eventKeyCache               map[string]int
	observer                    pipeline.EventProcessingDurationObserver

	inputChannel chan *event.Slo
	done         bool
	logger       logrus.FieldLogger
}

type InvalidSloEventResult struct {
	result       string
	validResults []event.Result
}

func (e *InvalidSloEventResult) Error() string {
	return fmt.Sprintf("result '%s' is not valid. Expected one of: %+v", e.result, e.validResults)
}

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*PrometheusSloEventExporter, error) {
	config := prometheusExporterConfig{}
	viperConfig.SetDefault("MetricName", "slo_events_total")
	viperConfig.SetDefault("LabelNames.Result", "result")
	viperConfig.SetDefault("LabelNames.SloDomain", "slo_domain")
	viperConfig.SetDefault("LabelNames.SloClass", "slo_class")
	viperConfig.SetDefault("LabelNames.SloApp", "slo_app")
	viperConfig.SetDefault("LabelNames.EventKey", "event_key")
	viperConfig.SetDefault("exceededKeyLimitPlaceholder", "cardinalityLimitExceeded")
	viperConfig.SetDefault("exemplarMetadataKeys", "[]")
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return New(config, logger)
}

func New(config prometheusExporterConfig, logger logrus.FieldLogger) (*PrometheusSloEventExporter, error) {
	// initialize and register Prometheus metrics
	eventKeyCardinalityLimit.Set(float64(config.MaximumUniqueEventKeys))

	aggregationLabels := []string{config.LabelNames.SloDomain, config.LabelNames.SloClass, config.LabelNames.SloApp, config.LabelNames.EventKey}
	newAggregatedMetricsSet := newAggregatedCounterVectorSet(config.MetricName, metricHelp, aggregationLabels, logger, config.ExemplarMetadataKeys)

	return &PrometheusSloEventExporter{
		aggregatedMetricsSet: newAggregatedMetricsSet,
		metricName:           config.MetricName,
		labelNames:           config.LabelNames,

		eventKeyLimit:               config.MaximumUniqueEventKeys,
		exceededKeyLimitPlaceholder: config.ExceededKeyLimitPlaceholder,
		eventKeyCache:               map[string]int{},

		exemplarMetadataKeys: config.ExemplarMetadataKeys,

		logger:   logger,
		observer: nil,
	}, nil
}

func (e *PrometheusSloEventExporter) RegisterMetrics(rootRegistry prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	if err := e.aggregatedMetricsSet.register(rootRegistry); err != nil {
		return err
	}
	toRegister := []prometheus.Collector{eventKeyCardinalityLimit, errorsTotal, eventKeys}
	for _, metric := range toRegister {
		if err := wrappedRegistry.Register(metric); err != nil {
			return err
		}
	}
	return nil
}

func (e *PrometheusSloEventExporter) String() string {
	return "prometheusExporter"
}

func (e *PrometheusSloEventExporter) Stop() {
	return
}

func (e *PrometheusSloEventExporter) Done() bool {
	return e.done
}

func (e *PrometheusSloEventExporter) SetInputChannel(channel chan *event.Slo) {
	e.inputChannel = channel
}

func (e *PrometheusSloEventExporter) Run() {
	go func() {
		for newEvent := range e.inputChannel {
			start := time.Now()
			e.logger.Debugf("processing event %s", newEvent)
			err := e.processEvent(newEvent)
			if err != nil {
				e.logger.Errorf("unable to process slo event: %+v", err)
				switch err.(type) {
				case *InvalidSloEventResult:
					errorsTotal.With(prometheus.Labels{"type": "InvalidResult"}).Inc()
				default:
					errorsTotal.With(prometheus.Labels{"type": "Unknown"}).Inc()
				}
			}
			e.observeDuration(start)
		}
		e.logger.Info("input channel closed, finishing")
		e.done = true
	}()
}

func (e *PrometheusSloEventExporter) RegisterEventProcessingDurationObserver(observer pipeline.EventProcessingDurationObserver) {
	e.observer = observer
}

func (e *PrometheusSloEventExporter) observeDuration(start time.Time) {
	if e.observer != nil {
		e.observer.Observe(time.Since(start).Seconds())
	}
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
	for _, validEventResult := range event.PossibleResults {
		if validEventResult == result {
			return true
		}
	}
	return false
}

// for given ev metadata, initialize exposed metric for all possible result label values
func (e *PrometheusSloEventExporter) initializeMetricForGivenMetadata(metadata stringmap.StringMap) {
	for _, result := range event.PossibleResults {
		metadata[e.labelNames.Result] = string(result)
		e.aggregatedMetricsSet.add(0, metadata)
	}
}

func (e *PrometheusSloEventExporter) labelsFromEvent(sloEvent *event.Slo) stringmap.StringMap {
	return stringmap.StringMap{
		e.labelNames.Result:    string(sloEvent.Result),
		e.labelNames.SloDomain: sloEvent.Domain,
		e.labelNames.SloClass:  sloEvent.Class,
		e.labelNames.SloApp:    sloEvent.App,
		e.labelNames.EventKey:  sloEvent.Key,
	}.Merge(sloEvent.Metadata)
}

func (e *PrometheusSloEventExporter) processEvent(newEvent *event.Slo) error {
	if !e.isValidResult(newEvent.Result) {
		return &InvalidSloEventResult{string(newEvent.Result), event.PossibleResults}
	}

	labels := e.labelsFromEvent(newEvent)

	if e.isCardinalityExceeded(newEvent.Key) {
		e.logger.Warnf("event key '%s' exceeded limit '%d', masked as '%s'", newEvent.Key, e.eventKeyLimit, e.exceededKeyLimitPlaceholder)
		labels[e.labelNames.EventKey] = e.exceededKeyLimitPlaceholder
	}

	e.initializeMetricForGivenMetadata(labels)

	// add result to metadata
	labels[e.labelNames.Result] = string(newEvent.Result)
	if len(e.exemplarMetadataKeys) > 0 {
		e.aggregatedMetricsSet.addWithExemplar(newEvent.Quantity, labels, newEvent.OriginalEvent.Metadata.Select(e.exemplarMetadataKeys))
	} else {
		e.aggregatedMetricsSet.add(newEvent.Quantity, labels)
	}
	return nil
}
