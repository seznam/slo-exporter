package event_filter

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"testing"
)

type ShouldDropTestCase struct {
	eventFilter *RequestEventFilter
	event       *producer.RequestEvent
	dropped     bool
	reason      string
}

func TestEventFilter_statusMatch(t *testing.T) {
	config := eventFilterConfig{
		FilteredHttpStatusCodes: []int{301, 404},
	}
	testCases := []struct {
		statusCode  int
		shouldMatch bool
	}{
		{statusCode: 200, shouldMatch: false},
		{statusCode: 301, shouldMatch: true},
		{statusCode: 404, shouldMatch: true},
		{statusCode: 500, shouldMatch: false},
	}
	eventFilter := New(config)
	for _, tc := range testCases {
		assert.Equal(t, tc.shouldMatch, eventFilter.statusMatch(tc.statusCode))
	}
}

func TestEventFilter_headersMatch(t *testing.T) {
	config := eventFilterConfig{
		FilteredHttpHeaders: stringmap.StringMap{"User-Agent": "Firefox"},
	}
	testCases := []struct {
		headers     stringmap.StringMap
		shouldMatch bool
	}{
		{headers: stringmap.StringMap{"foo": "bar"}, shouldMatch: false},
		{headers: stringmap.StringMap{"useragent": "firefox"}, shouldMatch: false},
		{headers: stringmap.StringMap{"user-agent": "firefox"}, shouldMatch: true},
		{headers: stringmap.StringMap{"User-Agent": "Firefox"}, shouldMatch: true},
	}
	eventFilter := New(config)
	for _, tc := range testCases {
		assert.Equal(t, tc.shouldMatch, eventFilter.headersMatch(tc.headers))
	}
}


func TestEventFilter_shouldDrop(t *testing.T) {
	config := eventFilterConfig{
		FilteredHttpStatusCodes: []int{301, 404},
		FilteredHttpHeaders: stringmap.StringMap{"name": "value"},
	}
	eventFilter := New(config)
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
				Headers:    stringmap.StringMap{"name1": "somevalue"},
			},
			false,
			"",
		},
		// just header name match
		{
			eventFilter,
			&producer.RequestEvent{
				StatusCode: 200,
				Headers:    stringmap.StringMap{"name": "somevalue"},
			},
			false,
			"",
		},
		// header match
		{
			eventFilter,
			&producer.RequestEvent{
				StatusCode: 200,
				Headers:    stringmap.StringMap{"name": "value"},
			},
			true,
			"header:name",
		},
		// header match, name normalization (->lower case)
		{
			New(eventFilterConfig{FilteredHttpHeaders: stringmap.StringMap{"NAME": "value"}}, ),
			&producer.RequestEvent{
				StatusCode: 200,
				Headers:    stringmap.StringMap{"name": "value"},
			},
			true,
			"header:name",
		},
		// both status code and header match
		{
			eventFilter,
			&producer.RequestEvent{
				StatusCode: 404,
				Headers:    stringmap.StringMap{"name": "value"},
			},
			true,
			"status:404",
		},
	}

	for _, test := range testCases {
		dropped := test.eventFilter.matches(test.event)
		assert.Equal(t, test.dropped, dropped, "event %+v failed with eventFilter %+v", test.event, test.eventFilter)
	}
}
