//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/stringmap"
	"github.com/stretchr/testify/assert"
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
		{event: event.NewRaw("", 1, stringmap.StringMap{"number": "20"}, nil), operator: &numberIsHigherThan{numberComparisonOperator{name: "numberIsHigherThan", key: "number", value: 10}}, result: true, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"number": "1"}, nil), operator: &numberIsHigherThan{numberComparisonOperator{name: "numberIsHigherThan", key: "number", value: 10}}, result: false, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"number": "12.5"}, nil), operator: &numberIsHigherThan{numberComparisonOperator{name: "numberIsHigherThan", key: "number", value: 10}}, result: true, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"number": "foo"}, nil), operator: &numberIsHigherThan{numberComparisonOperator{name: "numberIsHigherThan", key: "number", value: 10}}, result: false, err: true},
		{event: event.NewRaw("", 1, nil, nil), operator: &numberIsHigherThan{numberComparisonOperator{value: 10}}, result: false, err: false},

		// numberIsEqualOrHigherThan
		{event: event.NewRaw("", 1, stringmap.StringMap{"number": "10"}, nil), operator: &numberIsEqualOrHigherThan{numberComparisonOperator{name: "numberIsEqualOrHigherThan", key: "number", value: 10}}, result: true, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"number": "1"}, nil), operator: &numberIsEqualOrHigherThan{numberComparisonOperator{name: "numberIsEqualOrHigherThan", key: "number", value: 10}}, result: false, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"number": "12.5"}, nil), operator: &numberIsEqualOrHigherThan{numberComparisonOperator{name: "numberIsEqualOrHigherThan", key: "number", value: 10}}, result: true, err: false},

		// numberIsEqualTo
		{event: event.NewRaw("", 1, stringmap.StringMap{"number": "10"}, nil), operator: &numberIsEqualTo{numberComparisonOperator{name: "numberIsEqualTo", key: "number", value: 10}}, result: true, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"number": "1"}, nil), operator: &numberIsEqualTo{numberComparisonOperator{name: "numberIsEqualTo", key: "number", value: 10}}, result: false, err: false},

		// numberIsNotEqualTo
		{event: event.NewRaw("", 1, stringmap.StringMap{"number": "10"}, nil), operator: &numberIsNotEqualTo{numberComparisonOperator{name: "numberIsNotEqualTo", key: "number", value: 10}}, result: false, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"number": "11"}, nil), operator: &numberIsNotEqualTo{numberComparisonOperator{name: "numberIsNotEqualTo", key: "number", value: 10}}, result: true, err: false},

		// numberIsEqualOrLessThan
		{event: event.NewRaw("", 1, stringmap.StringMap{"number": "10"}, nil), operator: &numberIsEqualOrLessThan{numberComparisonOperator{name: "numberIsEqualOrLessThan", key: "number", value: 10}}, result: true, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"number": "1"}, nil), operator: &numberIsEqualOrLessThan{numberComparisonOperator{name: "numberIsEqualOrLessThan", key: "number", value: 10}}, result: true, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"number": "20"}, nil), operator: &numberIsEqualOrLessThan{numberComparisonOperator{name: "numberIsEqualOrLessThan", key: "number", value: 10}}, result: false, err: false},

		// durationIsHigherThan
		{event: event.NewRaw("", 1, stringmap.StringMap{"duration": "20s"}, nil), operator: &durationIsHigherThan{key: "duration", thresholdDuration: 10 * time.Second}, result: true, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"duration": "5ms"}, nil), operator: &durationIsHigherThan{key: "duration", thresholdDuration: 10 * time.Second}, result: false, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"duration": "foo"}, nil), operator: &durationIsHigherThan{key: "duration", thresholdDuration: 10 * time.Second}, result: false, err: true},
		{event: event.NewRaw("", 1, nil, nil), operator: &numberIsHigherThan{numberComparisonOperator{value: 10}}, result: false, err: false},

		// equalTo
		{event: event.NewRaw("", 1, stringmap.StringMap{"foo": "foobar"}, nil), operator: &isEqualTo{key: "foo", value: "foobar"}, result: true, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"foo": "foobar"}, nil), operator: &isEqualTo{key: "foo", value: "xxx"}, result: false, err: false},

		// notEqualTo
		{event: event.NewRaw("", 1, stringmap.StringMap{"foo": "foobar"}, nil), operator: &isNotEqualTo{key: "foo", value: "foobar"}, result: false, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"foo": "foobar"}, nil), operator: &isNotEqualTo{key: "foo", value: "xxx"}, result: true, err: false},

		// isMatchingRegexp
		{event: event.NewRaw("", 1, stringmap.StringMap{"foo": "foobar"}, nil), operator: &isMatchingRegexp{key: "foo", regexp: regexp.MustCompile("bar")}, result: true, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"foo": "foobar"}, nil), operator: &isMatchingRegexp{key: "foo", regexp: regexp.MustCompile("xxx")}, result: false, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"foo": ""}, nil), operator: &isMatchingRegexp{key: "foo", regexp: regexp.MustCompile(".*")}, result: true, err: false},

		// isNotMatchingRegexp
		{event: event.NewRaw("", 1, stringmap.StringMap{"foo": "foobar"}, nil), operator: &isNotMatchingRegexp{key: "foo", regexp: regexp.MustCompile("bar")}, result: false, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"foo": "foobar"}, nil), operator: &isNotMatchingRegexp{key: "foo", regexp: regexp.MustCompile("xxx")}, result: true, err: false},
		{event: event.NewRaw("", 1, stringmap.StringMap{"foo": ""}, nil), operator: &isNotMatchingRegexp{key: "foo", regexp: regexp.MustCompile(".*")}, result: false, err: false},
	}
	for _, c := range testCases {
		res, err := c.operator.Evaluate(c.event)
		if c.err {
			assert.Error(t, err, "expected error for event: %+v", c.event)
		} else {
			assert.NoError(t, err, "did not expect error for event: %+v", c.event)
		}
		assert.Equal(t, c.result, res, "unexpected result for event metadata: %s", c.event.Metadata)
	}
}
