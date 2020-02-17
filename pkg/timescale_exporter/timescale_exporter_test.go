package timescale_exporter

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"testing"
	"time"
)

type eventToRender struct {
	l   string
	v   float64
	t   time.Time
	res string
}

func TestTimescaleExporter_encodePrometheusMetric(t *testing.T) {
	te := TimescaleExporter{
		config: Config{
			metricName: "slo_events_total",
		},
	}
	cases := []eventToRender{
		{l: `label="value"`, v: 1, t: getTime(0), res: te.metricName + `{label="value"} 1 0`},
		{l: `label="value", another="value"`, v: 258.45, t: getTime(5000), res: te.metricName + `{label="value", another="value"} 258.45 5000000`},
	}
	for _, c := range cases {
		assert.Equal(t, te.encodePrometheusMetric(c.l, c.v, c.t), c.res)
	}
}

func TestTimescaleExporter_renderSqlInsert(t *testing.T) {
	te := TimescaleExporter{
		config: Config{
			metricName: "slo_events_total",
		},
	}
	cases := []eventToRender{
		{l: `label="value"`, v: 1, t: getTime(0), res: "INSERT INTO " + timescaleMetricsTable + " VALUES ('" + te.metricName + `{label="value"} 1 0');`},
	}
	for _, c := range cases {
		assert.Equal(t, te.renderSqlInsert(c.l, c.v, c.t), c.res)
	}
}

func TestTimescaleExporter_shouldBeMetricPushed(t *testing.T) {
	cases := []struct {
		metric         *timescaleMetric
		evaluationTime time.Time
		shouldBePushed bool
	}{
		{metric: &timescaleMetric{lastPushTime: getTime(0), lastEventTime: getTime(0)}, evaluationTime: getTime(60), shouldBePushed: true},
		{metric: &timescaleMetric{lastPushTime: getTime(0), lastEventTime: getTime(1)}, evaluationTime: getTime(10), shouldBePushed: true},
		{metric: &timescaleMetric{lastPushTime: getTime(0), lastEventTime: getTime(0)}, evaluationTime: getTime(10), shouldBePushed: false},
	}
	te := TimescaleExporter{
		config: Config{
			MaximumPushInterval: 30 * time.Second,
		},
	}
	for _, c := range cases {
		assert.Equal(t, c.shouldBePushed, te.shouldBeMetricPushed(c.evaluationTime, c.metric))
	}
}

func getTime(seconds int64) time.Time {
	return time.Unix(seconds, 0)
}

type sqlWriterMock struct {
	queries []string
}

func (m *sqlWriterMock) Write(sql string, _ ...interface{}) {
	m.queries = append(m.queries, sql)
}

func (m *sqlWriterMock) WriteQueueSize() int {
	return len(m.queries)
}

func (m *sqlWriterMock) RetryQueueSize() int {
	return 0
}

func (m *sqlWriterMock) Close(ctx context.Context) error {
	return nil
}

var testStatistics = map[string]*timescaleMetric{
	// Should be pushed because the last push is past the MaximumPushInterval.
	"label=1": &timescaleMetric{value: 0, lastPushTime: getTime(0), lastEventTime: getTime(0)},
	// Should not be pushed since it was pushed more recently than MaximumPushInterval and last event was the same time so was not updated.
	"label=2": &timescaleMetric{value: 0, lastPushTime: getTime(35), lastEventTime: getTime(35)},
	// Should be pushed because the last event happened after last push so the value changed.
	"label=3": &timescaleMetric{value: 0, lastPushTime: getTime(35), lastEventTime: getTime(40)},
}

func TestTimescaleExporter_pushMetricsWithTimestamp(t *testing.T) {
	te := TimescaleExporter{
		statistics: testStatistics,
		config: Config{
			MaximumPushInterval: 30 * time.Second,
		},
	}

	te.sqlWriter = &sqlWriterMock{}
	te.pushMetricsWithTimestamp(getTime(60))
	assert.Equal(t, 2, te.sqlWriter.WriteQueueSize())
}

func TestTimescaleExporter_pushAllWithOffset(t *testing.T) {
	te := TimescaleExporter{
		statistics: testStatistics,
		config: Config{
			MaximumPushInterval: 30 * time.Second,
		},
	}

	te.sqlWriter = &sqlWriterMock{}
	te.pushAllMetricsWithOffset(time.Hour)
	assert.Equal(t, 3, te.sqlWriter.WriteQueueSize())
}

func TestTimescaleExporter_processEvent(t *testing.T) {
	testedResult := event.Success
	testEvent := event.Slo{
		Occurred: time.Time{},
		Metadata: stringmap.StringMap{},
		Result:   testedResult,
	}
	te := TimescaleExporter{
		instanceName: "test",
		labelNames:   labelsNamesConfig{Instance: "instance", Result: "result"},
		statistics:   map[string]*timescaleMetric{},
	}
	expectedStatistics := map[string]*timescaleMetric{}
	for _, v := range event.PossibleResults {
		newMetric := &timescaleMetric{
			value:         0,
			lastPushTime:  time.Time{},
			lastEventTime: time.Time{},
		}
		if v == testedResult {
			newMetric.value = 1
		}
		expectedStatistics[fmt.Sprintf("%s=%q,%s=%q", te.labelNames.Instance, "test", te.labelNames.Result, v)] = newMetric
	}

	te.processEvent(&testEvent)
	assert.Equal(t, len(expectedStatistics), len(te.statistics))
	for k, v := range expectedStatistics {
		value, ok := te.statistics[k]
		if !ok {
			t.Errorf("did not find key %v in map %v", k, te.statistics)
			continue
		}
		assert.Equal(t, v.value, value.value)
	}
}
