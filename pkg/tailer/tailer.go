package tailer

import (
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/pipeline"
	"io"
	"os"
	"regexp"
	"time"

	logrusAdapter "github.com/go-kit/kit/log/logrus"
	"github.com/grafana/loki/pkg/promtail/positions"
	"github.com/hpcloud/tail"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
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
	positions               positions.Positions
	persistPositionInterval time.Duration
	observer                pipeline.EventProcessingDurationObserver
	lineParseRegexp         *regexp.Regexp
	emptyGroupRegexp        *regexp.Regexp
	outputChannel           chan *event.Raw
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
		pos    positions.Positions
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
		outputChannel:           make(chan *event.Raw),
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

func (t *Tailer) OutputChannel() chan *event.Raw {
	return t.outputChannel
}

// Run starts to tail the associated file, feeding events to output channel.
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

func (t *Tailer) processLine(line string) (*event.Raw, error) {
	lineData, err := parseLine(t.lineParseRegexp, t.emptyGroupRegexp, line)
	if err != nil {
		return nil, err
	}
	return &event.Raw{Quantity: 1, Metadata: lineData}, nil
}
