//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"time"

	"github.com/sirupsen/logrus"
)

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
	RulesFiles []string
}

func NewFromViper(viperConfig *viper.Viper, logger *logrus.Entry) (*SloEventProducer, error) {
	var config sloEventProducerConfig
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return New(config, logger)
}

func New(config sloEventProducerConfig, logger *logrus.Entry) (*SloEventProducer, error) {
	eventEvaluator, err := NewEventEvaluatorFromConfigFiles(config.RulesFiles, logger)
	if err != nil {
		return nil, err
	}
	return &SloEventProducer{
		eventEvaluator: eventEvaluator,
		inputChannel:   make(chan *event.HttpRequest),
		outputChannel:  make(chan *event.Slo),
		logger:         logger,
		done:           false,
	}, nil
}

type SloEventProducer struct {
	eventEvaluator *EventEvaluator
	observer       prometheus.Observer
	inputChannel   chan *event.HttpRequest
	outputChannel  chan *event.Slo
	logger         *logrus.Entry
	done           bool
}

func (sep *SloEventProducer) String() string {
	return "sloEventProducer"
}

func (sep *SloEventProducer) OutputChannel() chan *event.Slo {
	return sep.outputChannel
}

func (sep *SloEventProducer) SetInputChannel(channel chan *event.HttpRequest) {
	sep.inputChannel = channel
}

func (sep *SloEventProducer) RegisterMetrics(_ prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	toRegister := []prometheus.Collector{didNotMatchAnyRule, evaluationDurationSeconds, unclassifiedEventsTotal}
	for _, collector := range toRegister {
		if err := wrappedRegistry.Register(collector); err != nil {
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

func (sep *SloEventProducer) SetPrometheusObserver(observer prometheus.Observer) {
	sep.observer = observer
}

func (sep *SloEventProducer) observeDuration(start time.Time) {
	if sep.observer != nil {
		sep.observer.Observe(time.Since(start).Seconds())
	}
}

func (sep *SloEventProducer) generateSLOEvents(event *event.HttpRequest, sloEventsChan chan<- *event.Slo) {
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
