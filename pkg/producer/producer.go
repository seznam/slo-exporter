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
