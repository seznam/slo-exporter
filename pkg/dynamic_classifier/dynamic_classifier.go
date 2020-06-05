//revive:disable:var-naming
package dynamic_classifier

//revive:enable:var-naming

import (
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/pipeline"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/iancoleman/strcase"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const (
	classifiedEventLabel   = "classified"
	unclassifiedEventLabel = "unclassified"
)

var (
	// TODO matcher cache size, matcher or something related has to implement the prometheus.Collector interface and count the items.
	matcherOperationDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "matcher_operation_duration_seconds",
			Help:    "Histogram of duration matcher operations in dynamic classifier.",
			Buckets: prometheus.ExponentialBuckets(0.0001, 5, 7),
		}, []string{"operation", "matcher_type"})

	errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "errors_total",
			Help: "Total number of processed events by result.",
		},
		[]string{"type"},
	)
)

type classifierConfig struct {
	UnclassifiedEventMetadataKeys []string
	ExactMatchesCsvFiles          []string
	RegexpMatchesCsvFiles         []string
}

// DynamicClassifier is classifier based on cache and regexp matches
type DynamicClassifier struct {
	exactMatches                  matcher
	regexpMatches                 matcher
	unclassifiedEventMetadataKeys []string
	eventsMetric                  *prometheus.CounterVec
	observer                      pipeline.EventProcessingDurationObserver
	inputChannel                  chan *event.HttpRequest
	outputChannel                 chan *event.HttpRequest
	logger                        logrus.FieldLogger
	done                          bool
}

func (dc *DynamicClassifier) RegisterInMux(router *mux.Router) {
	router.HandleFunc("/matchers/{matcher}", func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		matcherType := vars["matcher"]
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%s.csv", matcherType))
		err := dc.DumpCSV(w, matcherType)
		if err != nil {
			http.Error(w, "Failed to dump matcher '"+matcherType+"': "+err.Error(), http.StatusInternalServerError)
		}

	})
}

func (dc *DynamicClassifier) String() string {
	return "dynamicClassifier"
}

func (dc *DynamicClassifier) RegisterMetrics(_ prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	toRegister := []prometheus.Collector{dc.eventsMetric, errorsTotal, matcherOperationDurationSeconds}
	for _, collector := range toRegister {
		if err := wrappedRegistry.Register(collector); err != nil {
			return err
		}
	}
	return nil
}

func (dc *DynamicClassifier) Done() bool {
	return dc.done
}

func (dc *DynamicClassifier) Stop() {
	return
}

func (dc *DynamicClassifier) SetInputChannel(channel chan *event.HttpRequest) {
	dc.inputChannel = channel
}

func (dc *DynamicClassifier) OutputChannel() chan *event.HttpRequest {
	return dc.outputChannel
}

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*DynamicClassifier, error) {
	var config classifierConfig
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return New(config, logger)
}

func metadataKeyToLabel(metadataKey string) string {
	return "metadata_" + strcase.ToSnake(metadataKey)
}

// New returns new instance of DynamicClassifier
func New(conf classifierConfig, logger logrus.FieldLogger) (*DynamicClassifier, error) {
	sort.Strings(conf.UnclassifiedEventMetadataKeys)
	classifier := DynamicClassifier{
		exactMatches:                  newMemoryExactMatcher(logger),
		regexpMatches:                 newRegexpMatcher(logger),
		unclassifiedEventMetadataKeys: conf.UnclassifiedEventMetadataKeys,
		inputChannel:                  make(chan *event.HttpRequest),
		outputChannel:                 make(chan *event.HttpRequest),
		done:                          false,
		logger:                        logger,
	}
	classifier.initializeEventsMetric()
	if err := classifier.LoadExactMatchesFromMultipleCSV(conf.ExactMatchesCsvFiles); err != nil {
		return nil, fmt.Errorf("failed to load exact matches from CSV: %w", err)
	}
	if err := classifier.LoadRegexpMatchesFromMultipleCSV(conf.RegexpMatchesCsvFiles); err != nil {
		return nil, fmt.Errorf("failed to load regexp matches from CSV: %w", err)
	}
	return &classifier, nil
}

func (dc *DynamicClassifier) initializeEventsMetric() {
	labels := []string{"result", "classified_by", "status_code"}
	for _, key := range dc.unclassifiedEventMetadataKeys {
		labels = append(labels, metadataKeyToLabel(key))
	}
	dc.eventsMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "events_processed_total",
			Help: "Total number of processed events by result.",
		},
		labels,
	)
}

func (dc *DynamicClassifier) reportEvent(result, classifiedBy, statusCode string, metadata stringmap.StringMap) {
	labels := stringmap.StringMap{"result": result, "classified_by": classifiedBy, "status_code": statusCode}
	for _, key := range dc.unclassifiedEventMetadataKeys {
		if result == unclassifiedEventLabel {
			labels[metadataKeyToLabel(key)] = metadata[key]
		} else {
			labels[metadataKeyToLabel(key)] = ""
		}

	}
	dc.eventsMetric.With(prometheus.Labels(labels)).Inc()
}

