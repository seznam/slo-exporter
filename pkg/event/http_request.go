package event

import (
	"fmt"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"net/url"
	"time"
)

// HttpRequest represents single event as received by an EventsProcessor instance
type HttpRequest struct {
	Time              time.Time
	StatusCode        int
	URL               *url.URL
	Metadata          stringmap.StringMap
	Method            string
	SloResult         string
	SloClassification *SloClassification
	Quantity          float64
}

const (
	eventKeyMetadataKey = "__eventKey"
)

func (e *HttpRequest) EventKey() string {
	return e.Metadata[eventKeyMetadataKey]
}

func (e *HttpRequest) SetEventKey(k string) {
	if e.Metadata == nil {
		e.Metadata = make(stringmap.StringMap)
	}
	e.Metadata[eventKeyMetadataKey] = k
}

// UpdateSLOClassification updates SloClassification field
func (e *HttpRequest) UpdateSLOClassification(classification *SloClassification) {
	e.SloClassification = classification
}

// IsClassified check if all SloClassification fields are set
func (e *HttpRequest) IsClassified() bool {
	if e.SloClassification != nil &&
		e.SloClassification.Domain != "" &&
		e.SloClassification.App != "" &&
		e.SloClassification.Class != "" {

		return true
	}
	return false
}

func (e HttpRequest) GetSloMetadata() stringmap.StringMap {
	if e.SloClassification == nil {
		return nil
	}
	metadata := e.SloClassification.GetMetadata()
	return metadata
}

func (e HttpRequest) GetSloClassification() *SloClassification {
	return e.SloClassification
}

func (e HttpRequest) GetTimeOccurred() time.Time {
	return e.Time
}

func (e HttpRequest) String() string {
	return fmt.Sprintf("key: %s, metadata: %s, classification: %s", e.EventKey(), e.Metadata, e.GetSloMetadata())
}
