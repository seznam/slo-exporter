//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/go-test/deep"
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"testing"
)

type sloEventTestCase struct {
	inputEvent        event.HttpRequest
	expectedSloEvents []event.Slo
	rulesConfig       rulesConfig
}

func TestSloEventProducer(t *testing.T) {
	testCases := []sloEventTestCase{
		{
			inputEvent: event.HttpRequest{Metadata: stringmap.StringMap{"statusCode": "502"}, SloClassification: &event.SloClassification{Class: "class", App: "app", Domain: "domain"}},
			rulesConfig: rulesConfig{Rules: []ruleOptions{
				{
					EventType:  "request",
					SloMatcher: sloMatcher{Domain: "domain"},
					FailureConditionsOptions: []operatorOptions{
						{Operator: "numberHigherThan", Key: "statusCode", Value: "500"},
					},
					AdditionalMetadata: stringmap.StringMap{"slo_type": "availability"},
				},
			},
			},
			expectedSloEvents: []event.Slo{
				{Domain: "domain", Class: "class", App: "app", Key: "", Metadata: stringmap.StringMap{"slo_type": "availability"}, Result: event.Fail},
			},
		},
		{
			inputEvent: event.HttpRequest{Metadata: stringmap.StringMap{"statusCode": "200"}, SloClassification: &event.SloClassification{Class: "class", App: "app", Domain: "domain"}},
			rulesConfig: rulesConfig{Rules: []ruleOptions{
				{
					EventType:  "request",
					SloMatcher: sloMatcher{Domain: "domain"},
					FailureConditionsOptions: []operatorOptions{
						{Operator: "numberHigherThan", Key: "statusCode", Value: "500"},
					},
					AdditionalMetadata: stringmap.StringMap{"slo_type": "availability"},
				},
			},
			},
			expectedSloEvents: []event.Slo{
				{Domain: "domain", Class: "class", App: "app", Key: "", Metadata: stringmap.StringMap{"slo_type": "availability"}, Result: event.Success},
			},
		},
	}

	for _, tc := range testCases {
		out := make(chan *event.Slo, 100)
		testedEvaluator, err := NewEventEvaluatorFromConfig(&tc.rulesConfig)
		if err != nil {
			t.Errorf("error when loading config: %v error: %v", tc.rulesConfig, err)
			continue
		}
		testedEvaluator.Evaluate(&tc.inputEvent, out)
		close(out)
		var results []event.Slo
		for newEvent := range out {
			results = append(results, *newEvent)
		}
		if diff := deep.Equal(tc.expectedSloEvents, results); diff != nil {
			t.Errorf("events are different %+v, \nexpected: %+v\n result: %+v\n input event metadata: %+v", diff, tc.expectedSloEvents, results, tc.inputEvent.Metadata)
		}
	}
}

func TestSloEventProducer_PossibleMetadataKeys(t *testing.T) {
	config := rulesConfig{Rules: []ruleOptions{
		{
			EventType:                "request",
			SloMatcher:               sloMatcher{},
			FailureConditionsOptions: []operatorOptions{},
			AdditionalMetadata:       stringmap.StringMap{"test1": "foo"},
		},
		{
			EventType:                "request",
			SloMatcher:               sloMatcher{},
			FailureConditionsOptions: []operatorOptions{},
			AdditionalMetadata:       stringmap.StringMap{"test2": "bar"},
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
