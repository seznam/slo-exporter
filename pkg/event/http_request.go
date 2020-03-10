package event

import (
	"fmt"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"net"
	"net/url"
	"time"
)

const (
	UndefinedFRPCStatus = 0
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
	Method            string
	SloEndpoint       string
	SloResult         string
	SloClassification *SloClassification
	FRPCStatus        int
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

// GetEventKey returns event identifier (called RPC name,...). It can be used as a key to group occurrence of given event through time.
func (e HttpRequest) GetEventKey() string {
	if e.SloEndpoint != "" {
		return e.SloEndpoint
	}
	return e.EventKey
}

func (e HttpRequest) String() string {
	key := e.Method + ":" + e.URL.Path
	if e.EventKey != "" {
		key = e.EventKey
	}
	return fmt.Sprintf("key: %q, status: %d, duration: %s", key, e.StatusCode, e.Duration)
}
