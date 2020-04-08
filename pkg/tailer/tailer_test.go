package tailer

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	lineParseRegexp   = `^(?P<ip>[A-Fa-f0-9.:]{4,50}) \S+ \S+ \[(?P<time>.*?)\] "(?P<request>.*?)" (?P<statusCode>\d+) \d+ "(?P<referer>.*?)" uag="(?P<userAgent>[^"]+)" "[^"]+" ua="[^"]+" rt="(?P<requestDuration>\d+(\.\d+)??)"(?: frpc-status="(?P<frpcStatus>\d*|-)")?(?: slo-domain="(?P<sloDomain>[^"]*)")?(?: slo-app="(?P<sloApp>[^"]*)")?(?: slo-class="(?P<sloClass>[^"]*)")?(?: slo-endpoint="(?P<sloEndpoint>[^"]*)")?(?: slo-result="(?P<sloResult>[^"]*)")?`
	emptyGroupRegexp  = `^-$`
	requestLineFormat = `{ip} - - [{time}] "{request}" {statusCode} 79 "-" uag="-" "-" ua="10.66.112.78:80" rt="{requestDuration}" frpc-status="{frpcStatus}" slo-domain="{sloDomain}" slo-app="{sloApp}" slo-class="{sloClass}" slo-endpoint="{sloEndpoint}" slo-result="{sloResult}"`
	// provided to getRequestLine, this returns a considered-valid line
	requestLineFormatMapValid = map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
		"ip":              "34.65.133.58",
		"request":         "GET /robots.txt HTTP/1.1",
		"statusCode":      "200",
		"requestDuration": "0.123", // in s, as logged by nginx
		"sloClass":        "-",
		"sloDomain":       "-",
		"sloApp":          "-",
		"sloResult":       "-",
		"sloEndpoint":     "-",
		"frpcStatus":      "-",
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
	// lineContentMapping is to be used to generate the request log line via getRequestLine func
	// (see it for what the defaults are, so that you dont have to fill them for every test case)
	lineContentMapping map[string]string
	isLineValid        bool
}

