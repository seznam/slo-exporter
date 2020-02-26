//revive:disable:var-naming
package dynamic_classifier

//revive:enable:var-naming

import (
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"io"
	"os"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	log *logrus.Entry

	eventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "slo_exporter",
			Subsystem: "dynamic_classifier",
			Name:      "events_processed_total",
			Help:      "Total number of processed events by result.",
		},
		[]string{"result", "classified_by"},
	)

	errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "slo_exporter",
			Subsystem: "dynamic_classifier",
			Name:      "errors_total",
			Help:      "Total number of processed events by result.",
		},
		[]string{"type"},
	)
)

type classifierConfig struct {
	SloDomain             string
	ExactMatchesCsvFiles  []string
	RegexpMatchesCsvFiles []string
}

// DynamicClassifier is classifier based on cache and regexp matches
type DynamicClassifier struct {
	exactMatches  matcher
	regexpMatches matcher
	sloDomain     string
	observer      prometheus.Observer
}

func NewFromViper(viperConfig *viper.Viper) (*DynamicClassifier, error) {
	var config classifierConfig
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return New(config)
}

// New returns new instance of DynamicClassifier
func New(conf classifierConfig) (*DynamicClassifier, error) {
	classifier := DynamicClassifier{
		exactMatches:  newMemoryExactMatcher(),
		regexpMatches: newRegexpMatcher(),
		sloDomain:     conf.SloDomain,
	}
	if err := classifier.LoadExactMatchesFromMultipleCSV(conf.ExactMatchesCsvFiles); err != nil {
		return nil, fmt.Errorf("failed to load exact matches from CSV: %w", err)
	}
	if err := classifier.LoadRegexpMatchesFromMultipleCSV(conf.RegexpMatchesCsvFiles); err != nil {
		return nil, fmt.Errorf("failed to load regexp matches from CSV: %w", err)
	}
	return &classifier, nil
}

func (dc *DynamicClassifier) SetPrometheusObserver(observer prometheus.Observer) {
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
	defer file.Close()

	csvReader := csv.NewReader(file)

	for {
		line, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		sloApp := line[0]
		sloClass := line[1]
		sloEndpoint := line[2]
		classification := &event.SloClassification{
			Domain: dc.sloDomain,
			App:    sloApp,
			Class:  sloClass,
		}

		err = matcher.set(sloEndpoint, classification)
		if err != nil {
			log.Errorf("failed to load match: %+v", err)
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
		if err := dc.exactMatches.set(newEvent.EventKey, classification); err != nil {
			return true, fmt.Errorf("failed to set the exact matcher: %w", err)
		}
		return true, nil
	}

	classifiers := []matcher{dc.exactMatches, dc.regexpMatches}
	for _, classifier := range classifiers {
		var err error
		classification, err = dc.classifyByMatch(classifier, newEvent)
		if err != nil {
			log.Errorf("error while classifying event: %+v", err)
			classificationErrors = multierror.Append(classificationErrors, err)
		}
		if classification != nil {
			classifiedBy = classifier.getType()
			break
		}
	}

	if classification == nil {
		eventsTotal.WithLabelValues("unclassified", string(classifiedBy)).Inc()
		return false, classificationErrors
	}

	log.Debugf("event '%s' matched by %s matcher", newEvent.EventKey, classifiedBy)
	newEvent.UpdateSLOClassification(classification)
	eventsTotal.WithLabelValues("classified", string(classifiedBy)).Inc()

	// Those matched by regex we want to write to the exact matcher so it is cached
	if classifiedBy == regexpMatcherType {
		if err := dc.exactMatches.set(newEvent.EventKey, classification); err != nil {
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
	return matcher.get(event.EventKey)
}

// Run event normalizer receiving events and filling their Key if not already filled.
func (dc *DynamicClassifier) Run(inputEventsChan <-chan *event.HttpRequest, outputEventsChan chan<- *event.HttpRequest) {
	go func() {
		defer close(outputEventsChan)

		for event := range inputEventsChan {
			start := time.Now()
			classified, err := dc.Classify(event)
			if err != nil {
				log.Error(err)
				errorsTotal.WithLabelValues("failedToClassify").Inc()
			}
			if !classified {
				log.Warnf("unable to classify %s", event.EventKey)
			} else {
				log.Debugf("processed event with Key: %s", event.EventKey)
			}
			outputEventsChan <- event
			dc.observeDuration(start)
		}
		log.Info("input channel closed, finishing")
	}()
}

func init() {
	log = logrus.WithFields(logrus.Fields{"component": "dynamic_classifier"})
	prometheus.MustRegister(eventsTotal, errorsTotal)

}
