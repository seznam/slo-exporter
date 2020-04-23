package tailer

import (
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/pipeline"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"io"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	logrusAdapter "github.com/go-kit/kit/log/logrus"
	"github.com/grafana/loki/pkg/promtail/positions"
	"github.com/hpcloud/tail"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const (
	timeLayout string = "02/Jan/2006:15:04:05 -0700"

	timeGroupName            = "time"
	requestDurationGroupName = "requestDuration"
	statusCodeGroupName      = "statusCode"
	requestGroupName         = "request"
	ipGroupName              = "ip"
	sloResultGroupName       = "sloResult"
	sloDomainGroupName       = "sloDomain"
	sloAppGroupName          = "sloApp"
	sloClassGroupName        = "sloClass"
)

var (
	knownGroupNames = []string{timeGroupName, requestDurationGroupName, statusCodeGroupName, requestGroupName, ipGroupName, sloResultGroupName, sloDomainGroupName, sloAppGroupName, sloClassGroupName}

	linesReadTotal = prometheus.NewCounter(prometheus.CounterOpts{

		Name: "lines_read_total",
		Help: "Total number of lines tailed from the file.",
	})
	malformedLinesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "malformed_lines_total",
			Help: "Total number of invalid lines that failed to parse.",
		},
	)
	fileSizeBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "file_size_bytes",
		Help: "Size of the tailed file in bytes.",
	})
	fileOffsetBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "file_offset_bytes",
		Help: "Current tailing offset within the file in bytes (from the beginning of the file).",
	})
)

type tailerConfig struct {
	TailedFile                  string
	Follow                      bool
	Reopen                      bool
	PositionFile                string
	PositionPersistenceInterval time.Duration
	LoglineParseRegexp          string
	EmptyGroupRE                string
}

// getDefaultPositionsFilePath derives positions file path for given tailed filename
func (c *tailerConfig) getDefaultPositionsFilePath() string {
	return c.TailedFile + ".pos"
}

// Tailer is an instance of github.com/hpcloud/tail dedicated to a single file
type Tailer struct {
	filename                string
	tail                    *tail.Tail
	positions               *positions.Positions
	persistPositionInterval time.Duration
	observer                pipeline.EventProcessingDurationObserver
	lineParseRegexp         *regexp.Regexp
	emptyGroupRegexp        *regexp.Regexp
	outputChannel           chan *event.HttpRequest
	shutdownChannel         chan struct{}
	logger                  logrus.FieldLogger
	done                    bool
}

func (t *Tailer) String() string {
	return "tailer"
}

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*Tailer, error) {
	viperConfig.SetDefault("Follow", true)
	viperConfig.SetDefault("Reopen", true)
	viperConfig.SetDefault("PositionPersistenceInterval", 2*time.Second)
	viperConfig.SetDefault("EmptyGroupRE", "^$")
	var config tailerConfig
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return New(config, logger)
}