func Test_ParseLineAndBuildEvent(t *testing.T) {
	testTable := []parseLineTest{
		// ipv4
		{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
			"ip":              "34.65.133.58",
			"request":         "GET /robots.txt HTTP/1.1",
			"statusCode":      "200",
			"requestDuration": "0.123", // in s, as logged by nginx
			"sloClass":        "-",
			"sloDomain":       "-",
			"sloApp":          "-",
			"sloResult":       "-",
			"sloEndpoint":     "-",
			"frpcStatus":      "-",
			"userAgent":       "-",
			"referer":         "-",
		}, true},
		// ipv6
		{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
			"ip":              "2001:718:801:230::1",
			"request":         "GET /robots.txt HTTP/1.1",
			"statusCode":      "200",
			"requestDuration": "0.123", // in s, as logged by nginx
			"sloClass":        "-",
			"sloDomain":       "-",
			"sloApp":          "-",
			"sloResult":       "-",
			"sloEndpoint":     "-",
			"frpcStatus":      "-",
			"userAgent":       "-",
			"referer":         "-",
		}, true},
		// invalid time
		{map[string]string{"time": "32/Nov/2019:25:20:07 +0100",
			"ip":              "2001:718:801:230::1",
			"request":         "GET /robots.txt HTTP/1.1",
			"statusCode":      "200x",
			"requestDuration": "0.123", // in s, as logged by nginx
			"sloClass":        "-",
			"sloDomain":       "-",
			"sloApp":          "-",
			"sloResult":       "-",
			"sloEndpoint":     "-",
			"frpcStatus":      "-",
			"userAgent":       "-",
			"referer":         "-",
		}, false},
		// invalid request
		{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
			"ip":              "2001:718:801:230::1",
			"request":         "invalid-request[eof]",
			"statusCode":      "200x",
			"requestDuration": "0.123", // in s, as logged by nginx
			"sloClass":        "-",
			"sloDomain":       "-",
			"sloApp":          "-",
			"sloResult":       "-",
			"sloEndpoint":     "-",
			"frpcStatus":      "-",
			"userAgent":       "-",
			"referer":         "-",
		}, false},
		// request without protocol
		{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
			"ip":              "2001:718:801:230::1",
			"request":         "GET /robots.txt",
			"statusCode":      "301",
			"requestDuration": "0.123", // in s, as logged by nginx
			"sloClass":        "-",
			"sloDomain":       "-",
			"sloApp":          "-",
			"sloResult":       "-",
			"sloEndpoint":     "-",
			"frpcStatus":      "-",
			"userAgent":       "-",
			"referer":         "-",
		}, true},
		// http2.0 proto request
		{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
			"ip":              "2001:718:801:230::1",
			"request":         "GET /robots.txt HTTP/2.0",
			"statusCode":      "200",
			"requestDuration": "0.123", // in s, as logged by nginx
			"sloClass":        "-",
			"sloDomain":       "-",
			"sloApp":          "-",
			"sloResult":       "-",
			"sloEndpoint":     "-",
			"frpcStatus":      "-",
			"userAgent":       "-",
			"referer":         "-",
		}, true},
		// zero status code
		{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
			"ip":              "2001:718:801:230::1",
			"request":         "GET /robots.txt HTTP/1.1",
			"statusCode":      "0",
			"requestDuration": "0.123", // in s, as logged by nginx
			"sloClass":        "-",
			"sloDomain":       "-",
			"sloApp":          "-",
			"sloResult":       "-",
			"sloEndpoint":     "-",
			"frpcStatus":      "-",
			"userAgent":       "-",
			"referer":         "-",
		}, true},
		// invalid status code
		{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
			"ip":              "2001:718:801:230::1",
			"request":         "GET /robots.txt HTTP/1.1",
			"statusCode":      "xxx",
			"requestDuration": "0.123", // in s, as logged by nginx
			"sloClass":        "-",
			"sloDomain":       "-",
			"sloApp":          "-",
			"sloResult":       "-",
			"sloEndpoint":     "-",
			"frpcStatus":      "-",
			"userAgent":       "-",
			"referer":         "-",
		}, false},
		// classified event
		{map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
			"ip":              "2001:718:801:230::1",
			"request":         "GET /robots.txt HTTP/1.1",
			"statusCode":      "200",
			"requestDuration": "0.123", // in s, as logged by nginx
			"sloClass":        "critical",
			"sloDomain":       "userportal",
			"sloApp":          "frontend-api",
			"sloResult":       "success",
			"sloEndpoint":     "AdInventoryManagerInterestsQuery",
			"frpcStatus":      "-",
			"userAgent":       "-",
			"referer":         "-",
		}, true},
	}

	lineParseRegexpCompiled := regexp.MustCompile(lineParseRegexp)
	emptyGroupRegexpCompiled := regexp.MustCompile(emptyGroupRegexp)
	for _, test := range testTable {
		requestLine := getRequestLine(test.lineContentMapping)

		data, err := parseLine(lineParseRegexpCompiled, emptyGroupRegexpCompiled, requestLine)
		if err != nil {
			if test.isLineValid {
				t.Fatalf("unable to parse request line '%s': %v", requestLine, err)
			} else {
				// the tested line is marked as not valid, Err is expected
				continue
			}
		}
		parsedEvent, err := buildEvent(data)

		var expectedEvent *event.HttpRequest

		if test.isLineValid {
			// line is considered valid, build the expectedEvent struct in order to compare it to the parsed one

			// first, drop all data which matches emptyGroupRegexpCompiled, as they should not be included in the data provided to buildEvent
			for k, v := range test.lineContentMapping {
				if emptyGroupRegexpCompiled.MatchString(v) {
					delete(test.lineContentMapping, k)
				}
			}
			expectedEvent, err = buildEvent(test.lineContentMapping)
			if err != nil {
				t.Fatalf("Unable to build event from test data: %v", err)
			}
			if !reflect.DeepEqual(expectedEvent, parsedEvent) {
				t.Errorf("Unexpected result of parse line: %s\nGot: %+v\nExpected: %+v", requestLine, parsedEvent, expectedEvent)
			}
		} else {
			// line is not valid, just check that err is returned
			if err == nil {
				t.Errorf("Line wrongly considered as valid: %s", requestLine)
			}
		}

	}
}

