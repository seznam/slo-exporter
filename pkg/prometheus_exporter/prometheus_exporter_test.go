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
		SloApp:    "slo_app",
		EventKey:  "event_key",
	},
	MaximumUniqueEventKeys: 2,
}

type testNormalizeEventMetadata struct {
	knownLabels    []string
	input          stringmap.StringMap
	expectedOutput stringmap.StringMap
}

type testProcessEvent struct {
	ev              *event.Slo
	expectedMetrics string
}

func Test_PrometheusSloEventExporter_processEvent(t *testing.T) {
	testedMetricName := aggregatedMetricName(conf.MetricName, conf.LabelNames.SloDomain, conf.LabelNames.SloClass, conf.LabelNames.SloApp, conf.LabelNames.EventKey)
	metricMetadata := fmt.Sprintf(`
		# HELP %[1]s %[2]s
		# TYPE %[1]s counter
	`, testedMetricName, metricHelp)

	testCases := []testProcessEvent{
		{
			ev: &event.Slo{
				Occurred: time.Time{},
				Metadata: stringmap.StringMap{"a": "a1", "b": "b1"},
				Key:      "foo",
				Domain:   "domain",
				Result:   event.Fail,
			},
			expectedMetrics: metricMetadata + fmt.Sprintf(`
				%[1]s{ a = "a1" , b = "b1", %[2]s = "foo", %[3]s ="fail", %[4]s = "", %[5]s = "", %[6]s = "domain"} 1
				%[1]s{ a = "a1" , b = "b1", %[2]s = "foo", %[3]s ="success", %[4]s = "", %[5]s = "", %[6]s = "domain"} 0
				`, testedMetricName, conf.LabelNames.EventKey, conf.LabelNames.Result, conf.LabelNames.SloApp, conf.LabelNames.SloClass, conf.LabelNames.SloDomain),
		},
		{
			ev: &event.Slo{
				Occurred: time.Time{},
				Metadata: stringmap.StringMap{"a": "a1", "b": "b1"},
				Key:      "foo",
				Domain:   "domain",
				Result:   event.Success,
			},
			expectedMetrics: metricMetadata + fmt.Sprintf(`
				%[1]s{ a = "a1" , b = "b1", %[2]s = "foo", %[3]s ="success", %[4]s = "", %[5]s = "", %[6]s = "domain"} 1
				%[1]s{ a = "a1" , b = "b1", %[2]s = "foo", %[3]s ="fail", %[4]s = "", %[5]s = "", %[6]s = "domain"} 0
				`, testedMetricName, conf.LabelNames.EventKey, conf.LabelNames.Result, conf.LabelNames.SloApp, conf.LabelNames.SloClass, conf.LabelNames.SloDomain),
		},
	}

	for _, test := range testCases {
		reg := prometheus.NewPedanticRegistry()
		exporter, err := New(reg, conf)
		if err != nil {
			t.Error(err)
			return
		}
		if err := exporter.processEvent(test.ev); err != nil {
			t.Error(err)
			return
		}
		if err := testutil.GatherAndCompare(reg, strings.NewReader(test.expectedMetrics), testedMetricName); err != nil {
			t.Errorf("unexpected collecting result:\n%s", err)
		}
	}
}

func Test_PrometheusSloEventExporter_isValidResult(t *testing.T) {
	exporter, err := New(prometheus.NewPedanticRegistry(), conf)
	if err != nil {
		t.Error(err)
		return
	}
	testCases := map[event.Result]bool{
		event.PossibleResults[0]:          true,
		event.Result("nonexistingresult"): false,
	}
	for eventResult, valid := range testCases {
		assert.Equal(t, valid, exporter.isValidResult(eventResult))
	}
}

func Test_PrometheusSloEventExporter_checkEventKeyCardinality(t *testing.T) {
	exporter, err := New(prometheus.NewPedanticRegistry(), conf)
	if err != nil {
		t.Error(err)
		return
	}
	for i := 0; i < 5; i++ {
		if exporter.isCardinalityExceeded(string(i)) && i+1 <= conf.MaximumUniqueEventKeys {
			t.Errorf("Event key '%d' masked while it the total count '%d' is under given limit '%d'", i, len(exporter.eventKeyCache), conf.MaximumUniqueEventKeys)
		}
		if !exporter.isCardinalityExceeded(string(i)) && i+1 > conf.MaximumUniqueEventKeys {
			t.Errorf("Event key '%d' should have been masked as the total count '%d' is above given limit '%d'", i, len(exporter.eventKeyCache), conf.MaximumUniqueEventKeys)
		}

	}
}