// New returns an instance of Tailer
func New(config tailerConfig, logger logrus.FieldLogger) (*Tailer, error) {
	var (
		offset int64
		err    error
		pos    *positions.Positions
	)

	if config.PositionFile == "" {
		config.PositionFile = config.getDefaultPositionsFilePath()
	}
	pos, err = positions.New(logrusAdapter.NewLogrusLogger(logger), positions.Config{SyncPeriod: config.PositionPersistenceInterval, PositionsFile: config.PositionFile})
	if err != nil {
		return nil, fmt.Errorf("could not initialize file position persister: %+v", err)
	}
	// check that loaded position for a file is valid
	fstat, err := os.Stat(config.TailedFile)
	if err != nil {
		return nil, fmt.Errorf("could not check that loaded offset is valid: %w", err)
	}
	offset, err = pos.Get(config.TailedFile)
	if err != nil {
		return nil, err
	}
	fileSize := fstat.Size()
	if fileSize < offset {
		logger.WithField("file", config.TailedFile).Warnf("loaded position '%d' for the file is larger that the file size '%d'. Tailer will start from the beginning of the file.", offset, fileSize)
		pos.Remove(config.TailedFile)
		offset = 0
	}

	if !config.Follow && config.Reopen {
		return nil, fmt.Errorf("cannot use reopen without follow")
	}
	tailFile, err := tail.TailFile(config.TailedFile, tail.Config{
		Follow:    config.Follow,
		ReOpen:    config.Reopen,
		MustExist: true,
		Location:  &tail.SeekInfo{Offset: offset, Whence: io.SeekStart},
		// tail library has claimed problems with inotify: https://github.com/grafana/loki/commit/c994823369d65785e72c4247fd50c656801e429a
		Poll:   true,
		Logger: logger,
	})
	if err != nil {
		return nil, err
	}

	lineParseRegexp, err := regexp.Compile(config.LoglineParseRegexp)
	if err != nil {
		return nil, fmt.Errorf("error while compiling the line parse RE ('%s'): %w", config.LoglineParseRegexp, err)
	}
	emptyGroupRegexp, err := regexp.Compile(config.EmptyGroupRE)
	if err != nil {
		return nil, fmt.Errorf("error while compiling the empty group matching RE ('%s'): %w", config.EmptyGroupRE, err)
	}

	return &Tailer{
		filename:                config.TailedFile,
		tail:                    tailFile,
		positions:               pos,
		persistPositionInterval: config.PositionPersistenceInterval,
		lineParseRegexp:         lineParseRegexp,
		emptyGroupRegexp:        emptyGroupRegexp,
		outputChannel:           make(chan *event.HttpRequest),
		shutdownChannel:         make(chan struct{}),
		done:                    false,
		logger:                  logger,
	}, nil
}

func (t *Tailer) RegisterEventProcessingDurationObserver(observer pipeline.EventProcessingDurationObserver) {
	t.observer = observer
}

func (t *Tailer) observeDuration(start time.Time) {
	if t.observer != nil {
		t.observer.Observe(time.Since(start).Seconds())
	}
}

func (t *Tailer) RegisterMetrics(_ prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	toRegister := []prometheus.Collector{linesReadTotal, malformedLinesTotal, fileSizeBytes, fileOffsetBytes}
	for _, collector := range toRegister {
		if err := wrappedRegistry.Register(collector); err != nil {
			return fmt.Errorf("error registering metric %s: %w", collector, err)
		}
	}
	return nil
}

func (t *Tailer) Done() bool {
	return t.done
}

func (t *Tailer) OutputChannel() chan *event.HttpRequest {
	return t.outputChannel
}

// Run starts to tail the associated file, feeding RequestEvents, errors into separated channels.
// Close eventsChan based on done chan signal (close, any read)
// Content of RequestEvent structure depends on input log lines. So not all information may be present or valid, though basic validation is being made.
// E.g.:
// - RequestEvent.IP may be nil in case invalid IP address is given in logline
// - Slo* fields may not be filled at all
// - Content of RequestEvent.Headers may vary
func (t *Tailer) Run() {
	go func() {
		ticker := time.NewTicker(t.persistPositionInterval)
		defer func() {
			t.positions.Stop()
			ticker.Stop()
			t.tail.Cleanup()
			close(t.outputChannel)
			t.done = true
		}()
		quitting := false
		for {
			select {
			case line, ok := <-t.tail.Lines:
				if !ok {
					t.logger.Info("input channel closed, finishing")
					return
				}
				start := time.Now()
				if line.Err != nil {
					t.logger.Error(line.Err)
				}
				linesReadTotal.Inc()
				newEvent, err := t.processLine(line.Text)
				if err != nil {
					malformedLinesTotal.Inc()
					t.logger.WithField("line", line).Errorf("err (%+v) while parsing line", err)
				} else {
					t.outputChannel <- newEvent
				}
				t.observeDuration(start)
			case <-ticker.C:
				if !quitting {
					// get current offset from tail.TailFile instance
					if err := t.markOffsetPosition(); err != nil {
						t.logger.Error(err)
					}
				}
			case <-t.shutdownChannel:
				if !quitting {
					quitting = true
					// we need to perform this strictly once, as tail return 0 offset when already stopped
					if err := t.markOffsetPosition(); err != nil {
						t.logger.Error(err)
					}
					// keep this in goroutine as this may block on tail's goroutine trying to write into t.tail.Lines
					go func() {
						if err := t.tail.Stop(); err != nil {
							t.logger.Errorf("failed to stop Tailer: %v", err)
						}
					}()
				}
			}
		}
	}()
}