func Test_ParseLine(t *testing.T) {
	testTable := []parseLineTest{
		{
			lineContentMapping: map[string]string{"time": "12/Nov/2019:10:20:07 +0100",
				"ip":              "34.65.133.58",
				"request":         "GET /robots.txt HTTP/1.1",
				"statusCode":      "200",
				"requestDuration": "0.123", // in s, as logged by nginx
				"sloClass":        "-",
				"sloDomain":       "-",
				"sloApp":          "-",
				"sloResult":       "-",
				"sloEndpoint":     "-",
				"frpcStatus":      "-",
			},
			isLineValid: true,
		},
	}
	lineParseRegexpCompiled := regexp.MustCompile(lineParseRegexp)
	emptyGroupRegexpCompiled := regexp.MustCompile(emptyGroupRegexp)

	for _, test := range testTable {
		requestLine := getRequestLine(test.lineContentMapping)
		data, err := parseLine(lineParseRegexpCompiled, emptyGroupRegexpCompiled, requestLine)
		if err != nil {
			t.Fatalf("unable to parse request line '%s': %v", requestLine, err)
		}
		for k, v := range test.lineContentMapping {
			if !emptyGroupRegexpCompiled.MatchString(v) {
				continue
			}
			// test that empty group was correctly replaced by an empty string
			if _, ok := data[k]; ok {
				t.Errorf("Content named group '%s':'%s' should not have been included in the resulting stringmap (as value matches emptyGroupRegexp: '%s'): %+v", k, v, emptyGroupRegexp, data)
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
func countEvents(in chan *event.HttpRequest, out chan int) {
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
		LoglineParseRegexp:          lineParseRegexp,
		EmptyGroupRE:                emptyGroupRegexp,
	}
	tailer, err := New(config, logrus.New())
	if err != nil {
		return err
	}

	tailer.Run()
	go countEvents(tailer.OutputChannel(), eventCount)

	for i := 0; i < t.during; i++ {
		f.WriteString(getRequestLine(requestLineFormatMapValid) + "\n")
	}
	time.Sleep(100 * time.Millisecond)

	tailer.Stop()
	eventsCount := <-eventCount

	if eventsCount != t.pre+t.during {
		return fmt.Errorf("Number of processed events during first open of a log file does not match: got '%d', expected '%d'", eventsCount, t.pre+t.during)
	}

	for i := 0; i < t.post; i++ {
		f.WriteString(getRequestLine(requestLineFormatMapValid) + "\n")
	}

	tailer, err = New(config, logrus.New())
	if err != nil {
		return err
	}
	tailer.Run()
	go countEvents(tailer.OutputChannel(), eventCount)

	for i := 0; i < t.reopen; i++ {
		f.WriteString(getRequestLine(requestLineFormatMapValid) + "\n")
	}

	time.Sleep(100 * time.Millisecond)

	tailer.Stop()
	eventsCount = <-eventCount

	if eventsCount != t.post+t.reopen {
		return fmt.Errorf("Number of processed events after reopening a log file does not match: got '%d', expected '%d'", eventsCount, t.post+t.reopen)
	}

	return nil
}

var testOffsetPersistenceTable = []offsetPersistenceTest{
	// events in log file present before the first open
	{10, 10, 0, 0},
	// just new events are read on file reopen
	{10, 10, 10, 10},
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
