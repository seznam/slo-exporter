//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"regexp"
	"testing"
)

type ruleTestCase struct {
	rule           evaluationRule
	inputEvent     event.HttpRequest
	outputSloEvent *event.Slo
	ok             bool
}

func TestEvaluateEvent(t *testing.T) {
	testCases := []ruleTestCase{
		{
			rule: evaluationRule{
				additionalMetadata: stringmap.StringMap{},
				failureConditions:  []operator{&matchesRegexp{key: "statusCode", regexp: regexp.MustCompile("500")}},
			},
			inputEvent: event.HttpRequest{
				Metadata:          stringmap.StringMap{"statusCode": "200"},
				SloClassification: &event.SloClassification{Class: "class", App: "app", Domain: "domain"},
			},
			outputSloEvent: &event.Slo{Domain: "domain", Class: "class", App: "app", Key: "", Metadata: stringmap.StringMap{}, Result: event.Success},
			ok:             true,
		},
		{
			rule: evaluationRule{
				additionalMetadata: stringmap.StringMap{},
				failureConditions:  []operator{&matchesRegexp{key: "statusCode", regexp: regexp.MustCompile("500")}},
				honorSloResult:     true,
			},
			inputEvent: event.HttpRequest{
				SloResult:         string(event.Success),
				Metadata:          stringmap.StringMap{"statusCode": "502"},
				SloClassification: &event.SloClassification{Class: "class", App: "app", Domain: "domain"},
			},
			outputSloEvent: &event.Slo{Domain: "domain", Class: "class", App: "app", Key: "", Metadata: stringmap.StringMap{}, Result: event.Success},
			ok:             true,
		},
		{
			rule: evaluationRule{
				additionalMetadata: stringmap.StringMap{},
				failureConditions:  []operator{&matchesRegexp{key: "statusCode", regexp: regexp.MustCompile("500")}},
				honorSloResult:     true,
			},
			inputEvent: event.HttpRequest{
				SloResult:         string(event.Fail),
				Metadata:          stringmap.StringMap{"statusCode": "200"},
				SloClassification: &event.SloClassification{Class: "class", App: "app", Domain: "domain"},
			},
			outputSloEvent: &event.Slo{Domain: "domain", Class: "class", App: "app", Key: "", Metadata: stringmap.StringMap{}, Result: event.Fail},
			ok:             true,
		},
		{
			rule: evaluationRule{
				additionalMetadata: stringmap.StringMap{},
				failureConditions:  []operator{&matchesRegexp{key: "statusCode", regexp: regexp.MustCompile("502")}},
			},
			inputEvent: event.HttpRequest{
				SloResult:         string(event.Success),
				Metadata:          stringmap.StringMap{"statusCode": "502"},
				SloClassification: &event.SloClassification{Class: "class", App: "app", Domain: "domain"},
			},
			outputSloEvent: &event.Slo{Domain: "domain", Class: "class", App: "app", Key: "", Metadata: stringmap.StringMap{}, Result: event.Fail},
			ok:             true,
		},
		{
			rule: evaluationRule{
				additionalMetadata: stringmap.StringMap{},
				failureConditions:  []operator{&matchesRegexp{key: "statusCode", regexp: regexp.MustCompile("500")}},
			},
			inputEvent: event.HttpRequest{
				SloResult:         string(event.Fail),
				Metadata:          stringmap.StringMap{"statusCode": "502"},
				SloClassification: &event.SloClassification{Class: "class", App: "app", Domain: "domain"},
			},
			outputSloEvent: &event.Slo{Domain: "domain", Class: "class", App: "app", Key: "", Metadata: stringmap.StringMap{}, Result: event.Success},
			ok:             true,
		},
		{
			rule: evaluationRule{
				additionalMetadata: stringmap.StringMap{},
				failureConditions:  []operator{&matchesRegexp{key: "statusCode", regexp: regexp.MustCompile("500")}},
			},
			inputEvent: event.HttpRequest{
				Metadata:          stringmap.StringMap{"statusCode": "502"},
				SloClassification: nil,
			},
			outputSloEvent: nil,
			ok:             false,
		},
		{
			rule: evaluationRule{
				sloMatcher:         event.SloClassification{Domain: "foo"},
				additionalMetadata: stringmap.StringMap{},
				failureConditions:  []operator{&matchesRegexp{key: "statusCode", regexp: regexp.MustCompile("500")}},
			},
			inputEvent: event.HttpRequest{
				Metadata:          stringmap.StringMap{"statusCode": "502"},
				SloClassification: &event.SloClassification{Class: "class", App: "app", Domain: "domain"},
			},
			outputSloEvent: nil,
			ok:             false,
		},
	}

	for _, tc := range testCases {
		sloEvent, ok := tc.rule.processEvent(&tc.inputEvent)
		assert.Equal(t, ok, tc.ok)
		assert.Equal(t, tc.outputSloEvent, sloEvent, "unexpected result evaluating rule: %+v\n  with conditions: %+v\non event:\n  metadata: %+v\n  classification: %+v",tc.rule,tc.rule.failureConditions[0], tc.inputEvent.Metadata, tc.inputEvent.SloClassification)
	}
}

func
TestPossibleLabels(t *testing.T) {
	rule := evaluationRule{
		additionalMetadata: stringmap.StringMap{"label": "value"},
		failureConditions:  []operator{&matchesRegexp{key: "statusCode", regexp: regexp.MustCompile("500")}},
	}
	expectedMetadata := []string{"label"}
	result := rule.PossibleMetadataKeys()
	assert.ElementsMatch(t, expectedMetadata, result)
}