func (t *Tailer) Stop() {
	if !t.done {
		t.shutdownChannel <- struct{}{}
	}
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

// buildEvent returns *event.HttpRequest based on input lineData
func buildEvent(lineData stringmap.StringMap) (*event.HttpRequest, error) {
	t, err := time.Parse(timeLayout, lineData[timeGroupName])
	if err != nil {
		return nil, fmt.Errorf("unable to parse time '%s' using the format '%s': %w", lineData[timeGroupName], timeLayout, err)
	}

	requestDuration := lineData[requestDurationGroupName] + "s"
	duration, err := time.ParseDuration(requestDuration)
	if err != nil {
		return nil, fmt.Errorf("unable to parse duration '%s': %w", requestDuration, err)
	}

	statusCode, err := strconv.Atoi(lineData[statusCodeGroupName])
	if err != nil {
		return nil, fmt.Errorf("invalid HTTP status code '%d': %w", statusCode, err)
	}

	method, requestURI, _, err := parseRequestLine(lineData[requestGroupName])
	if err != nil {
		return nil, fmt.Errorf("unable to parse request line '%s': %w", lineData[requestGroupName], err)
	}

	parsedUrl, err := url.Parse(requestURI)
	if err != nil {
		return nil, fmt.Errorf("unable to parse parsedUrl '%s': %w", requestURI, err)
	}

	classification := &event.SloClassification{
		Domain: lineData[sloDomainGroupName],
		App:    lineData[sloAppGroupName],
		Class:  lineData[sloClassGroupName],
	}

	return &event.HttpRequest{
		Time:              t,
		IP:                net.ParseIP(lineData[ipGroupName]),
		Duration:          duration,
		URL:               parsedUrl,
		StatusCode:        statusCode,
		Headers:           lineData.Copy().Without(knownGroupNames),
		Metadata:          lineData,
		Method:            method,
		SloResult:         lineData[sloResultGroupName],
		SloClassification: classification,
		Quantity:          1,
	}, nil
}

// parseLine parses the given line, producing a RequestEvent instance
// - lineParseRegexp is used to parse the line
// - if content of any of the matched named groups matches emptyGroupRegexp, it is replaced by an empty string ""
func parseLine(lineParseRegexp *regexp.Regexp, emptyGroupRegexp *regexp.Regexp, line string) (map[string]string, error) {
	lineData := make(map[string]string)

	match := lineParseRegexp.FindStringSubmatch(line)
	if len(match) != len(lineParseRegexp.SubexpNames()) {
		return nil, fmt.Errorf("unable to parse line")
	}
	for i, name := range lineParseRegexp.SubexpNames() {
		if i == 0 || name == "" || emptyGroupRegexp.MatchString(match[i]) {
			continue
		}
		lineData[name] = match[i]
	}

	return lineData, nil
}

func (t *Tailer) processLine(line string) (*event.HttpRequest, error) {
	lineData, err := parseLine(t.lineParseRegexp, t.emptyGroupRegexp, line)
	if err != nil {
		return nil, err
	}
	e, err := buildEvent(lineData)
	// TODO refactor once new module for setting eventKey from metadata will be available
	e.SetEventKey(lineData["eventKey"])
	return e, err
}
