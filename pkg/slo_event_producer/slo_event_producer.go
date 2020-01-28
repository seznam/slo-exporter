package slo_event_producer

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"time"
)

var (
	log                     *logrus.Entry
	generatedSloEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "slo_exporter",
		Subsystem: "slo_event_producer",
		Name:      "generated_slo_events_total",
		Help:      "Total number of generated SLO events per type.",
	}, []string{"type"})

	unclassifiedEventsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "slo_exporter",
		Subsystem: "slo_event_producer",
		Name:      "unclassified_events_total",
		Help:      "Total number of dropped events without classification.",
	})
)

func init() {
	log = logrus.WithField("component", "slo_event_producer")
	prometheus.MustRegister(generatedSloEventsTotal, unclassifiedEventsTotal)
}

type ClassifiableEvent interface {
	GetAvailabilityResult() bool
	GetLatencyResult(time.Duration) bool
	GetSloMetadata() *map[string]string
}

type SloEvent struct {
	Result      bool
	SloMetadata *map[string]string
}

func (se *SloEvent) String() string {
	return fmt.Sprintf("SloEvent result: %v  identifiers: %v", se.Result, se.SloMetadata)
}

func NewSloEventProducer(latencyBuckets []time.Duration) *SloEventProducer {
	return &SloEventProducer{latencyThresholds: latencyBuckets}
}

type SloEventProducer struct {
	latencyThresholds []time.Duration
}

func metadataWithSloType(metadata *map[string]string, sloType string) *map[string]string {
	newMetadata := map[string]string{}
	for k, v := range *metadata {
		newMetadata[k] = v
	}
	newMetadata["slo_type"] = sloType
	return &newMetadata
}

func availabilityMetadata(metadata *map[string]string) *map[string]string {
	return metadataWithSloType(metadata, "availability")
}

func latencyMetadata(metadata *map[string]string, threshold time.Duration) *map[string]string {
	m := *metadataWithSloType(metadata, "latency")
	m["le"] = fmt.Sprintf("%g", threshold.Seconds())
	return &m
}

func (sep *SloEventProducer) generateSLOEvents(event ClassifiableEvent, sloEventsChan chan<- *SloEvent) {
	metadata := event.GetSloMetadata()
	if metadata == nil {
		log.Warnf("dropping unclassified event")
		unclassifiedEventsTotal.Inc()
		return
	}

	sloEventsChan <- &SloEvent{
		Result:      event.GetAvailabilityResult(),
		SloMetadata: availabilityMetadata(metadata),
	}
	generatedSloEventsTotal.WithLabelValues("availability").Inc()
	for _, threshold := range sep.latencyThresholds {
		sloEventsChan <- &SloEvent{
			Result:      event.GetLatencyResult(threshold),
			SloMetadata: latencyMetadata(metadata, threshold),
		}
		generatedSloEventsTotal.WithLabelValues("latency").Inc()
	}
}

// TODO move to interfaces in channels, those cannot be mixed so we have to stick to one type now
func (sep *SloEventProducer) Run(ctx context.Context, inputEventChan <-chan *producer.RequestEvent, outputSLOEventChan chan<- *SloEvent) {
	go func() {
		defer close(outputSLOEventChan)
		defer log.Info("stopping slo_event_producer")
		for {
			select {
			case <-ctx.Done():
				return
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
