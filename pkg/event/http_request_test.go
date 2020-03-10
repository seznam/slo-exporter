package event

import (
	"net/url"
	"testing"
	"time"
)

func Test_GetEventKey(t *testing.T) {
	e := &HttpRequest{
		Time:              time.Time{},
		IP:                nil,
		StatusCode:        0,
		Duration:          0,
		URL:               &url.URL{},
		EventKey:          "eventKey",
		Headers:           map[string]string{},
		Method:            "",
		SloEndpoint:       "",
		SloClassification: &SloClassification{},
	}
	if e.GetEventKey() != e.EventKey {
		t.Errorf("Method GetEventKey on event '%+v' should have returned '%s'", e, e.EventKey)
	}

	e.SloEndpoint = "sloEndpoint"
	if e.GetEventKey() != e.SloEndpoint {
		t.Errorf("Method GetEventKey on event '%+v' should have returned '%s'", e, e.SloEndpoint)
	}
}
