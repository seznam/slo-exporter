//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"testing"
	"time"
)

type metadataMatchTestCase struct {
	a      eventMetadata
	b      eventMetadata
	result bool
}

func TestEventMetadata_matches(t *testing.T) {
	testCases := []metadataMatchTestCase{
		{a: eventMetadata{"a": "1"}, b: eventMetadata{"b": "2"}, result: false},
		{a: eventMetadata{"a": "1"}, b: eventMetadata{"a": "2"}, result: false},
		{a: eventMetadata{"a": "1"}, b: eventMetadata{"a": "1"}, result: true},
		{a: eventMetadata{}, b: eventMetadata{"a": "1"}, result: true},
		{a: eventMetadata{"a": "1"}, b: eventMetadata{}, result: false},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.a.matches(tc.b), tc.result)
	}
}

type metadataMergeTestCase struct {
	a      map[string]string
	b      map[string]string
	result map[string]string
}

func TestEventMetadata_merge(t *testing.T) {
	testCases := []metadataMergeTestCase{
		{a: eventMetadata{"a": "1"}, b: eventMetadata{"b": "2"}, result: eventMetadata{"a": "1", "b": "2"}},
		{a: eventMetadata{"a": "1"}, b: eventMetadata{"a": "2"}, result: eventMetadata{"a": "2"}},
		{a: eventMetadata{"a": "1"}, b: eventMetadata{}, result: eventMetadata{"a": "1"}},
		{a: eventMetadata{}, b: eventMetadata{"a": "1"}, result: eventMetadata{"a": "1"}},
		{a: eventMetadata{}, b: eventMetadata{}, result: eventMetadata{}},
	}

	for _, tc := range testCases {
		assert.Equal(t, mergeMetadata(tc.a, tc.b), tc.result)
	}
}

type ruleTestCase struct {
	rule           evaluationRule
	inputEvent     producer.RequestEvent
	outputSloEvent *SloEvent
	ok             bool
}

func TestEvaluateEvent(t *testing.T) {
	testCases := []ruleTestCase{
		{
			rule:           evaluationRule{matcher: eventMetadata{}, additionalMetadata: eventMetadata{}, failureCriteria: []criterium{&requestStatusHigherThan{statusThreshold: 500}}},
			inputEvent:     producer.RequestEvent{Duration: time.Second, StatusCode: 200, SloClassification: &producer.SloClassification{Class: "class", App: "app", Domain: "domain"}},
			outputSloEvent: &SloEvent{SloMetadata: map[string]string{"failed": "false", "slo_domain": "domain", "slo_class": "class", "app": "app", "event_key": ""}},
			ok:             true,
		},
		{
			rule:           evaluationRule{matcher: eventMetadata{}, additionalMetadata: eventMetadata{}, failureCriteria: []criterium{&requestStatusHigherThan{statusThreshold: 500}}},
			inputEvent:     producer.RequestEvent{Duration: time.Second, StatusCode: 200, SloClassification: nil},
			outputSloEvent: nil,
			ok:             false,
		},
		{
			rule:           evaluationRule{matcher: eventMetadata{"foo": "bar"}, additionalMetadata: eventMetadata{}, failureCriteria: []criterium{&requestStatusHigherThan{statusThreshold: 500}}},
			inputEvent:     producer.RequestEvent{Duration: time.Second, StatusCode: 200, SloClassification: &producer.SloClassification{Class: "class", App: "app", Domain: "domain"}},
			outputSloEvent: nil,
			ok:             false,
		},
	}

	for _, tc := range testCases {
		sloEvent, ok := tc.rule.evaluateEvent(&tc.inputEvent)
		assert.Equal(t, ok, tc.ok)
		assert.Equal(t, tc.outputSloEvent, sloEvent)
	}
}

func TestPossibleLabels(t *testing.T) {
	rule := evaluationRule{matcher: eventMetadata{}, additionalMetadata: eventMetadata{"label": "value"}, failureCriteria: []criterium{&requestStatusHigherThan{statusThreshold: 500}}}
	expectedMetadata := []string{"label", "failed", "slo_domain", "slo_class", "app", "event_key"}
	result := rule.PossibleMetadataKeys()
	assert.ElementsMatch(t, expectedMetadata, result)
}
