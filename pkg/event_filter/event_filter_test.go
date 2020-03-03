package event_filter

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"testing"
)

type ShouldDropTestCase struct {
	eventFilter *RequestEventFilter
	event       *event.HttpRequest
	dropped     bool
	reason      string
}

func TestEventFilter_statusMatch(t *testing.T) {
	config := eventFilterConfig{
		FilteredHttpStatusCodeMatchers: []string{"301", "40[04]"},
	}
	testCases := []struct {
		statusCode  int
		shouldMatch bool
	}{
		{statusCode: 200, shouldMatch: false},
		{statusCode: 301, shouldMatch: true},
		{statusCode: 400, shouldMatch: true},
		{statusCode: 404, shouldMatch: true},
		{statusCode: 418, shouldMatch: false},
		{statusCode: 500, shouldMatch: false},
	}
	eventFilter, err := NewFromConfig(config)
	if err != nil {
		t.Error(err)
	}
	for _, tc := range testCases {
		assert.Equal(t, tc.shouldMatch, eventFilter.statusMatch(tc.statusCode))
	}
}

func TestEventFilter_headersMatch(t *testing.T) {
	config := eventFilterConfig{
		FilteredHttpHeaderMatchers: stringmap.StringMap{"(?i)User-Agent": "(?i)Firefox"},
	}
	testCases := []struct {
		headers     stringmap.StringMap
		shouldMatch bool
	}{
		{headers: stringmap.StringMap{"foo": "bar"}, shouldMatch: false},
		{headers: stringmap.StringMap{"useragent": "firefox"}, shouldMatch: false},
		{headers: stringmap.StringMap{"user-agent": "firefox"}, shouldMatch: true},
		{headers: stringmap.StringMap{"User-Agent": "Firefox"}, shouldMatch: true},
		{headers: stringmap.StringMap{"User-Agent foo": "Firefox bar"}, shouldMatch: true},
	}
	eventFilter, err := NewFromConfig(config)
	if err != nil {
		t.Error(err)
	}
	for _, tc := range testCases {
		assert.Equal(t, tc.shouldMatch, eventFilter.headersMatch(tc.headers))
	}
}

func TestEventFilter_shouldDrop(t *testing.T) {
	config := eventFilterConfig{
		FilteredHttpStatusCodeMatchers: []string{"301", "404"},
		FilteredHttpHeaderMatchers:     stringmap.StringMap{"(?i)name": "(?i)value"},
	}
	eventFilter, err := NewFromConfig(config)
	if err != nil {
		t.Error(err)
	}
	testCases := []ShouldDropTestCase{
		// no match
		{
			eventFilter,
			&event.HttpRequest{
				StatusCode: 200,
			},
			false,
			"",
		},
		// status code match
		{
			eventFilter,
			&event.HttpRequest{
				StatusCode: 301,
			},
			true,
			"status:301",
		},
		// no match
		{
			eventFilter,
			&event.HttpRequest{
				StatusCode: 200,
				Headers:    stringmap.StringMap{"name1": "somevalue"},
			},
			true,
			"",
		},
		// just header name match
		{
			eventFilter,
			&event.HttpRequest{
				StatusCode: 200,
				Headers:    stringmap.StringMap{"name": "somevalue"},
			},
			true,
			"",
		},
		// header match
		{
			eventFilter,
			&event.HttpRequest{
				StatusCode: 200,
				Headers:    stringmap.StringMap{"name": "value"},
			},
			true,
			"header:name",
		},
		// header match, name normalization (->lower case)
		{
			eventFilter,
			&event.HttpRequest{
				StatusCode: 200,
				Headers:    stringmap.StringMap{"Name": "Value"},
			},
			true,
			"header:name",
		},
		// both status code and header match
		{
			eventFilter,
			&event.HttpRequest{
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
