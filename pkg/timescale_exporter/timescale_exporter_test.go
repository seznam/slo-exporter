package timescale_exporter

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/slo_event_producer"
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
	cases := []eventToRender{
		{l: `{label="value"}`, v: 1, t: getTime(0), res: sloEventsMetricName + `{label="value"} 1 0`},
		{l: `{label="value", another="value"}`, v: 258.45, t: getTime(5000), res: sloEventsMetricName + `{label="value", another="value"} 258.45 5000000`},
	}
	for _, c := range cases {
		assert.Equal(t, encodePrometheusMetric(c.l, c.v, c.t), c.res)
	}
}

func TestTimescaleExporter_renderSqlInsert(t *testing.T) {
	cases := []eventToRender{
		{l: `{label="value"}`, v: 1, t: getTime(0), res: "INSERT INTO " + timescaleMetricsTable + " VALUES ('" + sloEventsMetricName + `{label="value"} 1 0');`},
	}
	for _, c := range cases {
		assert.Equal(t, renderSqlInsert(c.l, c.v, c.t), c.res)
	}
}

func TestTimescaleExporter_metadataToString(t *testing.T) {
	cases := []struct {
		l   map[string]string
		res string
	}{
		{l: map[string]string{"b": "1", "a": "2"}, res: `{a="2",b="1"}`},
		{l: map[string]string{"a": "1", "b": "2"}, res: `{a="1",b="2"}`},
		{l: map[string]string{}, res: `{}`},
	}
	for _, c := range cases {
		assert.Equal(t, metadataToString(c.l), c.res)
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
	"{label=1}": &timescaleMetric{value: 0, lastPushTime: getTime(0), lastEventTime: getTime(0)},
	// Should not be pushed since it was pushed more recently than MaximumPushInterval and last event was the same time so was not updated.
	"{label=2}": &timescaleMetric{value: 0, lastPushTime: getTime(35), lastEventTime: getTime(35)},
	// Should be pushed because the last event happened after last push so the value changed.
	"{label=3}": &timescaleMetric{value: 0, lastPushTime: getTime(35), lastEventTime: getTime(40)},
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
	testedResult := slo_event_producer.SloEventResultSuccess
	testEvent := slo_event_producer.SloEvent{
		TimeOccurred: time.Time{},
		SloMetadata:  map[string]string{},
		Result:       testedResult,
	}
	expectedStatistics := map[string]*timescaleMetric{}
	for _, v := range slo_event_producer.EventResults {
		newMetric := &timescaleMetric{
			value:         0,
			lastPushTime:  time.Time{},
			lastEventTime: time.Time{},
		}
		if v == testedResult {
			newMetric.value = 1
		}
		expectedStatistics[fmt.Sprintf("{%s=%q,%s=%q}", instanceLabel, "test",sloResultLabel, v)] = newMetric
	}
	te := TimescaleExporter{
		instanceName: "test",
		statistics: map[string]*timescaleMetric{},
	}

	te.processEvent(&testEvent)
	assert.Equal(t, len(expectedStatistics), len(te.statistics))
	for k, v := range expectedStatistics {
		value, ok := te.statistics[k]
		if !ok {
			t.Errorf("did not find key %v", k)
			continue
		}
		assert.Equal(t, v.value, value.value)
	}
}
