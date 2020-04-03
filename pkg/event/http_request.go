package event

import (
	"fmt"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"net"
	"net/url"
	"time"
)

// HttpRequest represents single event as received by an EventsProcessor instance
type HttpRequest struct {
	Time              time.Time
	IP                net.IP
	StatusCode        int
	Duration          time.Duration
	URL               *url.URL
	EventKey          string
	Headers           stringmap.StringMap // name:value, header name is in lower-case
	Metadata          stringmap.StringMap
	Method            string
	SloResult         string
	SloClassification *SloClassification
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

func (e HttpRequest) GetSloMetadata() *stringmap.StringMap {
	if e.SloClassification == nil {
		return nil
	}
	metadata := e.SloClassification.GetMetadata()
	return &metadata
}

func (e HttpRequest) GetSloClassification() *SloClassification {
	return e.SloClassification
}

func (e HttpRequest) GetTimeOccurred() time.Time {
	return e.Time
}

func (e HttpRequest) String() string {
	key := e.Method + ":" + e.URL.Path
	if e.EventKey != "" {
		key = e.EventKey
	}
	return fmt.Sprintf("key: %q, status: %d, duration: %s, classification: %s", key, e.StatusCode, e.Duration, e.SloClassification)
}