func (dc *DynamicClassifier) RegisterEventProcessingDurationObserver(observer pipeline.EventProcessingDurationObserver) {
	dc.observer = observer
}

func (dc *DynamicClassifier) observeDuration(start time.Time) {
	if dc.observer != nil {
		dc.observer.Observe(time.Since(start).Seconds())
	}
}

// LoadExactMatchesFromMultipleCSV loads exact matches from csv
func (dc *DynamicClassifier) LoadExactMatchesFromMultipleCSV(paths []string) error {
	return dc.loadMatchesFromMultipleCSV(dc.exactMatches, paths)
}

// LoadRegexpMatchesFromMultipleCSV loads regexp matches from csv
func (dc *DynamicClassifier) LoadRegexpMatchesFromMultipleCSV(paths []string) error {
	return dc.loadMatchesFromMultipleCSV(dc.regexpMatches, paths)
}

func (dc *DynamicClassifier) loadMatchesFromMultipleCSV(matcher matcher, paths []string) error {
	var errs error
	for _, p := range paths {
		if err := dc.loadMatchesFromCSV(matcher, p); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

func (dc *DynamicClassifier) loadMatchesFromCSV(matcher matcher, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	defer func() {
		if err := file.Close(); err != nil {
			dc.logger.WithField("path", path).Errorf("failed to close the CSV")
		}
	}()

	csvReader := csv.NewReader(file)

	for {
		line, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		sloDomain := line[0]
		sloApp := line[1]
		sloClass := line[2]
		sloEndpoint := line[3]
		classification := &event.SloClassification{
			Domain: sloDomain,
			App:    sloApp,
			Class:  sloClass,
		}

		err = matcher.set(sloEndpoint, classification)
		if err != nil {
			dc.logger.Errorf("failed to load match: %+v", err)
		}
	}

	return nil
}

// Classify classifies endpoint by updating its Classification field
func (dc *DynamicClassifier) Classify(newEvent *event.HttpRequest) (bool, error) {
	var (
		classificationErrors error
		classification       *event.SloClassification
		classifiedBy         matcherType
	)
	if newEvent.IsClassified() {
		if err := dc.exactMatches.set(newEvent.EventKey(), newEvent.SloClassification); err != nil {
			return true, fmt.Errorf("failed to set the exact matcher: %w", err)
		}
		return true, nil
	}

	classifiers := []matcher{dc.exactMatches, dc.regexpMatches}
	for _, classifier := range classifiers {
		var err error
		classification, err = dc.classifyByMatch(classifier, newEvent)
		if err != nil {
			dc.logger.Errorf("error while classifying event: %+v", err)
			classificationErrors = multierror.Append(classificationErrors, err)
		}
		if classification != nil {
			classifiedBy = classifier.getType()
			break
		}
	}

	if classification == nil {
		dc.reportEvent(unclassifiedEventLabel, string(classifiedBy), strconv.Itoa(newEvent.StatusCode), newEvent.Metadata)
		return false, classificationErrors
	}

	dc.logger.Debugf("event '%s' matched by %s matcher", newEvent.EventKey(), classifiedBy)
	newEvent.UpdateSLOClassification(classification)
	dc.reportEvent(classifiedEventLabel, string(classifiedBy), "", newEvent.Metadata)

	// Those matched by regex we want to write to the exact matcher so it is cached
	if classifiedBy == regexpMatcherType {
		if err := dc.exactMatches.set(newEvent.EventKey(), classification); err != nil {
			return true, fmt.Errorf("failed to set the exact matcher: %w", err)
		}
	}
	return true, nil
}

// DumpCSV dump matches in CSV format to io.Writer
func (dc *DynamicClassifier) DumpCSV(w io.Writer, matcherType string) error {
	matchers := map[string]matcher{
		string(dc.exactMatches.getType()):  dc.exactMatches,
		string(dc.regexpMatches.getType()): dc.regexpMatches,
	}

	matcher, ok := matchers[matcherType]
	if !ok {
		return errors.New("MetadataMatcher '" + matcherType + "' does not exists")
	}

	return matcher.dumpCSV(w)
}

func (dc *DynamicClassifier) classifyByMatch(matcher matcher, event *event.HttpRequest) (*event.SloClassification, error) {
	return matcher.get(event.EventKey())
}

// Run event normalizer receiving events and filling their Key if not already filled.
func (dc *DynamicClassifier) Run() {
	go func() {
		defer func() {
			close(dc.outputChannel)
			dc.done = true
		}()

		for newEvent := range dc.inputChannel {
			start := time.Now()
			classified, err := dc.Classify(newEvent)
			if err != nil {
				dc.logger.Error(err)
				errorsTotal.WithLabelValues("failedToClassify").Inc()
			}
			if !classified {
				dc.logger.Warnf("unable to classify %s", newEvent)
			} else {
				dc.logger.Debugf("processed newEvent with Key: %s", newEvent.EventKey())
			}
			dc.outputChannel <- newEvent
			dc.observeDuration(start)
		}
		dc.logger.Info("input channel closed, finishing")
	}()
}
