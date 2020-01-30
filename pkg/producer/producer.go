package producer

import (
	"net"
	"net/url"
	"time"
)

type SloClassification struct {
	Domain string
	App    string
	Class  string
}

func (sc *SloClassification) GetMap() map[string]string {
	return map[string]string{
		"slo_domain": sc.Domain,
		"slo_class": sc.Class,
		"app": sc.App,
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
	Headers           map[string]string
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

func (e RequestEvent) GetSloMetadata() *map[string]string {
	if e.SloClassification == nil {
		return nil
	}
	metadata := e.SloClassification.GetMap()
	metadata["endpoint"] = e.EventKey
	return &metadata
}
