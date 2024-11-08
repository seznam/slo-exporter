package dynamic_classifier

import (
	"io"

	"github.com/seznam/slo-exporter/pkg/event"
)

type matcherType string

type matcher interface {
	getType() matcherType
	set(key string, classification *event.SloClassification) error
	get(key string) (*event.SloClassification, error)
	dumpCSV(w io.Writer) error
}
