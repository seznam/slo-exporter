package prometheus_exporter

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var conf = prometheusExporterConfig{
	MetricName: "slo_events_total",
	LabelNames: labelsNamesConfig{
		Result:    "result",
		SloDomain: "slo_domain",
		SloClass:  "slo_class",
		SloApp:    "app",
		EventKey:  "event_key",
	},
	MaximumUniqueEventKeys: 2,
}

type testNormalizeEventMetadata struct {
	knownLabels    []string
	input          stringmap.StringMap
	expectedOutput stringmap.StringMap
}

func Test_normalizeEventMetadata(t *testing.T) {
	testCases := []testNormalizeEventMetadata{
		testNormalizeEventMetadata{
			knownLabels:    []string{"b", "c"},
			input:          stringmap.StringMap{"b": "b", "c": "c"},
			expectedOutput: stringmap.StringMap{"b": "b", "c": "c", conf.LabelNames.EventKey: "", conf.LabelNames.Result: "", conf.LabelNames.SloDomain: "", conf.LabelNames.SloClass: "", conf.LabelNames.SloApp: ""},
		},
		testNormalizeEventMetadata{
			knownLabels:    []string{"b", "c"},
			input:          stringmap.StringMap{"a": "a", "b": "b", "c": "c"},
			expectedOutput: stringmap.StringMap{"b": "b", "c": "c", conf.LabelNames.EventKey: "", conf.LabelNames.Result: "", conf.LabelNames.SloDomain: "", conf.LabelNames.SloClass: "", conf.LabelNames.SloApp: ""},
		},
		testNormalizeEventMetadata{
			knownLabels:    []string{"b", "c"},
			input:          stringmap.StringMap{"c": "c", "d": "d"},
			expectedOutput: stringmap.StringMap{"b": "", "c": "c", conf.LabelNames.EventKey: "", conf.LabelNames.Result: "", conf.LabelNames.SloDomain: "", conf.LabelNames.SloClass: "", conf.LabelNames.SloApp: ""},
		},
		testNormalizeEventMetadata{
			knownLabels:    []string{"b", "c"},
			input:          stringmap.StringMap{"d": "d", "e": "e"},
			expectedOutput: stringmap.StringMap{"b": "", "c": "", conf.LabelNames.EventKey: "", conf.LabelNames.Result: "", conf.LabelNames.SloDomain: "", conf.LabelNames.SloClass: "", conf.LabelNames.SloApp: ""},
		},
	}
	for _, testCase := range testCases {
		p := New(prometheus.NewRegistry(), testCase.knownLabels, event.PossibleResults, conf)
		assert.Equal(t, testCase.expectedOutput, p.normalizeEventMetadata(testCase.input))
	}
}

var (
	metricMetadata = fmt.Sprintf(`
		# HELP %s Total number of SLO events exported with it's result and metadata.
		# TYPE %s counter
	`, conf.MetricName, conf.MetricName)
	labels = []string{"a", "b"}
)

type testProcessEvent struct {
	ev              *event.Slo
	expectedMetrics string
}

func Test_PrometheusSloEventExporter_processEvent(t *testing.T) {
	testCases := []testProcessEvent{
		{
			ev: &event.Slo{
				Occurred: time.Time{},
				Metadata: stringmap.StringMap{"a": "a1", "b": "b1"},
				Key: "foo",
				Domain: "domain",
				Result:   event.Fail,
			},
			expectedMetrics: metricMetadata + fmt.Sprintf(`
				%[1]s{ a = "a1" , %[2]s = "", b = "b1", %[3]s = "foo", %[4]s ="fail", %[5]s = "", %[6]s = "domain"} 1
				%[1]s{ a = "a1" , %[2]s = "", b = "b1", %[3]s = "foo", %[4]s ="success", %[5]s = "", %[6]s = "domain"} 0
				`, conf.MetricName, conf.LabelNames.SloApp, conf.LabelNames.EventKey, conf.LabelNames.Result, conf.LabelNames.SloClass, conf.LabelNames.SloDomain),
		},
		{
			ev: &event.Slo{
				Occurred: time.Time{},
				Metadata: stringmap.StringMap{"a": "a1", "b": "b1"},
				Key: "foo",
				Domain: "domain",
				Result:   event.Success,
			},
			expectedMetrics: metricMetadata + fmt.Sprintf(`
				%[1]s{ a = "a1" , %[2]s = "", b = "b1", %[3]s = "foo", %[4]s ="success", %[5]s = "", %[6]s = "domain"} 1
				%[1]s{ a = "a1" , %[2]s = "", b = "b1", %[3]s = "foo", %[4]s ="fail", %[5]s = "", %[6]s = "domain"} 0
				`, conf.MetricName, conf.LabelNames.SloApp, conf.LabelNames.EventKey, conf.LabelNames.Result, conf.LabelNames.SloClass, conf.LabelNames.SloDomain),
		},
	}

	for _, test := range testCases {
		exporter := New(prometheus.NewPedanticRegistry(), labels, event.PossibleResults, conf)
		exporter.processEvent(test.ev)
		if err := testutil.CollectAndCompare(exporter.eventsCount, strings.NewReader(test.expectedMetrics), conf.MetricName); err != nil {
			t.Errorf("unexpected collecting result:\n%s", err)
		}
	}
}

func Test_PrometheusSloEventExporter_isValidResult(t *testing.T) {
	exporter := New(prometheus.NewPedanticRegistry(), []string{}, event.PossibleResults, conf)
	testCases := map[event.Result]bool{
		event.PossibleResults[0]:          true,
		event.Result("nonexistingresult"): false,
	}
	for eventResult, valid := range testCases {
		assert.Equal(t, valid, exporter.isValidResult(eventResult))
	}
}

func Test_PrometheusSloEventExporter_checkEventKeyCardinality(t *testing.T) {
	exporter := New(prometheus.NewPedanticRegistry(), []string{}, event.PossibleResults, conf)
	for i := 0; i < 5; i++ {
		if exporter.isCardinalityExceeded(string(i)) && i+1 <= conf.MaximumUniqueEventKeys {
			t.Errorf("Event key '%d' masked while it the total count '%d' is under given limit '%d'", i, len(exporter.eventKeyCache), conf.MaximumUniqueEventKeys)
		}
		if !exporter.isCardinalityExceeded(string(i)) && i+1 > conf.MaximumUniqueEventKeys {
			t.Errorf("Event key '%d' should have been masked as the total count '%d' is above given limit '%d'", i, len(exporter.eventKeyCache), conf.MaximumUniqueEventKeys)
		}

	}
}
