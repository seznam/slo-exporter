package prometheus_ingester

import (
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_queryResult_applyStaleness(t *testing.T) {
	ts := model.Time(0)
	fingerprint := model.Fingerprint(0)
	tests := []struct {
		name            string
		input           queryResult
		staleness       time.Duration
		ts              time.Time
		expectedMetrics int
	}{
		{
			name:      "keep recent samples",
			ts:        ts.Time().Add(time.Minute),
			staleness: defaultStaleness,
			input: queryResult{
				timestamp: ts.Time(),
				metrics: map[model.Fingerprint]model.SamplePair{
					fingerprint: {
						Timestamp: ts,
						Value:     0,
					},
				},
			},
			expectedMetrics: 1,
		},
		{
			name:      "drop outdated samples",
			ts:        ts.Time().Add(time.Minute + defaultStaleness),
			staleness: defaultStaleness,
			input: queryResult{
				timestamp: ts.Time(),
				metrics: map[model.Fingerprint]model.SamplePair{
					fingerprint: {
						Timestamp: ts,
						Value:     0,
					},
				},
			},
			expectedMetrics: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.dropStaleResults(tt.staleness, tt.ts)
			assert.Equalf(t, tt.expectedMetrics, len(tt.input.metrics), "unexpected number of metrics in result: %s", tt.input.metrics)
		})
	}
}
