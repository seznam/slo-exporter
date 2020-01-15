package producer

import (
	"net"
	"time"
)

type RequestEvent struct {
	Time           time.Time
	IP             net.IP
	StatusCode     int
	Duration       time.Duration
	Path           string
	NormalizedPath string
	UserAgent      string
	Method         string
	Params         map[string]string
	SloClass       string
	SloApp         string
	SloEndpoint    string
	SloDomain      string
}
