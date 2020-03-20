package event_filter

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"testing"
)

type ShouldDropTestCase struct {
	eventFilter *EventFilter
	event       *event.HttpRequest
	dropped     bool
	reason      string
}

func TestEventFilter_headersMatch(t *testing.T) {
	config := eventFilterConfig{
		MetadataFilter: stringmap.StringMap{"(?i)User-Agent": "(?i)Firefox"},
	}
	testCases := []struct {
		metadata    stringmap.StringMap
		shouldMatch bool
	}{
		{metadata: stringmap.StringMap{"foo": "bar"}, shouldMatch: false},
		{metadata: stringmap.StringMap{"useragent": "firefox"}, shouldMatch: false},
		{metadata: stringmap.StringMap{"user-agent": "firefox"}, shouldMatch: true},
		{metadata: stringmap.StringMap{"User-Agent": "Firefox"}, shouldMatch: true},
		{metadata: stringmap.StringMap{"User-Agent foo": "Firefox bar"}, shouldMatch: true},
	}
	eventFilter, err := NewFromConfig(config)
	if err != nil {
		t.Error(err)
	}
	for _, tc := range testCases {
		matches, _ := eventFilter.metadataMatch(tc.metadata)
		assert.Equal(t, tc.shouldMatch, matches)
	}
}

func TestEventFilter_shouldDrop(t *testing.T) {
	config := eventFilterConfig{
		MetadataFilter: stringmap.StringMap{
			"(?i)name":       "(?i)value",
			"(?i)statusCode": "301|404",
		},
	}
	eventFilter, err := NewFromConfig(config)
	if err != nil {
		t.Error(err)
	}
	testCases := []ShouldDropTestCase{
		// no match
		{
			eventFilter,
			&event.HttpRequest{Metadata: stringmap.StringMap{"statusCode": "200"}},
			false,
			"",
		},
		// status code match
		{
			eventFilter,
			&event.HttpRequest{Metadata: stringmap.StringMap{"statusCode": "301"}},
			true,
			"status:301",
		},
		// no match
		{
			eventFilter,
			&event.HttpRequest{Metadata: stringmap.StringMap{"name1": "somevalue"}},
			true,
			"",
		},
		// just header name match
		{
			eventFilter,
			&event.HttpRequest{Metadata: stringmap.StringMap{"statusCode": "200", "name": "somevalue"}},
			true,
			"",
		},
		// header match
		{
			eventFilter,
			&event.HttpRequest{Metadata: stringmap.StringMap{"statusCode": "200", "name": "value"}},
			true,
			"header:name",
		},
		// header match, name normalization (->lower case)
		{
			eventFilter,
			&event.HttpRequest{Metadata: stringmap.StringMap{"statusCode": "200", "Name": "Value"}},
			true,
			"header:name",
		},
		// both status code and header match
		{
			eventFilter,
			&event.HttpRequest{Metadata: stringmap.StringMap{"statusCode": "404", "name": "value"}},
			true,
			"status:404",
		},
	}

	for _, test := range testCases {
		dropped := test.eventFilter.matches(test.event)
		assert.Equal(t, test.dropped, dropped, "event %+v failed with eventFilter %+v", test.event, test.eventFilter)
	}
}
