//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/stretchr/testify/assert"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/stringmap"
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
		{opts: operatorOptions{Operator: "numberIsHigherThan", Value: "10"}, expectedOperator: &numberIsHigherThan{numberComparisonOperator{name: "numberIsHigherThan", value: 10}}, expectedErr: false},
		{opts: operatorOptions{Operator: "numberIsHigherThan", Value: "1.5"}, expectedOperator: &numberIsHigherThan{numberComparisonOperator{name: "numberIsHigherThan", value: 1.5}}, expectedErr: false},
		{opts: operatorOptions{Operator: "numberIsHigherThan", Value: "foo"}, expectedOperator: nil, expectedErr: true},

		{opts: operatorOptions{Operator: "durationIsHigherThan", Value: "1s"}, expectedOperator: &durationIsHigherThan{thresholdDuration: time.Second}, expectedErr: false},
		{opts: operatorOptions{Operator: "durationIsHigherThan", Value: "foo"}, expectedOperator: nil, expectedErr: true},

		{opts: operatorOptions{Operator: "isMatchingRegexp", Value: ".*"}, expectedOperator: &isMatchingRegexp{regexp: regexp.MustCompile(".*")}, expectedErr: false},
		{opts: operatorOptions{Operator: "isMatchingRegexp", Value: "***"}, expectedOperator: nil, expectedErr: true},

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
	event    event.Raw
	operator operator
	result   bool
	err      bool
}

func TestCriteria(t *testing.T) {
	testCases := []testEvent{
		// numberIsHigherThan
		{event: event.Raw{Metadata: stringmap.StringMap{"number": "20"}}, operator: &numberIsHigherThan{numberComparisonOperator{name: "numberIsHigherThan", key: "number", value: 10}}, result: true, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"number": "1"}}, operator: &numberIsHigherThan{numberComparisonOperator{name: "numberIsHigherThan", key: "number", value: 10}}, result: false, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"number": "12.5"}}, operator: &numberIsHigherThan{numberComparisonOperator{name: "numberIsHigherThan", key: "number", value: 10}}, result: true, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"number": "foo"}}, operator: &numberIsHigherThan{numberComparisonOperator{name: "numberIsHigherThan", key: "number", value: 10}}, result: false, err: true},
		{event: event.Raw{}, operator: &numberIsHigherThan{numberComparisonOperator{value: 10}}, result: false, err: false},

		// numberIsEqualOrHigherThan
		{event: event.Raw{Metadata: stringmap.StringMap{"number": "10"}}, operator: &numberIsEqualOrHigherThan{numberComparisonOperator{name: "numberIsEqualOrHigherThan", key: "number", value: 10}}, result: true, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"number": "1"}}, operator: &numberIsEqualOrHigherThan{numberComparisonOperator{name: "numberIsEqualOrHigherThan", key: "number", value: 10}}, result: false, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"number": "12.5"}}, operator: &numberIsEqualOrHigherThan{numberComparisonOperator{name: "numberIsEqualOrHigherThan", key: "number", value: 10}}, result: true, err: false},

		// numberIsEqualTo
		{event: event.Raw{Metadata: stringmap.StringMap{"number": "10"}}, operator: &numberIsEqualTo{numberComparisonOperator{name: "numberIsEqualTo", key: "number", value: 10}}, result: true, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"number": "1"}}, operator: &numberIsEqualTo{numberComparisonOperator{name: "numberIsEqualTo", key: "number", value: 10}}, result: false, err: false},

		// numberIsNotEqualTo
		{event: event.Raw{Metadata: stringmap.StringMap{"number": "10"}}, operator: &numberIsNotEqualTo{numberComparisonOperator{name: "numberIsNotEqualTo", key: "number", value: 10}}, result: false, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"number": "11"}}, operator: &numberIsNotEqualTo{numberComparisonOperator{name: "numberIsNotEqualTo", key: "number", value: 10}}, result: true, err: false},

		// numberIsEqualOrLessThan
		{event: event.Raw{Metadata: stringmap.StringMap{"number": "10"}}, operator: &numberIsEqualOrLessThan{numberComparisonOperator{name: "numberIsEqualOrLessThan", key: "number", value: 10}}, result: true, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"number": "1"}}, operator: &numberIsEqualOrLessThan{numberComparisonOperator{name: "numberIsEqualOrLessThan", key: "number", value: 10}}, result: true, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"number": "20"}}, operator: &numberIsEqualOrLessThan{numberComparisonOperator{name: "numberIsEqualOrLessThan", key: "number", value: 10}}, result: false, err: false},

		// durationIsHigherThan
		{event: event.Raw{Metadata: stringmap.StringMap{"duration": "20s"}}, operator: &durationIsHigherThan{key: "duration", thresholdDuration: 10 * time.Second}, result: true, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"duration": "5ms"}}, operator: &durationIsHigherThan{key: "duration", thresholdDuration: 10 * time.Second}, result: false, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"duration": "foo"}}, operator: &durationIsHigherThan{key: "duration", thresholdDuration: 10 * time.Second}, result: false, err: true},
		{event: event.Raw{}, operator: &numberIsHigherThan{numberComparisonOperator{value: 10}}, result: false, err: false},

		// equalTo
		{event: event.Raw{Metadata: stringmap.StringMap{"foo": "foobar"}}, operator: &isEqualTo{key: "foo", value: "foobar"}, result: true, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"foo": "foobar"}}, operator: &isEqualTo{key: "foo", value: "xxx"}, result: false, err: false},

		// notEqualTo
		{event: event.Raw{Metadata: stringmap.StringMap{"foo": "foobar"}}, operator: &isNotEqualTo{key: "foo", value: "foobar"}, result: false, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"foo": "foobar"}}, operator: &isNotEqualTo{key: "foo", value: "xxx"}, result: true, err: false},

		// isMatchingRegexp
		{event: event.Raw{Metadata: stringmap.StringMap{"foo": "foobar"}}, operator: &isMatchingRegexp{key: "foo", regexp: regexp.MustCompile("bar")}, result: true, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"foo": "foobar"}}, operator: &isMatchingRegexp{key: "foo", regexp: regexp.MustCompile("xxx")}, result: false, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"foo": ""}}, operator: &isMatchingRegexp{key: "foo", regexp: regexp.MustCompile(".*")}, result: true, err: false},

		// isNotMatchingRegexp
		{event: event.Raw{Metadata: stringmap.StringMap{"foo": "foobar"}}, operator: &isNotMatchingRegexp{key: "foo", regexp: regexp.MustCompile("bar")}, result: false, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"foo": "foobar"}}, operator: &isNotMatchingRegexp{key: "foo", regexp: regexp.MustCompile("xxx")}, result: true, err: false},
		{event: event.Raw{Metadata: stringmap.StringMap{"foo": ""}}, operator: &isNotMatchingRegexp{key: "foo", regexp: regexp.MustCompile(".*")}, result: false, err: false},
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
