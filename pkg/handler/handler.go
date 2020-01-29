package handler

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	log *logrus.Entry

	errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "slo_exporter",
			Subsystem: "request_handler",
			Name:      "errors_total",
			Help:      "Total number of processed events by result.",
		},
		[]string{"type"},
	)
)

func init() {
	log = logrus.WithFields(logrus.Fields{"component": "request_handler"})
	prometheus.MustRegister(errorsTotal)

}
