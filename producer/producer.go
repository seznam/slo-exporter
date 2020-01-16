package producer

import (
	"net"
	"net/url"
	"time"
)

type RequestEvent struct {
	Time          time.Time
	IP            net.IP
	StatusCode    int
	Duration      time.Duration
	URL           *url.URL
	NormalizedURL *url.URL
	Headers       map[string]string
	Method        string
	SloClass      string
	SloApp        string
	SloEndpoint   string
	SloDomain     string
}
