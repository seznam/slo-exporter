package slo_event_producer

import (
	"context"
	"fmt"
	"github.com/go-test/deep"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"testing"
	"time"
)

type latencyMetadataTestCase struct {
	metadata         map[string]string
	expectedMetadata map[string]string
	threshold        time.Duration
}

func TestSloEventProducer_latencyMetadata(t *testing.T) {
	testCases := []latencyMetadataTestCase{
		{metadata: map[string]string{}, expectedMetadata: map[string]string{"slo_type": "latency", "le": "1"}, threshold: time.Second},
		{metadata: map[string]string{}, expectedMetadata: map[string]string{"slo_type": "latency", "le": "0.001"}, threshold: 1 * time.Millisecond},
		{metadata: map[string]string{}, expectedMetadata: map[string]string{"slo_type": "latency", "le": "0"}, threshold: 0},
	}
	for _, tc := range testCases {
		newMetadata := *latencyMetadata(&tc.metadata, tc.threshold)
		if diff := deep.Equal(tc.expectedMetadata, newMetadata); diff != nil {
			t.Error(diff)
		}
	}
}

type sloEventTestCase struct {
	inputEvent        producer.RequestEvent
	expectedSloEvents []SloEvent
	thresholds        []time.Duration
}

func TestSloEventProducer(t *testing.T) {
	testCases := []sloEventTestCase{
		{
			inputEvent: producer.RequestEvent{Duration: 10 * time.Millisecond, StatusCode: 200, SloClassification: &producer.SloClassification{Class: "class", App: "app", Domain: "domain"}},
			thresholds: []time.Duration{1 * time.Second},
			expectedSloEvents: []SloEvent{
				{Result: true, SloMetadata: &map[string]string{"slo_type": "availability", "slo_domain": "domain", "slo_class": "class", "app": "app", "endpoint": ""}},
				{Result: true, SloMetadata: &map[string]string{"le": "1", "slo_type": "latency", "slo_domain": "domain", "slo_class": "class", "app": "app", "endpoint": ""}},
			},
		},
		{
			inputEvent: producer.RequestEvent{Duration: 10 * time.Second, StatusCode: 503, SloClassification: &producer.SloClassification{Class: "class", App: "app", Domain: "domain"}},
			thresholds: []time.Duration{1 * time.Second},
			expectedSloEvents: []SloEvent{
				{Result: false, SloMetadata: &map[string]string{"slo_type": "availability", "slo_domain": "domain", "slo_class": "class", "app": "app", "endpoint": ""}},
				{Result: false, SloMetadata: &map[string]string{"le": "1", "slo_type": "latency", "slo_domain": "domain", "slo_class": "class", "app": "app", "endpoint": ""}},
			},
		},
		{
			inputEvent: producer.RequestEvent{Duration: 2 * time.Second, StatusCode: 200, SloClassification: &producer.SloClassification{Class: "class", App: "app", Domain: "domain"}},
			thresholds: []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second},
			expectedSloEvents: []SloEvent{
				{Result: true, SloMetadata: &map[string]string{"slo_type": "availability", "slo_domain": "domain", "slo_class": "class", "app": "app", "endpoint": ""}},
				{Result: false, SloMetadata: &map[string]string{"le": "1", "slo_type": "latency", "slo_domain": "domain", "slo_class": "class", "app": "app", "endpoint": ""}},
				{Result: true, SloMetadata: &map[string]string{"le": "2", "slo_type": "latency", "slo_domain": "domain", "slo_class": "class", "app": "app", "endpoint": ""}},
				{Result: true, SloMetadata: &map[string]string{"le": "3", "slo_type": "latency", "slo_domain": "domain", "slo_class": "class", "app": "app", "endpoint": ""}},
			},
		},
	}

	for _, tc := range testCases {
		fmt.Println("==================")
		ctx, cancel := context.WithCancel(context.Background())
		in := make(chan *producer.RequestEvent)
		out := make(chan *SloEvent)
		testedProducer := NewSloEventProducer(tc.thresholds)
		testedProducer.Run(ctx, in, out)
		in <- &tc.inputEvent
		close(in)
		var results []SloEvent
		for event := range out {
			fmt.Println(event)
			results = append(results, *event)
		}
		cancel()
		if diff := deep.Equal(tc.expectedSloEvents, results); diff != nil {
			t.Error(diff)
		}
	}
}
