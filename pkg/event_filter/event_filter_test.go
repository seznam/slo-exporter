package event_filter

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"testing"
)

type ShouldDropTestCase struct {
	eventFilter *RequestEventFilter
	event       *producer.RequestEvent
	dropped     bool
	reason      string
}

func TestEventFilter_statusMatch(t *testing.T) {
	testCases := []struct {
		statusCode  int
		shouldMatch bool
	}{
		{statusCode: 200, shouldMatch: false},
		{statusCode: 301, shouldMatch: true},
		{statusCode: 404, shouldMatch: true},
		{statusCode: 500, shouldMatch: false},
	}
	eventFilter := New([]int{301, 404}, map[string]string{})
	for _, tc := range testCases {
		assert.Equal(t, tc.shouldMatch, eventFilter.statusMatch(tc.statusCode))
	}
}

func TestEventFilter_headersMatch(t *testing.T) {
	testCases := []struct {
		headers     map[string]string
		shouldMatch bool
	}{
		{headers: map[string]string{"foo": "bar"}, shouldMatch: false},
		{headers: map[string]string{"useragent": "firefox"}, shouldMatch: false},
		{headers: map[string]string{"user-agent": "firefox"}, shouldMatch: true},
		{headers: map[string]string{"User-Agent": "Firefox"}, shouldMatch: true},
	}
	eventFilter := New([]int{}, map[string]string{"User-Agent": "Firefox"})
	for _, tc := range testCases {
		assert.Equal(t, tc.shouldMatch, eventFilter.headersMatch(tc.headers))
	}
}

func TestEventFilter_headersToLowercase(t *testing.T) {
	testCases := []struct {
		in  map[string]string
		out map[string]string
	}{
		{
			in:  map[string]string{"foo": "bar"},
			out: map[string]string{"foo": "bar"},
		},
		{
			in:  map[string]string{"Foo": "Bar"},
			out: map[string]string{"foo": "bar"},
		},
	}
	for _, tc := range testCases {
		assert.Equal(t, tc.out, headersToLowercase(tc.in))
	}
}

func TestEventFilter_shouldDrop(t *testing.T) {
	eventFilter := New([]int{301, 404}, map[string]string{"name": "value"})
	testCases := []ShouldDropTestCase{
		// no match
		{
			eventFilter,
			&producer.RequestEvent{
				StatusCode: 200,
			},
			false,
			"",
		},
		// status code match
		{
			eventFilter,
			&producer.RequestEvent{
				StatusCode: 301,
			},
			true,
			"status:301",
		},
		// no match
		{
			eventFilter,
			&producer.RequestEvent{
				StatusCode: 200,
				Headers:    map[string]string{"name1": "somevalue"},
			},
			false,
			"",
		},
		// just header name match
		{
			eventFilter,
			&producer.RequestEvent{
				StatusCode: 200,
				Headers:    map[string]string{"name": "somevalue"},
			},
			false,
			"",
		},
		// header match
		{
			eventFilter,
			&producer.RequestEvent{
				StatusCode: 200,
				Headers:    map[string]string{"name": "value"},
			},
			true,
			"header:name",
		},
		// header match, name normalization (->lower case)
		{
			New([]int{}, map[string]string{"NAME": "value"}),
			&producer.RequestEvent{
				StatusCode: 200,
				Headers:    map[string]string{"name": "value"},
			},
			true,
			"header:name",
		},
		// both status code and header match
		{
			eventFilter,
			&producer.RequestEvent{
				StatusCode: 404,
				Headers:    map[string]string{"name": "value"},
			},
			true,
			"status:404",
		},
	}

	for _, test := range testCases {
		dropped := test.eventFilter.matches(test.event)
		assert.Equal(t, test.dropped, dropped, "event %v failed with eventFilter %v", test.event, test.eventFilter)
	}
}
