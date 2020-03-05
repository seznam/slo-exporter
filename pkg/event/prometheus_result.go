package event

import (
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"time"
)

type PrometheusQueryResult struct {
	Value     float64
	Timestamp time.Time
	Labels    stringmap.StringMap
}
