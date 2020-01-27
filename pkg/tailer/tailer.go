package tailer

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hpcloud/tail"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

const timeLayout string = "02/Jan/2006:15:04:05 -0700"

var (
	log             *logrus.Entry
	lineParseRegexp = regexp.MustCompile(`^(?P<ip>[A-Fa-f0-9.:]{4,50}) \S+ \S+ \[(?P<time>.*?)] "(?P<request>.*?)" (?P<statusCode>\d+) \d+ "(?P<referer>.*?)" uag="(?P<userAgent>[^"]+)" "[^"]+" ua="[^"]+" rt="(?P<requestDuration>\d+(\.\d+)??)"`)

	linesReadTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "slo_exporter",
		Subsystem: "tailer",
		Name:      "lines_read_total",
		Help:      "Total number of lines tailed from the file.",
	})
	malformedLinesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "slo_exporter",
			Subsystem: "tailer",
			Name:      "malformed_lines_total",
			Help:      "Total number of invalid lines that faild to prse.",
		},
		[]string{"reason"},
	)
)

func init() {
	log = logrus.WithField("component", "tailer")
	prometheus.MustRegister(linesReadTotal, malformedLinesTotal)
}

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
func (t Tailer) Run(ctx context.Context, eventsChan chan *producer.RequestEvent, errChan chan error) {
	go func() {
		defer close(eventsChan)
		defer t.tail.Cleanup()
		defer log.Info("stopping tailer")

		for {
			select {
			case <-ctx.Done():
				return
			case line, ok := <-t.tail.Lines:
				if !ok {
					log.Info("tail lines channel has been closed")
					return
				}
				if line.Err != nil {
					log.Error(line.Err)
				}
				linesReadTotal.Inc()
				event, err := parseLine(line.Text)
				if err != nil {
					malformedLinesTotal.WithLabelValues(err.Error()).Inc()
					reportErrLine(line.Text, err)
				} else {
					eventsChan <- event
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

// InvalidRequestError is error representing invalid RequestEvent
type InvalidRequestError struct {
	request string
}

func (e *InvalidRequestError) Error() string {
	return fmt.Sprintf("Request '%s' contains unexpected number of fields", e.request)
}

// parseRequestLine parses request line (see https://www.w3.org/Protocols/rfc2616/rfc2616-sec5.html)
// golang's http/parseRequestLine is too strict, it does not consider missing HTTP protocol as a valid request line
// however we need to accept those as well
func parseRequestLine(requestLine string) (method string, uri string, protocol string, err error) {
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
		return nil, fmt.Errorf("Unable to parse time '%s' using the format '%s': %w", lineData["time"], timeLayout, err)
	}

	requestDuration := lineData["requestDuration"] + "ms"
	duration, err := time.ParseDuration(requestDuration)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse duration '%s': %w", requestDuration, err)
	}

	statusCode, err := strconv.Atoi(lineData["statusCode"])
	if err != nil {
		return nil, fmt.Errorf("Invalid HTTP status code '%d': %w", statusCode, err)
	}

	method, requestURI, _, err := parseRequestLine(lineData["request"])
	if err != nil {
		return nil, fmt.Errorf("Unable to parse request line '%s': %w", lineData["request"], err)
	}

	url, err := url.Parse(requestURI)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse url '%s': %w", requestURI, err)
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
