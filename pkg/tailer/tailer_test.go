package tailer

import (
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

type parseRequestLineTestData struct {
	request    string
	method     string
	requestURI string
	proto      string
	err        error
}

var parseRequestLineTestTable = []parseRequestLineTestData{
	{"GET / HTTP/1.1", "GET", "/", "HTTP/1.1", nil},
	{"POST /api/v1/notifications/flash HTTP/2.0", "POST", "/api/v1/notifications/flash", "HTTP/2.0", nil},
	{"GET /", "GET", "/", "", nil},
	{"lv[endof]", "", "", "", &InvalidRequestError{"lv[endof]"}},
}

func TestParseRequestLine(t *testing.T) {
	for _, test := range parseRequestLineTestTable {
		method, requestURI, proto, err := parseRequestLine(test.request)
		assert.IsType(t, test.err, err)
		assert.Equal(t, test.method, method)
		assert.Equal(t, test.requestURI, requestURI)
		assert.Equal(t, test.proto, proto)
	}
}

type parseLineTest struct {
	lineContentMapping map[string]string
	isLineValid        bool
}

var parseLineTestTable = []parseLineTest{
	// ipv4
	{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":          "34.65.133.58",
		"request":     "GET /robots.txt HTTP/1.1",
		"statusCode":  "200",
		"requestTime": "0.123", // in ms, as logged by nginx
	}, true},
	// ipv6
	{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":          "2001:718:801:230::1",
		"request":     "GET /robots.txt HTTP/1.1",
		"statusCode":  "200",
		"requestTime": "0.123", // in ms, as logged by nginx
	}, true},
	// invalid time
	{map[string]string{"time": "32/Nov/2019:25:20:07 +0100",
		"ip":          "2001:718:801:230::1",
		"request":     "GET /robots.txt HTTP/1.1",
		"statusCode":  "200x",
		"requestTime": "0.123", // in ms, as logged by nginx
	}, false},
	// invalid request
	{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":          "2001:718:801:230::1",
		"request":     "invalid-request[eof]",
		"statusCode":  "200x",
		"requestTime": "0.123", // in ms, as logged by nginx
	}, false},
	// request without protocol
	{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":          "2001:718:801:230::1",
		"request":     "GET /robots.txt",
		"statusCode":  "301",
		"requestTime": "0.123", // in ms, as logged by nginx
	}, true},
	// http2.0 proto request
	{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":          "2001:718:801:230::1",
		"request":     "GET /robots.txt HTTP/2.0",
		"statusCode":  "200",
		"requestTime": "0.123", // in ms, as logged by nginx
	}, true},
	// zero status code
	{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":          "2001:718:801:230::1",
		"request":     "GET /robots.txt HTTP/1.1",
		"statusCode":  "0",
		"requestTime": "0.123", // in ms, as logged by nginx
	}, false},
	// invalid status code
	{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":          "2001:718:801:230::1",
		"request":     "GET /robots.txt HTTP/1.1",
		"statusCode":  "xxx",
		"requestTime": "0.123", // in ms, as logged by nginx
	}, false},
}

func TestParseLine(t *testing.T) {
	requestLineFormat := `{ip} - - [{time}] "{request}" {statusCode} 79 "-" uag="Go-http-client/1.1" "-" ua="10.66.112.78:80" rt="{requestTime}" uct="0.000" uht="0.000" urt="0.000" cc="static" occ="-" url="68" ourl="-"`
	for _, test := range parseLineTestTable {
		requestLine := requestLineFormat
		for k, v := range test.lineContentMapping {
			requestLine = strings.Replace(requestLine, fmt.Sprintf("{%s}", k), v, -1)
		}
		parsedEvent, err := parseLine(requestLine)

		var expectedEvent *producer.RequestEvent

		if test.isLineValid {
			// line is considered valid, build the expectedEvent struct in order to compare it to the parsed one
			duration, _ := time.ParseDuration(test.lineContentMapping["requestTime"] + "ms")
			lineTime, _ := time.Parse(timeLayout, test.lineContentMapping["time"])
			statusCode, _ := strconv.Atoi(test.lineContentMapping["statusCode"])
			method, requestURI, _, _ := parseRequestLine(test.lineContentMapping["request"])
			uri, _ := url.Parse(requestURI)

			expectedEvent = &producer.RequestEvent{
				Time:       lineTime,
				IP:         net.ParseIP(test.lineContentMapping["ip"]),
				Duration:   duration,
				URL:        uri,
				StatusCode: statusCode,
				EventKey:   "",
				Headers:    make(map[string]string),
				Method:     method,
			}
			if !reflect.DeepEqual(expectedEvent, parsedEvent) {
				t.Errorf("Unexpected result of parse line: %s\n%v\nExpected:\n%v", requestLine, parsedEvent, expectedEvent)
			}
		} else {
			// line is not valid, just check that err is returned
			if err == nil {
				t.Errorf("Line wrongly considered as valid: %s", requestLine)
			}
		}

	}
}
