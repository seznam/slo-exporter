//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/go-test/deep"
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"testing"
	"time"
)

type sloEventTestCase struct {
	inputEvent        producer.RequestEvent
	expectedSloEvents []SloEvent
	rulesConfig       rulesConfig
}

func TestSloEventProducer(t *testing.T) {
	testCases := []sloEventTestCase{
		{
			inputEvent: producer.RequestEvent{Duration: time.Second, StatusCode: 500, SloClassification: &producer.SloClassification{Class: "class", App: "app", Domain: "domain"}},
			rulesConfig: rulesConfig{Rules: []ruleOptions{
				{
					EventType: "request",
					Matcher:   eventMetadata{"slo_domain": "domain"},
					FailureCriteriaOptions: []criteriumOptions{
						{Criterium: "requestStatusHigherThan", Value: "500"},
					},
					AdditionalMetadata: eventMetadata{"slo_type": "availability"},
				},
			},
			},
			expectedSloEvents: []SloEvent{
				{SloMetadata: map[string]string{"slo_type": "availability", "slo_domain": "domain", "slo_class": "class", "app": "app", "event_key": ""}, Failed: false},
			},
		},
		{
			inputEvent: producer.RequestEvent{Duration: time.Second, StatusCode: 200, SloClassification: &producer.SloClassification{Class: "class", App: "app", Domain: "domain"}},
			rulesConfig: rulesConfig{Rules: []ruleOptions{
				{
					EventType: "request",
					Matcher:   eventMetadata{"slo_domain": "domain"},
					FailureCriteriaOptions: []criteriumOptions{
						{Criterium: "requestDurationHigherThan", Value: "0.5s"},
					},
					AdditionalMetadata: eventMetadata{"slo_type": "availability"},
				},
			},
			},
			expectedSloEvents: []SloEvent{
				{SloMetadata: map[string]string{"slo_type": "availability", "slo_domain": "domain", "slo_class": "class", "app": "app", "event_key": ""}, Failed: true},
			},
		},
	}

	for _, tc := range testCases {
		out := make(chan *SloEvent, 100)
		testedEvaluator, err := NewEventEvaluatorFromConfig(&tc.rulesConfig)
		if err != nil {
			t.Error(err)
		}
		testedEvaluator.Evaluate(&tc.inputEvent, out)
		close(out)
		var results []SloEvent
		for event := range out {
			results = append(results, *event)
		}
		if diff := deep.Equal(tc.expectedSloEvents, results); diff != nil {
			t.Errorf("events are different %v, \nexpected: %v\n result: %v\n", diff, tc.expectedSloEvents, results)
		}
	}
}

func TestSloEventProducer_PossibleMetadataKeys(t *testing.T) {
	config := rulesConfig{Rules: []ruleOptions{
		{
			EventType:              "request",
			Matcher:                eventMetadata{},
			FailureCriteriaOptions: []criteriumOptions{},
			AdditionalMetadata:     eventMetadata{"test1": "foo"},
		},
		{
			EventType:              "request",
			Matcher:                eventMetadata{},
			FailureCriteriaOptions: []criteriumOptions{},
			AdditionalMetadata:     eventMetadata{"test2": "bar"},
		},
	},
	}
	expectedKeys := []string{"test1", "test2", "slo_domain", "slo_class", "app", "event_key"}

	evaluator, err := NewEventEvaluatorFromConfig(&config)
	if err != nil {
		t.Error(err)
	}
	possibleKeys := evaluator.PossibleMetadataKeys()

	assert.ElementsMatch(t, possibleKeys, expectedKeys)

}
