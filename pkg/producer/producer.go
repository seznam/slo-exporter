package producer

import (
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"net"
	"net/url"
	"time"
)

type SloClassification struct {
	Domain string
	App    string
	Class  string
}

func (sc *SloClassification) Matches(other SloClassification) bool {
	if sc.Domain != "" && (sc.Domain != other.Domain) {
		return false
	}
	if sc.Class != "" && (sc.Class != other.Class) {
		return false
	}
	if sc.App != "" && (sc.App != other.App) {
		return false
	}
	return true
}


func (sc *SloClassification) GetMetadata() stringmap.StringMap {
	return stringmap.StringMap{
		"slo_domain": sc.Domain,
		"slo_class":  sc.Class,
		"app":        sc.App,
	}
}

// RequestEvent represents single event as received by an EventsProcessor instance
type RequestEvent struct {
	Time              time.Time
	IP                net.IP
	StatusCode        int
	Duration          time.Duration
	URL               *url.URL
	EventKey          string
	Headers           stringmap.StringMap // name:value, header name is in lower-case
	Method            string
	SloEndpoint       string
	SloClassification *SloClassification
}

// UpdateSLOClassification updates SloClassification field
func (e *RequestEvent) UpdateSLOClassification(classification *SloClassification) {
	e.SloClassification = classification
}

// IsClassified check if all SloClassification fields are set
func (e *RequestEvent) IsClassified() bool {
	if e.SloClassification != nil &&
		e.SloClassification.Domain != "" &&
		e.SloClassification.App != "" &&
		e.SloClassification.Class != "" {

		return true
	}
	return false
}

func (e RequestEvent) GetSloMetadata() *stringmap.StringMap {
	if e.SloClassification == nil {
		return nil
	}
	metadata := e.SloClassification.GetMetadata()
	return &metadata
}

func (e RequestEvent) GetSloClassification() *SloClassification {
	return e.SloClassification
}

func (e RequestEvent) GetTimeOccurred() time.Time {
	return e.Time
}

func (e RequestEvent) GetEventKey() string {
	return e.EventKey
}
