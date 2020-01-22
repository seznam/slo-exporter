package tailer

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/hpcloud/tail"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

const timeLayout string = "02/Jan/2006:15:04:05 -0700"

var (
	lineParseRegexp = regexp.MustCompile(`^(?P<ip>[A-Fa-f0-9.:]{4,50}) \S+ \S+ \[(?P<time>.*?)] "(?P<request>.*?)" (?P<statusCode>\d+) \d+ "(?P<referer>.*?)" uag="(?P<userAgent>[^"]+)" "[^"]+" ua="[^"]+" rt="(?P<requestDuration>\d+(\.\d+)??)"`)
)

// Tailer is an instance of github.com/hpcloud/tail dedicated to a single file
type Tailer struct {
	filename string
	tail     *tail.Tail
}

// New returns an instance of Tailer
func New(filename string, follow bool, reopen bool) (*Tailer, error) {
	tail, err := tail.TailFile(filename, tail.Config{Follow: follow, ReOpen: reopen, MustExist: true})
	if err != nil {
		return nil, err
	}
	tailer := &Tailer{filename, tail}
	return tailer, nil
}

// Run starts to tail the associated file, feeding RequestEvents, errors into separated channels.
// Close eventsChan based on done chan signal (close, any read)
// Content of RequestEvent structure depends on input log lines. So not all information may be present or valid, though basic validation is being made.
// E.g.:
// - RequestEvent.IP may be nil in case invalid IP address is given in logline
// - Slo* fields may not be filled at all
// - Content of RequestEvent.Headers may vary
func (t Tailer) Run(done chan struct{}, eventsChan chan *producer.RequestEvent, errChan chan error) {
	go func() {
		defer close(eventsChan)
		defer t.tail.Cleanup()

		for {
			select {
			case <-done:
				return
			case line, ok := <-t.tail.Lines:
				if line.Err != nil {
					log.Error(line.Err)
				}
				event, err := parseLine(line.Text)
				if err != nil {
					reportErrLine(line.Text, err)
				} else {
					eventsChan <- event
				}
				if !ok {
					return
				}
			}
		}
	}()
}

// reportErrLine does the necessary reporting in case a wrong line occurs
func reportErrLine(line string, err error) {
	// TODO increment metrics
	log.Errorf("Error (%v) while parsing line: %s", err, line)
}

type InvalidRequestError struct {
	request string
}

func (e *InvalidRequestError) Error() string {
	return fmt.Sprintf("Invalid request: %s", e.request)
}

// parseRequestLine parses request line (see https://www.w3.org/Protocols/rfc2616/rfc2616-sec5.html)
// golang's http/parseRequestLine is too strict, it does not consider missing HTTP protocol as a valid request line
// however we need to accept those as well
func parseRequestLine(requestLine string) (string, string, string, error) {
	requestLineArr := strings.Fields(requestLine)
	// protocol is missing, happens in case of redirects
	if len(requestLineArr) == 2 {
		return requestLineArr[0], requestLineArr[1], "", nil
	}
	// full valid request line
	if len(requestLineArr) == 3 {
		return requestLineArr[0], requestLineArr[1], requestLineArr[2], nil
	}
	// in other cases we consider the request as invalid
	return "", "", "", &InvalidRequestError{requestLine}
}

// parseLine parses the given line, producing a RequestEvent instance
// - lineParseRegexp is used to parse the line
// - RequestEvent.IP may
func parseLine(line string) (*producer.RequestEvent, error) {
	lineData := make(map[string]string)

	match := lineParseRegexp.FindStringSubmatch(line)
	if len(match) != len(lineParseRegexp.SubexpNames()) {
		return nil, fmt.Errorf("Unable to parse line")
	}
	for i, name := range lineParseRegexp.SubexpNames() {
		if i != 0 && name != "" {
			lineData[name] = match[i]
		}
	}

	t, err := time.Parse(timeLayout, lineData["time"])
	if err != nil {
		return nil, err
	}

	duration, err := time.ParseDuration(lineData["requestDuration"] + "ms")
	if err != nil {
		return nil, err
	}

	statusCode, err := strconv.Atoi(lineData["statusCode"])
	if err != nil {
		return nil, err
	}
	if statusCode < 100 || statusCode > 599 {
		return nil, fmt.Errorf("Invalid HTTP status: %d", statusCode)
	}

	method, requestURI, _, err := parseRequestLine(lineData["request"])
	if err != nil {
		return nil, err
	}

	url, err := url.Parse(requestURI)
	if err != nil {
		return nil, err
	}

	return &producer.RequestEvent{
		Time:       t,
		IP:         net.ParseIP(lineData["ip"]),
		Duration:   duration,
		URL:        url,
		StatusCode: statusCode,
		Headers:    make(map[string]string),
		Method:     method,
	}, nil
}
