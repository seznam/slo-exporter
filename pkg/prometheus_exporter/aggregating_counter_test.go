package prometheus_exporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"strings"
	"testing"
)

func Test_aggregatingCounter(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	aggVec := newAggregatedCounterVectorSet("slo_events_total", metricHelp, []string{
		"slo_domain",
		"slo_class",
		"slo_app",
		"event_key",
	}, logrus.New())
	err := aggVec.register(reg)
	assert.NoError(t, err)

	expectedMetrics := `
# HELP slo_domain:slo_events_total Total number of SLO events exported with it's result and metadata.
# TYPE slo_domain:slo_events_total counter
slo_domain:slo_events_total{result="success",slo_domain="domain"} 1
# HELP slo_domain_slo_class:slo_events_total Total number of SLO events exported with it's result and metadata.
# TYPE slo_domain_slo_class:slo_events_total counter
slo_domain_slo_class:slo_events_total{result="success",slo_class="critical",slo_domain="domain"} 1
# HELP slo_domain_slo_class_slo_app:slo_events_total Total number of SLO events exported with it's result and metadata.
# TYPE slo_domain_slo_class_slo_app:slo_events_total counter
slo_domain_slo_class_slo_app:slo_events_total{result="success",slo_app="app",slo_class="critical",slo_domain="domain"} 1
# HELP slo_domain_slo_class_slo_app_event_key:slo_events_total Total number of SLO events exported with it's result and metadata.
# TYPE slo_domain_slo_class_slo_app_event_key:slo_events_total counter
slo_domain_slo_class_slo_app_event_key:slo_events_total{event_key="key",result="success",slo_app="app",slo_class="critical",slo_domain="domain"} 1
`

	aggVec.inc(stringmap.StringMap{
		"result":     "success",
		"slo_class":  "critical",
		"slo_app":    "app",
		"slo_domain": "domain",
		"event_key":  "key",
	})

	if err := testutil.GatherAndCompare(reg, strings.NewReader(expectedMetrics)); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}
