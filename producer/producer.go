package producer

import (
	"net"
	"time"
)

type RequestEvent struct {
	Time           time.Time
	IP             net.IP
	Path           string
	NormalizedPath string
	Referer        string
	StatusCode     int
	Method         string
	Params         map[string]string
	Duration       float32 // request duration in seconds
	Grid           string
	SloClass       string
	SloApp         string
	SloEndpoint    string
}
