package tailer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/loki/pkg/promtail/positions"
	"github.com/hpcloud/tail"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"

	logrusAdapter "github.com/go-kit/kit/log/logrus"
)

const (
	timeLayout string = "02/Jan/2006:15:04:05 -0700"
	component  string = "tailer"
)

var (
	log             *logrus.Entry
	lineParseRegexp = regexp.MustCompile(`^(?P<ip>[A-Fa-f0-9.:]{4,50}) \S+ \S+ \[(?P<time>.*?)] "(?P<request>.*?)" (?P<statusCode>\d+) \d+ "(?P<referer>.*?)" uag="(?P<userAgent>[^"]+)" "[^"]+" ua="[^"]+" rt="(?P<requestDuration>\d+(\.\d+)??)"`)

	linesReadTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "slo_exporter",
		Subsystem: component,
		Name:      "lines_read_total",
		Help:      "Total number of lines tailed from the file.",
	})
	malformedLinesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "slo_exporter",
			Subsystem: component,
			Name:      "malformed_lines_total",
			Help:      "Total number of invalid lines that failed to parse.",
		},
		[]string{"reason"},
	)
	fileSizeBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "slo_exporter",
		Subsystem: component,
		Name:      "file_size_bytes",
		Help:      "Size of the tailed file in bytes.",
	})
	fileOffsetBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "slo_exporter",
		Subsystem: component,
		Name:      "file_offset_bytes",
		Help:      "Current tailing offset within the file in bytes (from the beginning of the file).",
	})
)

func init() {
	log = logrus.WithFields(logrus.Fields{"component": component})
	prometheus.MustRegister(linesReadTotal, malformedLinesTotal, fileSizeBytes, fileOffsetBytes)
}

// Tailer is an instance of github.com/hpcloud/tail dedicated to a single file
type Tailer struct {
	filename                string
	tail                    *tail.Tail
	positions               *positions.Positions
	persistPositionInterval time.Duration
}

// getDefaultPositionsFilePath derives positions file path for given tailed filename
func getDefaultPositionsFilePath(filename string) string {
	dir, file := filepath.Split(filename)
	if file[0:1] != "." {
		file = fmt.Sprintf(".%s.pos", file)
	} else {
		file = fmt.Sprintf("%s.pos", file)
	}
	return filepath.Join(dir, file)
}

// New returns an instance of Tailer
func New(filename string, follow bool, reopen bool, persistPositionFile string, persistPositionInterval time.Duration) (*Tailer, error) {

	var (
		offset int64
		err    error
		pos    *positions.Positions
	)

	if persistPositionFile == "" {
		persistPositionFile = getDefaultPositionsFilePath(filename)
	}
	pos, err = positions.New(logrusAdapter.NewLogrusLogger(log), positions.Config{persistPositionInterval, persistPositionFile})
	if err != nil {
		return nil, fmt.Errorf("could not initialize file position persister: %v", err)
	}
	// check that loaded position for a file is valid
	fstat, err := os.Stat(filename)
	if err != nil {
		return nil, fmt.Errorf("could not check that loaded offset is valid: %w", err)
	}
	offset, err = pos.Get(filename)
	if err != nil {
		return nil, err
	}
	if fstat.Size() < offset {
		pos.Remove(filename)
		offset = 0
		log.Warnf("Loaded position '%d' for the file is larger that the file size '%d'. Tailer will start from the beginning of the file.", offset, fstat.Size())
	}

	tailFile, err := tail.TailFile(filename, tail.Config{
		Follow:    follow,
		ReOpen:    reopen,
		MustExist: true,
		Location:  &tail.SeekInfo{Offset: offset, Whence: io.SeekStart},
		// tail library has claimed problems with inotify: https://github.com/grafana/loki/commit/c994823369d65785e72c4247fd50c656801e429a
		Poll: true,
	})
	if err != nil {
		return nil, err
	}

	return &Tailer{filename, tailFile, pos, persistPositionInterval}, nil
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
		ticker := time.NewTicker(t.persistPositionInterval)
		defer ticker.Stop()
		defer t.positions.Stop()

		quitting := false
		for {
			select {
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
			case <-ticker.C:
				if !quitting {
					// get current offset from tail.TailFile instance
					if err := t.markOffsetPosition(); err != nil {
						log.Error(err)
					}
				}
			case <-ctx.Done():
				if !quitting {
					quitting = true
					// we need to perform this strictly once, as tail return 0 offset when already stopped
					if err := t.markOffsetPosition(); err != nil {
						log.Error(err)
					}
					go t.tail.Stop()
				}
			}
		}
	}()
}

// marks current file offset and size for the use of:
// - offset persistence
// - prometheus metrics
func (t *Tailer) markOffsetPosition() error {
	// we may lose a log line due to claimed inaccuracy of Tail.tell (https://godoc.org/github.com/hpcloud/tail#Tail.Tell)
	offset, err := t.tail.Tell()
	if err != nil {
		if errors.Unwrap(err) != nil {
			// include more details about the file inaccessibility, if possible
			return fmt.Errorf("could not get the file offset: %w", errors.Unwrap(err))
		} else {
			return fmt.Errorf("could not get the file offset: %w", err)
		}
	}
	fileOffsetBytes.Set(float64(offset))
	t.positions.Put(t.filename, offset)

	fstat, err := os.Stat(t.filename)
	if err != nil {
		return fmt.Errorf("unable to get file size: %w", err)
	}
	fileSizeBytes.Set(float64(fstat.Size()))

	return nil
}

// reportErrLine does the necessary reporting in case a wrong line occurs
func reportErrLine(line string, err error) {
	// TODO increment metrics
	log.Errorf("err (%w) while parsing the line: %s", err, line)
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
		return nil, fmt.Errorf("unable to parse line")
	}
	for i, name := range lineParseRegexp.SubexpNames() {
		if i != 0 && name != "" {
			lineData[name] = match[i]
		}
	}

	t, err := time.Parse(timeLayout, lineData["time"])
	if err != nil {
		return nil, fmt.Errorf("unable to parse time '%s' using the format '%s': %w", lineData["time"], timeLayout, err)
	}

	requestDuration := lineData["requestDuration"] + "s"
	duration, err := time.ParseDuration(requestDuration)
	if err != nil {
		return nil, fmt.Errorf("unable to parse duration '%s': %w", requestDuration, err)
	}

	statusCode, err := strconv.Atoi(lineData["statusCode"])
	if err != nil {
		return nil, fmt.Errorf("invalid HTTP status code '%d': %w", statusCode, err)
	}

	method, requestURI, _, err := parseRequestLine(lineData["request"])
	if err != nil {
		return nil, fmt.Errorf("unable to parse request line '%s': %w", lineData["request"], err)
	}

	url, err := url.Parse(requestURI)
	if err != nil {
		return nil, fmt.Errorf("unable to parse url '%s': %w", requestURI, err)
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
