//revive:disable:var-naming
package dynamic_classifier

//revive:enable:var-naming

import (
	"github.com/prometheus/client_golang/prometheus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"io"
)

type matcherType string

var (
	// TODO matcher cache size, matcher or something related has to implement the prometheus.Collector interface and count the items.
	matcherOperationDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "slo_exporter",
			Subsystem: "dynamic_classifier",
			Name:      "matcher_operation_duration_seconds",
			Help:      "Histogram of duration matcher operations in dynamic classifier.",
			Buckets:   prometheus.ExponentialBuckets(0.0001, 5, 7),
		}, []string{"operation", "matcher_type"})
)

func init() {
	prometheus.MustRegister(matcherOperationDurationSeconds)
}

type matcher interface {
	getType() matcherType
	set(key string, classification *producer.SloClassification) error
	get(key string) (*producer.SloClassification, error)
	dumpCSV(w io.Writer) error
}
