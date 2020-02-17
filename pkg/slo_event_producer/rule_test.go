//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"testing"
	"time"
)



type ruleTestCase struct {
	rule           evaluationRule
	inputEvent     producer.RequestEvent
	outputSloEvent *event.Slo
	ok             bool
}

func TestEvaluateEvent(t *testing.T) {
	testCases := []ruleTestCase{
		{
			rule:           evaluationRule{additionalMetadata: stringmap.StringMap{}, failureCriteria: []criterium{&requestStatusHigherThan{statusThreshold: 500}}},
			inputEvent:     producer.RequestEvent{Duration: time.Second, StatusCode: 200, SloClassification: &producer.SloClassification{Class: "class", App: "app", Domain: "domain"}},
			outputSloEvent: &event.Slo{Domain:"domain", Class:"class", App:"app", Key: "", Metadata: stringmap.StringMap{}, Result: event.Success},
			ok:             true,
		},
		{
			rule:           evaluationRule{additionalMetadata: stringmap.StringMap{}, failureCriteria: []criterium{&requestStatusHigherThan{statusThreshold: 500}}},
			inputEvent:     producer.RequestEvent{Duration: time.Second, StatusCode: 200, SloClassification: nil},
			outputSloEvent: nil,
			ok:             false,
		},
		{
			rule:           evaluationRule{sloMatcher: producer.SloClassification{Domain:"foo"}, additionalMetadata: stringmap.StringMap{}, failureCriteria: []criterium{&requestStatusHigherThan{statusThreshold: 500}}},
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
	rule := evaluationRule{additionalMetadata: stringmap.StringMap{"label": "value"}, failureCriteria: []criterium{&requestStatusHigherThan{statusThreshold: 500}}}
	expectedMetadata := []string{"label"}
	result := rule.PossibleMetadataKeys()
	assert.ElementsMatch(t, expectedMetadata, result)
}
