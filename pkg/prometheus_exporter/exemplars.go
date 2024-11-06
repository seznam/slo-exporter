package prometheus_exporter

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Support for exemplars is still considered experimental both, in Prometheus and in the client library.
// The client still does not allow to set the exemplars for const metrics intended to be used for exporters.
// As a workaround part of the functionality had to be copied out from the client and new custom constCounterWithExemplar had to be created.
// This is definitely not ideal but if we want to support this functionality, there is no other way for now.

// FIXME once implemented, use client constMetric exemplars support

// Copied from https://github.com/prometheus/client_golang/blob/0400fc44d42dd0bca7fb16e87ea0313bb2eb8c53/prometheus/value.go#L183 since there is no exposed API for now.
func newExemplar(value float64, ts time.Time, l prometheus.Labels) (*dto.Exemplar, error) {
	e := &dto.Exemplar{}
	e.Value = proto.Float64(value)
	tsProto := timestamppb.New(ts)
	e.Timestamp = tsProto
	labelPairs := make([]*dto.LabelPair, 0, len(l))
	var runes int
	for name, value := range l {
		// Also not exported
		// if !checkLabelName(name) {
		//	return nil, fmt.Errorf("exemplar label name %q is invalid", name)
		//}
		runes += utf8.RuneCountInString(name)
		if !utf8.ValidString(value) {
			return nil, fmt.Errorf("exemplar label value %q is not valid UTF-8", value)
		}
		runes += utf8.RuneCountInString(value)
		labelPairs = append(labelPairs, &dto.LabelPair{
			Name:  proto.String(name),
			Value: proto.String(value),
		})
	}
	if runes > prometheus.ExemplarMaxRunes {
		return nil, fmt.Errorf("exemplar labels have %d runes, exceeding the limit of %d", runes, prometheus.ExemplarMaxRunes)
	}
	e.Label = labelPairs
	return e, nil
}

func NewConstCounterWithExemplar(desc *prometheus.Desc, _ prometheus.ValueType, value float64, labelValues ...string) (ConstCounterWithExemplar, error) {
	return ConstCounterWithExemplar{
		desc:       desc,
		val:        value,
		labelPairs: prometheus.MakeLabelPairs(desc, labelValues),
		exemplar:   nil,
	}, nil
}

type ExemplarAdder interface {
	AddExemplar(exemplar *dto.Exemplar)
}

type ConstCounterWithExemplar struct {
	desc       *prometheus.Desc
	val        float64
	labelPairs []*dto.LabelPair
	exemplar   *dto.Exemplar
}

func (c *ConstCounterWithExemplar) AddExemplar(e *dto.Exemplar) {
	c.exemplar = e
}

func (c ConstCounterWithExemplar) Desc() *prometheus.Desc {
	return c.desc
}

func (c ConstCounterWithExemplar) Write(metric *dto.Metric) error {
	metric.Label = c.labelPairs
	metric.Counter = &dto.Counter{Value: &c.val, Exemplar: c.exemplar}
	return nil
}
