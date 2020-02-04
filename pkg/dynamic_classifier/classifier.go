package dynamic_classifier

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
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

// DynamicClassifier is classifier based on cache and regexp matches
type DynamicClassifier struct {
	exactMatches  matcher
	regexpMatches matcher
	sloDomain     string
}

// NewDynamicClassifier returns new instance of DynamicClassifier
func NewDynamicClassifier(sloDomain string) *DynamicClassifier {
	return &DynamicClassifier{
		exactMatches:  newMemoryExactMatcher(),
		regexpMatches: newRegexpMatcher(),
		sloDomain:     sloDomain,
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
	var errors error
	for _, p := range paths {
		if err := dc.loadMatchesFromCSV(matcher, p); err != nil {
			errors = multierror.Append(errors, err)
		}
	}
	return errors
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
		classification := &producer.SloClassification{
			Domain: dc.sloDomain,
			App:    sloApp,
			Class:  sloClass,
		}

		err = matcher.set(sloEndpoint, classification)
		if err != nil {
			log.Errorf("failed to load match: %v", err)
		}
	}

	return nil
}

// Classify classifies endpoint by updating its Classification field
func (dc *DynamicClassifier) Classify(event *producer.RequestEvent) (bool, error) {
	var (
		classificationErrors error
		classification       *producer.SloClassification
		classifiedBy         matcherType
	)
	if event.IsClassified() {
		if err := dc.exactMatches.set(event.EventKey, classification); err != nil {
			return true, fmt.Errorf("failed to set the exact matcher: %w", err)
		}
		return true, nil
	}

	classifiers := []matcher{dc.exactMatches, dc.regexpMatches}
	for _, classifier := range classifiers {
		var err error
		classification, err = dc.classifyByMatch(classifier, event)
		if err != nil {
			log.Errorf("error while classifying event: %v", err)
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

	log.Debugf("event '%s' matched by %s matcher", event.EventKey, classifiedBy)
	event.UpdateSLOClassification(classification)
	eventsTotal.WithLabelValues("classified", string(classifiedBy)).Inc()

	// Those matched by regex we want to write to the exact matcher so it is cached
	if classifiedBy == regexpMatcherType {
		if err := dc.exactMatches.set(event.EventKey, classification); err != nil {
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
		return errors.New("Matcher '" + matcherType + "' does not exists")
	}

	return matcher.dumpCSV(w)
}

func (dc *DynamicClassifier) classifyByMatch(matcher matcher, event *producer.RequestEvent) (*producer.SloClassification, error) {
	return matcher.get(event.EventKey)
}

// Run event normalizer receiving events and filling their EventKey if not already filled.
func (dc *DynamicClassifier) Run(ctx context.Context, inputEventsChan <-chan *producer.RequestEvent, outputEventsChan chan<- *producer.RequestEvent) {
	go func() {
		defer close(outputEventsChan)
		defer log.Info("stopping dynamic classifier")

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-inputEventsChan:
				if !ok {
					log.Info("input channel closed, finishing")
					return
				}
				classified, err := dc.Classify(event)
				if err != nil {
					log.Error(err)
					errorsTotal.WithLabelValues(err.Error()).Inc()
				}
				if !classified {
					log.Warnf("unable to classify %s", event.EventKey)
				} else {
					log.Debugf("processed event with EventKey: %s", event.EventKey)
				}
				outputEventsChan <- event
			}
		}
	}()
}

func init() {
	log = logrus.WithFields(logrus.Fields{"component": "dynamic_classifier"})
	prometheus.MustRegister(eventsTotal, errorsTotal)

}
