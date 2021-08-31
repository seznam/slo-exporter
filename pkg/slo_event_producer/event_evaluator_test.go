//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/go-test/deep"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/stringmap"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
)

type sloEventTestCase struct {
	inputEvent        event.Raw
	expectedSloEvents []event.Slo
	rulesConfig       rulesConfig
}

func TestSloEventProducer(t *testing.T) {
	testCases := []sloEventTestCase{
		{
			inputEvent: event.NewRaw("", 1, stringmap.StringMap{"statusCode": "502"}, &event.SloClassification{Class: "class", App: "app", Domain: "domain"}),
			rulesConfig: rulesConfig{Rules: []ruleOptions{
				{
					SloMatcher:                       sloMatcher{DomainRegexp: "domain"},
					MetadataMatcherConditionsOptions: []operatorOptions{},
					FailureConditionsOptions: []operatorOptions{
						{
							Operator: "numberIsHigherThan", Key: "statusCode", Value: "500",
						},
					},
					AdditionalMetadata: stringmap.StringMap{"slo_type": "availability"},
				},
			},
			},
			expectedSloEvents: []event.Slo{
				event.NewSlo("", 1, event.Fail, event.SloClassification{Domain: "domain", App: "app", Class: "class"}, stringmap.StringMap{"slo_type": "availability"}),
			},
		},
		{
			inputEvent: event.NewRaw("", 1, stringmap.StringMap{"statusCode": "200"}, &event.SloClassification{Class: "class", App: "app", Domain: "domain"}),
			rulesConfig: rulesConfig{Rules: []ruleOptions{
				{
					SloMatcher:                       sloMatcher{DomainRegexp: "domain"},
					MetadataMatcherConditionsOptions: []operatorOptions{},
					FailureConditionsOptions: []operatorOptions{
						{
							Operator: "numberIsHigherThan", Key: "statusCode", Value: "500",
						},
					},
					AdditionalMetadata: stringmap.StringMap{"slo_type": "availability"},
				},
			},
			},
			expectedSloEvents: []event.Slo{
				event.NewSlo("", 1, event.Success, event.SloClassification{Domain: "domain", App: "app", Class: "class"}, stringmap.StringMap{"slo_type": "availability"}),
			},
		},
	}

	for _, tc := range testCases {
		out := make(chan event.Slo, 100)
		testedEvaluator, err := NewEventEvaluatorFromConfig(&tc.rulesConfig, logrus.New())
		if err != nil {
			t.Errorf("error when loading config: %v error: %v", tc.rulesConfig, err)
		}
		testedEvaluator.Evaluate(tc.inputEvent, out)
		close(out)
		var results []event.Slo
		for newEvent := range out {
			results = append(results, newEvent)
		}
		if diff := deep.Equal(tc.expectedSloEvents, results); diff != nil {
			t.Errorf("events are different %+v, \nexpected: %+v\n result: %+v\n input event metadata: %+v", diff, tc.expectedSloEvents, results, tc.inputEvent.Metadata())
		}
	}
}

type getMetricsFromRuleOptionsTestCase struct {
	Name           string
	RulesConfig    rulesConfig
	ExpectedMetric []metric
}

func TestConfig_getMetricsFromRuleOptions(t *testing.T) {
	testCases := []getMetricsFromRuleOptionsTestCase{
		{
			Name: "Configured rules are exposed as Prometheus metric",
			RulesConfig: rulesConfig{[]ruleOptions{
				{
					MetadataMatcherConditionsOptions: []operatorOptions{
						{
							Operator: "isEqualTo",
							Key:      "name",
							Value:    "ad.banner",
						},
					},
					SloMatcher: sloMatcher{},
					FailureConditionsOptions: []operatorOptions{
						{
							Operator: "numberIsHigherThan",
							Key:      "prometheusQueryResult",
							Value:    "6300",
						},
						{
							Operator: "numberIsEqualOrLessThan",
							Key:      "prometheusQueryResult",
							Value:    "7000",
						},
					},
					AdditionalMetadata: stringmap.StringMap{"foo": "bar"},
				},
			},
			},
			ExpectedMetric: []metric{
				{
					Labels: stringmap.StringMap{"foo": "bar", "name": "ad.banner", "operator": "numberIsHigherThan"},
					Value:  6300,
				},
				{
					Labels: stringmap.StringMap{"foo": "bar", "name": "ad.banner", "operator": "numberIsEqualOrLessThan"},
					Value:  7000,
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(
			testCase.Name,
			func(t *testing.T) {
				var (
					metrics []metric
					err     error
				)
				evaluator, err := NewEventEvaluatorFromConfig(&testCase.RulesConfig, logrus.New())
				if err != nil {
					t.Error(err)
				}
				metrics, _, err = evaluator.ruleOptionsToMetrics()
				if err != nil {
					t.Error(err)
				}
				assert.Equal(t, testCase.ExpectedMetric, metrics)
			},
		)
	}
}
