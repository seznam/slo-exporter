package tailer

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

var (
	requestLineFormat = `{ip} - - [{time}] "{request}" {statusCode} 79 "-" uag="Go-http-client/1.1" "-" ua="10.66.112.78:80" rt="{requestTime}" uct="0.000" uht="0.000" urt="0.000" cc="static" occ="-" url="68" ourl="-"`
	// provided to getRequestLine, this returns a cosidered-valid line
	requestLineFormatMapValid = map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":          "34.65.133.58",
		"request":     "GET /robots.txt HTTP/1.1",
		"statusCode":  "200",
		"requestTime": "0.123", // in s, as logged by nginx
	}
)

// return request line formatted using the provided formatMap
func getRequestLine(formatMap map[string]string) (requestLine string) {
	requestLine = requestLineFormat
	for k, v := range formatMap {
		requestLine = strings.Replace(requestLine, fmt.Sprintf("{%s}", k), v, -1)
	}
	return requestLine
}

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
		"requestTime": "0.123", // in s, as logged by nginx
	}, true},
	// ipv6
	{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":          "2001:718:801:230::1",
		"request":     "GET /robots.txt HTTP/1.1",
		"statusCode":  "200",
		"requestTime": "0.123", // in s, as logged by nginx
	}, true},
	// invalid time
	{map[string]string{"time": "32/Nov/2019:25:20:07 +0100",
		"ip":          "2001:718:801:230::1",
		"request":     "GET /robots.txt HTTP/1.1",
		"statusCode":  "200x",
		"requestTime": "0.123", // in s, as logged by nginx
	}, false},
	// invalid request
	{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":          "2001:718:801:230::1",
		"request":     "invalid-request[eof]",
		"statusCode":  "200x",
		"requestTime": "0.123", // in s, as logged by nginx
	}, false},
	// request without protocol
	{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":          "2001:718:801:230::1",
		"request":     "GET /robots.txt",
		"statusCode":  "301",
		"requestTime": "0.123", // in s, as logged by nginx
	}, true},
	// http2.0 proto request
	{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":          "2001:718:801:230::1",
		"request":     "GET /robots.txt HTTP/2.0",
		"statusCode":  "200",
		"requestTime": "0.123", // in s, as logged by nginx
	}, true},
	// zero status code
	{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":          "2001:718:801:230::1",
		"request":     "GET /robots.txt HTTP/1.1",
		"statusCode":  "0",
		"requestTime": "0.123", // in s, as logged by nginx
	}, true},
	// invalid status code
	{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":          "2001:718:801:230::1",
		"request":     "GET /robots.txt HTTP/1.1",
		"statusCode":  "xxx",
		"requestTime": "0.123", // in s, as logged by nginx
	}, false},
}

func TestParseLine(t *testing.T) {
	for _, test := range parseLineTestTable {
		requestLine := getRequestLine(test.lineContentMapping)

		parsedEvent, err := parseLine(requestLine)

		var expectedEvent *producer.RequestEvent

		if test.isLineValid {
			// line is considered valid, build the expectedEvent struct in order to compare it to the parsed one
			duration, _ := time.ParseDuration(test.lineContentMapping["requestTime"] + "s")
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

type offsetPersistenceTest struct {
	// all values refers to number of events which should be written to a log file at a given phase of test
	pre    int // *before* the tailing starts
	during int // while the tailer is running
	post   int // after the tailer temporarily stops
	reopen int // after the tailer starts again
}

// reads in chan and on close returns count to out chan
func countEvents(in chan *producer.RequestEvent, out chan int) {
	count := 0
	for range in {
		count++
	}
	out <- count
}

func offsetPersistenceTestRun(t offsetPersistenceTest) error {
	// temp file for logs
	f, err := ioutil.TempFile("", "")
	if err != nil {
		return fmt.Errorf("Error while creating temp file: %w", err)
	}
	fname := f.Name()
	positionsFname := f.Name() + ".pos"
	defer os.Remove(positionsFname)
	defer os.Remove(fname)
	defer f.Close()

	eventCount := make(chan int)
	persistPositionInterval, _ := time.ParseDuration("10s")

	for i := 0; i < t.pre; i++ {
		f.WriteString(getRequestLine(requestLineFormatMapValid) + "\n")
	}

	config := tailerConfig{
		TailedFile:                  fname,
		Follow:                      true,
		Reopen:                      true,
		PositionFile:                positionsFname,
		PositionPersistenceInterval: persistPositionInterval,
	}
	tailer, err := New(config)
	if err != nil {
		return err
	}

	eventsChan := make(chan *producer.RequestEvent)
	errChan := make(chan error, 10)
	ctx, cancelFunc := context.WithCancel(context.Background())
	tailer.Run(ctx, eventsChan, errChan)
	go countEvents(eventsChan, eventCount)

	for i := 0; i < t.during; i++ {
		f.WriteString(getRequestLine(requestLineFormatMapValid) + "\n")
	}
	time.Sleep(100 * time.Millisecond)

	cancelFunc()
	eventsCount := <-eventCount

	if eventsCount != t.pre+t.during {
		return fmt.Errorf("Number of processed events during first open of a log file does not match: got '%d', expected '%d'", eventsCount, t.pre+t.during)
	}

	for i := 0; i < t.post; i++ {
		f.WriteString(getRequestLine(requestLineFormatMapValid) + "\n")
	}

	eventsChan = make(chan *producer.RequestEvent)
	errChan = make(chan error, 10)
	ctx, cancelFunc = context.WithCancel(context.Background())

	tailer, err = New(config)
	if err != nil {
		return err
	}
	tailer.Run(ctx, eventsChan, errChan)
	go countEvents(eventsChan, eventCount)

	for i := 0; i < t.reopen; i++ {
		f.WriteString(getRequestLine(requestLineFormatMapValid) + "\n")
	}

	time.Sleep(100 * time.Millisecond)

	cancelFunc()
	eventsCount = <-eventCount

	if eventsCount != t.post+t.reopen {
		return fmt.Errorf("Number of processed events after reopening a log file does not match: got '%d', expected '%d'", eventsCount, t.post+t.reopen)
	}

	return nil
}

var testOffsetPersistenceTable = []offsetPersistenceTest{
	// events in log file present before the first open
	offsetPersistenceTest{10, 10, 0, 0},
	// just new events are read on file reopen
	offsetPersistenceTest{10, 10, 10, 10},
}

func TestOffsetPersistence(t *testing.T) {
	for _, testData := range testOffsetPersistenceTable {
		err := offsetPersistenceTestRun(testData)
		if err != nil {
			t.Error(err)
		}
	}
}

func TestGetDefaultPositionsFilePath(t *testing.T) {
	testData := map[string]string{
		"/tmp/access_log":  "/tmp/access_log.pos",
		"./access_log.pos": "./access_log.pos.pos",
	}

	for logFile, posFile := range testData {
		config := tailerConfig{TailedFile: logFile}
		assert.Equal(t, posFile, config.getDefaultPositionsFilePath())
	}
}
