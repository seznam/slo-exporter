//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/stringmap"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

type ruleTestCase struct {
	name           string
	rule           evaluationRule
	inputEvent     event.Raw
	outputSloEvent *event.Slo
	ok             bool
}

func TestEvaluateEvent(t *testing.T) {
	testCases := []ruleTestCase{
		{
			name: "no metadata_matcher, failure_condition does not match -> successful event",
			rule: evaluationRule{
				additionalMetadata: stringmap.StringMap{},
				failureConditions:  []operator{&numberIsEqualOrHigherThan{numberComparisonOperator{key: "statusCode", value: 500}}},
				logger:             logrus.New(),
			},
			inputEvent: event.Raw{
				Metadata:          stringmap.StringMap{"statusCode": "200"},
				SloClassification: &event.SloClassification{Class: "class", App: "app", Domain: "domain"},
			},
			outputSloEvent: &event.Slo{Domain: "domain", Class: "class", App: "app", Key: "", Metadata: stringmap.StringMap{}, Result: event.Success},
			ok:             true,
		},
		{
			name: "no metadata_matcher, failure_condition does match -> failed event",
			rule: evaluationRule{
				additionalMetadata: stringmap.StringMap{},
				failureConditions:  []operator{&numberIsEqualOrHigherThan{numberComparisonOperator{key: "statusCode", value: 500}}},
				logger:             logrus.New(),
			},
			inputEvent: event.Raw{
				Metadata:          stringmap.StringMap{"statusCode": "502"},
				SloClassification: &event.SloClassification{Class: "class", App: "app", Domain: "domain"},
			},
			outputSloEvent: &event.Slo{Domain: "domain", Class: "class", App: "app", Key: "", Metadata: stringmap.StringMap{}, Result: event.Fail},
			ok:             true,
		},
		{
			name: "event is unclassified -> error reported",
			rule: evaluationRule{
				additionalMetadata: stringmap.StringMap{},
				failureConditions:  []operator{&isMatchingRegexp{key: "statusCode", regexp: regexp.MustCompile("500")}},
				logger:             logrus.New(),
			},
			inputEvent: event.Raw{
				Metadata:          stringmap.StringMap{"statusCode": "502"},
				SloClassification: nil,
			},
			outputSloEvent: nil,
			ok:             false,
		},
		{
			name: "event does not match the only matcher -> error reported",
			rule: evaluationRule{
				sloMatcher:         sloClassificationMatcher{domainRegexp: regexp.MustCompile("foo")},
				additionalMetadata: stringmap.StringMap{},
				failureConditions:  []operator{&isMatchingRegexp{key: "statusCode", regexp: regexp.MustCompile("500")}},
				logger:             logrus.New(),
			},
			inputEvent: event.Raw{
				Metadata:          stringmap.StringMap{"statusCode": "502"},
				SloClassification: &event.SloClassification{Class: "class", App: "app", Domain: "domain"},
			},
			outputSloEvent: nil,
			ok:             false,
		},
		{
			name: "event does not match any matcher -> error reported",
			rule: evaluationRule{
				sloMatcher:         sloClassificationMatcher{domainRegexp: regexp.MustCompile("domain")},
				metadataMatcher:    []operator{&isMatchingRegexp{key: "key", regexp: regexp.MustCompile("value")}},
				additionalMetadata: stringmap.StringMap{},
				failureConditions:  []operator{&isMatchingRegexp{key: "statusCode", regexp: regexp.MustCompile("500")}},
				logger:             logrus.New(),
			},
			inputEvent: event.Raw{
				Metadata:          stringmap.StringMap{"statusCode": "200"},
				SloClassification: &event.SloClassification{Class: "class", App: "app", Domain: "domain"},
			},
			outputSloEvent: nil,
			ok:             false,
		},
		{
			name: "metadata matcher matches, failure_condition does not match -> succesful event",
			rule: evaluationRule{
				sloMatcher:         sloClassificationMatcher{domainRegexp: regexp.MustCompile("domain")},
				metadataMatcher:    []operator{&isMatchingRegexp{key: "key", regexp: regexp.MustCompile("value")}},
				additionalMetadata: stringmap.StringMap{},
				failureConditions:  []operator{&isMatchingRegexp{key: "statusCode", regexp: regexp.MustCompile("500")}},
				logger:             logrus.New(),
			},
			inputEvent: event.Raw{
				Metadata:          stringmap.StringMap{"statusCode": "200", "key": "value"},
				SloClassification: &event.SloClassification{Class: "class", App: "app", Domain: "domain"},
			},
			outputSloEvent: &event.Slo{Domain: "domain", Class: "class", App: "app", Key: "", Metadata: stringmap.StringMap{}, Result: event.Success},
			ok:             true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sloEvent, ok := tc.rule.processEvent(&tc.inputEvent)
			assert.Equal(t, ok, tc.ok)
			assert.Equal(t, tc.outputSloEvent, sloEvent, "unexpected result evaluating rule: %+v\n  with conditions: %+v\non event:\n  metadata: %+v\n  classification: %+v", tc.rule, tc.rule.failureConditions[0], tc.inputEvent.Metadata, tc.inputEvent.SloClassification)

		})
	}
}
