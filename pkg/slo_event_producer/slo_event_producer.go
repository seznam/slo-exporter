//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

const (
	SloEventResultSuccess SloEventResult = "success"
	SloEventResultFail    SloEventResult = "fail"
)

var (
	log                     *logrus.Entry
	generatedSloEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "slo_exporter",
		Subsystem: "slo_event_producer",
		Name:      "generated_slo_events_total",
		Help:      "Total number of generated SLO events per type.",
	}, []string{"type"})
	EventResults = []SloEventResult{SloEventResultSuccess, SloEventResultFail}
)

func init() {
	log = logrus.WithField("component", "slo_event_producer")
	prometheus.MustRegister(generatedSloEventsTotal)
}

type ClassifiableEvent interface {
	GetEventKey() string
	IsClassified() bool
	GetSloMetadata() *map[string]string
	GetTimeOccurred() time.Time
}

type SloEventResult string

type SloEvent struct {
	TimeOccurred time.Time
	SloMetadata  map[string]string
	Result       SloEventResult
}

func (se *SloEvent) String() string {
	return fmt.Sprintf("SloEvent %v", se.SloMetadata)
}

func NewSloEventProducer(configPath string) (*SloEventProducer, error) {
	eventEvaluator, err := NewEventEvaluatorFromConfigFile(configPath)
	if err != nil {
		return nil, err
	}
	return &SloEventProducer{eventEvaluator: eventEvaluator}, nil
}

type SloEventProducer struct {
	eventEvaluator EventEvaluator
}

func (sep *SloEventProducer) PossibleMetadataKeys() []string {
	return sep.eventEvaluator.PossibleMetadataKeys()
}

func (sep *SloEventProducer) generateSLOEvents(event *producer.RequestEvent, sloEventsChan chan<- *SloEvent) {
	sep.eventEvaluator.Evaluate(event, sloEventsChan)
}

// TODO move to interfaces in channels, those cannot be mixed so we have to stick to one type now
func (sep *SloEventProducer) Run(inputEventChan <-chan *producer.RequestEvent, outputSLOEventChan chan<- *SloEvent) {
	go func() {
		defer close(outputSLOEventChan)
		defer log.Info("stopping...")

		for {
			select {
			case event, ok := <-inputEventChan:
				if !ok {
					log.Info("input channel closed, finishing")
					return
				}
				sep.generateSLOEvents(event, outputSLOEventChan)
			}
		}
	}()
}
