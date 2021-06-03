//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/pipeline"
	"github.com/spf13/viper"

	"github.com/sirupsen/logrus"
)

var ErrUnknowEvaluatorType = errors.New("unkonwn evaluator type")

var (
	unclassifiedEventsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "unclassified_events_total",
		Help: "Total number of dropped events without classification.",
	})

	didNotMatchAnyRule = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "events_not_matching_any_rule_total",
		Help: "Total number of events not matching any SLO rule.",
	})

	evaluationDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "evaluation_duration_seconds",
		Help:    "Histogram of event evaluation duration.",
		Buckets: prometheus.ExponentialBuckets(0.0001, 5, 7),
	})
)

type sloEventProducerConfig struct {
	ExposeRulesAsMetrics bool
	RulesFiles           []string
	EvaulatorType        string
}

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*SloEventProducer, error) {
	var config sloEventProducerConfig
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	viperConfig.SetDefault("ExposeRulesAsMetrics", false)
	viperConfig.SetDefault("EvaulatorType", "yaml")
	return New(config, logger)
}

func New(config sloEventProducerConfig, logger logrus.FieldLogger) (*SloEventProducer, error) {

	var eventEvaluator EventEvaluator
	var err error

	switch config.EvaulatorType {
	case "yaml":
		eventEvaluator, err = NewYamlEventEvaluatorFromConfigFiles(config.RulesFiles, logger)
	case "expr":
		eventEvaluator, err = NewExprEventEvaluatorFromConfigFiles(config.RulesFiles, logger)
	default:
		err = ErrUnknowEvaluatorType
	}

	if err != nil {
		return nil, err
	}
	return &SloEventProducer{
		eventEvaluator:       eventEvaluator,
		inputChannel:         make(chan *event.Raw),
		outputChannel:        make(chan *event.Slo),
		logger:               logger,
		exposeRulesInMetrics: config.ExposeRulesAsMetrics,
		done:                 false,
	}, nil
}

type EventEvaluator interface {
	registerMetrics(prometheus.Registerer) error
	Evaluate(newEvent *event.Raw, outChan chan<- *event.Slo)
}

type SloEventProducer struct {
	eventEvaluator       EventEvaluator
	observer             pipeline.EventProcessingDurationObserver
	inputChannel         chan *event.Raw
	outputChannel        chan *event.Slo
	logger               logrus.FieldLogger
	exposeRulesInMetrics bool
	done                 bool
}

func (sep *SloEventProducer) String() string {
	return "sloEventProducer"
}

func (sep *SloEventProducer) OutputChannel() chan *event.Slo {
	return sep.outputChannel
}

func (sep *SloEventProducer) SetInputChannel(channel chan *event.Raw) {
	sep.inputChannel = channel
}

func (sep *SloEventProducer) RegisterMetrics(_ prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	toRegister := []prometheus.Collector{didNotMatchAnyRule, evaluationDurationSeconds, unclassifiedEventsTotal}
	for _, collector := range toRegister {
		if err := wrappedRegistry.Register(collector); err != nil {
			return err
		}
	}
	if sep.exposeRulesInMetrics {
		if err := sep.eventEvaluator.registerMetrics(wrappedRegistry); err != nil {
			return err
		}
	}
	return nil
}

func (sep *SloEventProducer) Stop() {
	return
}

func (sep *SloEventProducer) Done() bool {
	return sep.done
}

func (sep *SloEventProducer) RegisterEventProcessingDurationObserver(observer pipeline.EventProcessingDurationObserver) {
	sep.observer = observer
}

func (sep *SloEventProducer) observeDuration(start time.Time) {
	if sep.observer != nil {
		sep.observer.Observe(time.Since(start).Seconds())
	}
}

func (sep *SloEventProducer) generateSLOEvents(event *event.Raw, sloEventsChan chan<- *event.Slo) {
	sep.eventEvaluator.Evaluate(event, sloEventsChan)
}

func (sep *SloEventProducer) Run() {
	go func() {
		defer func() {
			close(sep.outputChannel)
			sep.done = true
		}()
		for newEvent := range sep.inputChannel {
			start := time.Now()
			sep.generateSLOEvents(newEvent, sep.outputChannel)
			sep.observeDuration(start)
		}
		sep.logger.Info("input channel closed, finishing")
	}()
}
