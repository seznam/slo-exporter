package prometheus_ingester

import (
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"time"
)

type queryOptions struct {
	Query            string
	Interval         time.Duration
	DropLabels       []string
	AdditionalLabels stringmap.StringMap
}
