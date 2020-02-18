//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/go-test/deep"
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"testing"
	"time"
)

type sloEventTestCase struct {
	inputEvent        producer.RequestEvent
	expectedSloEvents []event.Slo
	rulesConfig       rulesConfig
}

func TestSloEventProducer(t *testing.T) {
	testCases := []sloEventTestCase{
		{
			inputEvent: producer.RequestEvent{Duration: time.Second, StatusCode: 500, SloClassification: &producer.SloClassification{Class: "class", App: "app", Domain: "domain"}},
			rulesConfig: rulesConfig{Rules: []ruleOptions{
				{
					EventType:  "request",
					SloMatcher: sloMatcher{Domain: "domain"},
					FailureCriteriaOptions: []criteriumOptions{
						{Criterium: "requestStatusHigherThan", Value: "500"},
					},
					AdditionalMetadata: stringmap.StringMap{"slo_type": "availability"},
				},
			},
			},
			expectedSloEvents: []event.Slo{
				{Domain:"domain", Class:"class", App:"app", Key: "", Metadata: stringmap.StringMap{"slo_type": "availability"}, Result: event.Success},
			},
		},
		{
			inputEvent: producer.RequestEvent{Duration: time.Second, StatusCode: 200, SloClassification: &producer.SloClassification{Class: "class", App: "app", Domain: "domain"}},
			rulesConfig: rulesConfig{Rules: []ruleOptions{
				{
					EventType:  "request",
					SloMatcher: sloMatcher{Domain: "domain"},
					FailureCriteriaOptions: []criteriumOptions{
						{Criterium: "requestDurationHigherThan", Value: "0.5s"},
					},
					AdditionalMetadata: stringmap.StringMap{"slo_type": "availability"},
				},
			},
			},
			expectedSloEvents: []event.Slo{
				{Domain:"domain", Class:"class", App:"app", Key: "", Metadata: stringmap.StringMap{"slo_type": "availability"}, Result: event.Fail},
			},
		},
	}

	for _, tc := range testCases {
		out := make(chan *event.Slo, 100)
		testedEvaluator, err := NewEventEvaluatorFromConfig(&tc.rulesConfig)
		if err != nil {
			t.Error(err)
		}
		testedEvaluator.Evaluate(&tc.inputEvent, out)
		close(out)
		var results []event.Slo
		for newEvent := range out {
			results = append(results, *newEvent)
		}
		if diff := deep.Equal(tc.expectedSloEvents, results); diff != nil {
			t.Errorf("events are different %+v, \nexpected: %+v\n result: %+v\n", diff, tc.expectedSloEvents, results)
		}
	}
}

func TestSloEventProducer_PossibleMetadataKeys(t *testing.T) {
	config := rulesConfig{Rules: []ruleOptions{
		{
			EventType:              "request",
			SloMatcher:             sloMatcher{},
			FailureCriteriaOptions: []criteriumOptions{},
			AdditionalMetadata:     stringmap.StringMap{"test1": "foo"},
		},
		{
			EventType:              "request",
			SloMatcher:             sloMatcher{},
			FailureCriteriaOptions: []criteriumOptions{},
			AdditionalMetadata:     stringmap.StringMap{"test2": "bar"},
		},
	},
	}
	expectedKeys := []string{"test1", "test2"}

	evaluator, err := NewEventEvaluatorFromConfig(&config)
	if err != nil {
		t.Error(err)
	}
	possibleKeys := evaluator.PossibleMetadataKeys()

	assert.ElementsMatch(t, possibleKeys, expectedKeys)

}
