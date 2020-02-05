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
	labels = []string{"a", "b"}
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
				SloMetadata:  map[string]string{"a": "a1", "b": "b1"},
				Result:       slo_event_producer.SloEventResultFail,
			},
			expectedMetrics: metricMetadata + fmt.Sprintf(`
				%s{ a = "a1" , b = "b1", result="fail"} 1
				%s{ a = "a1" , b = "b1", result="success"} 0
				`, metricName, metricName),
		},
		{
			event: &slo_event_producer.SloEvent{
				TimeOccurred: time.Time{},
				SloMetadata:  map[string]string{"a": "a1", "b": "b1"},
				Result:       slo_event_producer.SloEventResultSuccess,
			},
			expectedMetrics: metricMetadata + fmt.Sprintf(`
				%s{ a = "a1" , b = "b1", result="success"} 1
				%s{ a = "a1" , b = "b1", result="fail"} 0
				`, metricName, metricName),
		},
	}

	for _, test := range testCases {
		exporter := New(labels, slo_event_producer.EventResults)
		exporter.processEvent(test.event)
		if err := testutil.CollectAndCompare(exporter.eventsCount, strings.NewReader(test.expectedMetrics), metricName); err != nil {
			t.Errorf("unexpected collecting result:\n%s", err)
		}
	}
}
