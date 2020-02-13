package prometheus_exporter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/slo_event_producer"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	eventKeyLabel = "event_key"
)

type testNormalizeEventMetadata struct {
	knownMetadata  []string
	input          map[string]string
	expectedOutput map[string]string
}

func Test_normalizeEventMetadata(t *testing.T) {
	testCases := []testNormalizeEventMetadata{
		testNormalizeEventMetadata{
			knownMetadata:  []string{"b", "c"},
			input:          map[string]string{"b": "b", "c": "c"},
			expectedOutput: map[string]string{"b": "b", "c": "c"},
		},
		testNormalizeEventMetadata{
			knownMetadata:  []string{"b", "c"},
			input:          map[string]string{"a": "a", "b": "b", "c": "c"},
			expectedOutput: map[string]string{"b": "b", "c": "c"},
		},
		testNormalizeEventMetadata{
			knownMetadata:  []string{"b", "c"},
			input:          map[string]string{"c": "c", "d": "d"},
			expectedOutput: map[string]string{"b": "", "c": "c"},
		},
		testNormalizeEventMetadata{
			knownMetadata:  []string{"b", "c"},
			input:          map[string]string{"d": "d", "e": "e"},
			expectedOutput: map[string]string{"b": "", "c": ""},
		},
	}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.expectedOutput, normalizeEventMetadata(testCase.knownMetadata, testCase.input))
	}
}

var (
	metricMetadata = fmt.Sprintf(`
		# HELP %s Total number of SLO events exported with it's result and metadata.
		# TYPE %s counter
	`, metricName, metricName)
	labels = []string{"a", "b", eventKeyLabel}
)

type testProcessEvent struct {
	event           *slo_event_producer.SloEvent
	expectedMetrics string
}

func Test_PrometheusSloEventExporter_processEvent(t *testing.T) {
	testCases := []testProcessEvent{
		{
			event: &slo_event_producer.SloEvent{
				TimeOccurred: time.Time{},
				SloMetadata:  map[string]string{"a": "a1", "b": "b1", eventKeyLabel: ""},
				Result:       slo_event_producer.SloEventResultFail,
			},
			expectedMetrics: metricMetadata + fmt.Sprintf(`
				%s{ a = "a1" , b = "b1", %s = "", result="fail"} 1
				%s{ a = "a1" , b = "b1", %s = "", result="success"} 0
				`, metricName, eventKeyLabel, metricName, eventKeyLabel),
		},
		{
			event: &slo_event_producer.SloEvent{
				TimeOccurred: time.Time{},
				SloMetadata:  map[string]string{"a": "a1", "b": "b1", eventKeyLabel: ""},
				Result:       slo_event_producer.SloEventResultSuccess,
			},
			expectedMetrics: metricMetadata + fmt.Sprintf(`
				%s{ a = "a1" , b = "b1", %s = "", result="success"} 1
				%s{ a = "a1" , b = "b1", %s = "", result="fail"} 0
				`, metricName, eventKeyLabel, metricName, eventKeyLabel),
		},
	}

	for _, test := range testCases {
		exporter := New(labels, slo_event_producer.EventResults, eventKeyLabel, 0)
		exporter.processEvent(test.event)
		if err := testutil.CollectAndCompare(exporter.eventsCount, strings.NewReader(test.expectedMetrics), metricName); err != nil {
			t.Errorf("unexpected collecting result:\n%s", err)
		}
	}
}

func Test_PrometheusSloEventExporter_isValidResult(t *testing.T) {
	exporter := New([]string{}, slo_event_producer.EventResults, eventKeyLabel, 0)
	testCases := map[slo_event_producer.SloEventResult]bool{
		slo_event_producer.EventResults[0]:                     true,
		slo_event_producer.SloEventResult("nonexistingresult"): false,
	}
	for eventResult, valid := range testCases {
		assert.Equal(t, valid, exporter.isValidResult(eventResult))
	}
}

func Test_PrometheusSloEventExporter_checkEventKeyCardinality(t *testing.T) {
	eventKeyLimit := 2
	exporter := New([]string{}, slo_event_producer.EventResults, eventKeyLabel, eventKeyLimit)
	for i := 0; i < 5; i++ {
		if exporter.isCardinalityExceeded(string(i)) && i+1 <= eventKeyLimit {
			t.Errorf("Event key '%d' masked while it the total count '%d' is under given limit '%d'", i, len(exporter.eventKeyCache), eventKeyLimit)
		}
		if !exporter.isCardinalityExceeded(string(i)) && i+1 > eventKeyLimit {
			t.Errorf("Event key '%d' should have been masked as the total count '%d' is above given limit '%d'", i, len(exporter.eventKeyCache), eventKeyLimit)
		}

	}
}
