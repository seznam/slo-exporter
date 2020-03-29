//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"regexp"
	"testing"
	"time"
)

type testOperatorOpts struct {
	opts             operatorOptions
	expectedOperator operator
	expectedErr      bool
}

func TestOperator_newOperator(t *testing.T) {
	testCases := []testOperatorOpts{
		{opts: operatorOptions{Operator: "numberHigherThan", Value: "10"}, expectedOperator: &numberHigherThan{threshold: 10}, expectedErr: false},
		{opts: operatorOptions{Operator: "numberHigherThan", Value: "1.5"}, expectedOperator: &numberHigherThan{threshold: 1.5}, expectedErr: false},
		{opts: operatorOptions{Operator: "numberHigherThan", Value: "foo"}, expectedOperator: nil, expectedErr: true},

		{opts: operatorOptions{Operator: "durationHigherThan", Value: "1s"}, expectedOperator: &durationHigherThan{thresholdDuration: time.Second}, expectedErr: false},
		{opts: operatorOptions{Operator: "durationHigherThan", Value: "foo"}, expectedOperator: nil, expectedErr: true},

		{opts: operatorOptions{Operator: "matchesRegexp", Value: ".*"}, expectedOperator: &matchesRegexp{regexp: regexp.MustCompile(".*")}, expectedErr: false},
		{opts: operatorOptions{Operator: "matchesRegexp", Value: "***"}, expectedOperator: nil, expectedErr: true},

		{opts: operatorOptions{Operator: "xxx", Value: "xxx"}, expectedOperator: nil, expectedErr: true},
	}
	for _, c := range testCases {
		newOperator, err := newOperator(c.opts)
		if c.expectedErr {
			assert.Error(t, err, "expected error for options: %+v", c.opts)
			continue
		}
		assert.Equal(t, newOperator, c.expectedOperator, "unexpected result for options: %+v", c.opts)
	}
}

type testEvent struct {
	event    event.HttpRequest
	operator operator
	result   bool
	err      bool
}

func TestCriteria(t *testing.T) {
	testCases := []testEvent{
		// numberHigherThan
		{event: event.HttpRequest{Metadata: stringmap.StringMap{"number": "20"}}, operator: &numberHigherThan{key: "number", threshold: 10}, result: true, err: false},
		{event: event.HttpRequest{Metadata: stringmap.StringMap{"number": "1"}}, operator: &numberHigherThan{key: "number", threshold: 10}, result: false, err: false},
		{event: event.HttpRequest{Metadata: stringmap.StringMap{"number": "12.5"}}, operator: &numberHigherThan{key: "number", threshold: 10}, result: true, err: false},
		{event: event.HttpRequest{Metadata: stringmap.StringMap{"number": "foo"}}, operator: &numberHigherThan{key: "number", threshold: 10}, result: false, err: true},
		{event: event.HttpRequest{}, operator: &numberHigherThan{threshold: 10}, result: false, err: false},

		// durationHigherThan
		{event: event.HttpRequest{Metadata: stringmap.StringMap{"duration": "20s"}}, operator: &durationHigherThan{key: "duration", thresholdDuration: 10 * time.Second}, result: true, err: false},
		{event: event.HttpRequest{Metadata: stringmap.StringMap{"duration": "5ms"}}, operator: &durationHigherThan{key: "duration", thresholdDuration: 10 * time.Second}, result: false, err: false},
		{event: event.HttpRequest{Metadata: stringmap.StringMap{"duration": "foo"}}, operator: &durationHigherThan{key: "duration", thresholdDuration: 10 * time.Second}, result: false, err: true},
		{event: event.HttpRequest{}, operator: &numberHigherThan{threshold: 10}, result: false, err: false},

		// matchesRegexp
		{event: event.HttpRequest{Metadata: stringmap.StringMap{"foo": "foobar"}}, operator: &matchesRegexp{key: "foo", regexp: regexp.MustCompile("bar")}, result: true, err: false},
		{event: event.HttpRequest{Metadata: stringmap.StringMap{"foo": "foobar"}}, operator: &matchesRegexp{key: "foo", regexp: regexp.MustCompile("xxx")}, result: false, err: false},
		{event: event.HttpRequest{Metadata: stringmap.StringMap{"foo": ""}}, operator: &matchesRegexp{key: "foo", regexp: regexp.MustCompile(".*")}, result: true, err: false},
	}
	for _, c := range testCases {
		res, err := c.operator.Evaluate(&c.event)
		if c.err {
			assert.Error(t, err, "expected error for event: %+v", c.event)
		} else {
			assert.NoError(t, err, "did not expect error for event: %+v", c.event)
		}
		assert.Equal(t, c.result, res, "unexpected result for event metadata: %s", c.event.Metadata)
	}
}
