package dynamicclassifier

import (
	"context"
	"encoding/csv"
	"io"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

var log *logrus.Entry

var eventsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "slo_exporter",
		Subsystem: "dynamicclassifier",
		Name:      "events_matched_total",
		Help:      "Total number of invalid lines that faild to prse.",
	},
	[]string{"result", "classified_by"},
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

// LoadExactMatchesFromCSV loads exact matches from csv
func (dc *DynamicClassifier) LoadExactMatchesFromCSV(path string) error {
	return dc.loadMatchesFromCSV(dc.exactMatches, path)
}

// LoadRegexpMatchesFromCSV loads exact matches from csv
func (dc *DynamicClassifier) LoadRegexpMatchesFromCSV(path string) error {
	return dc.loadMatchesFromCSV(dc.regexpMatches, path)
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
	// classify against exact match
	classification, err := dc.classifyByMatch(dc.exactMatches, event)
	if err != nil {
		log.Error(err)
	}
	if classification != nil {
		// event is classified by exact match
		log.Tracef("Event '%s' matched against exact match", event.EventKey)
		event.UpdateSLOClassification(classification)
		eventsTotal.WithLabelValues("classified", "exact_match").Inc()
		return true, nil
	}

	log.Tracef("Event '%s' not matched against exact match, trying regexp match", event.EventKey)

	// not classified against exact matches, try regexp match
	classification, err = dc.classifyByMatch(dc.regexpMatches, event)
	if err != nil {
		log.Error(err)
	}

	if classification != nil {
		// event is classified by regexp match
		log.Tracef("Event '%s' matched against regex match", event.EventKey)
		event.UpdateSLOClassification(classification)
		dc.exactMatches.set(event.EventKey, classification)
		eventsTotal.WithLabelValues("classified", "regexp_match").Inc()
		return true, nil
	}

	log.Tracef("Event '%s' not matched", event.EventKey)
	eventsTotal.WithLabelValues("unclassified", "").Inc()

	return false, nil
}

func (dc *DynamicClassifier) classifyByMatch(matcher matcher, event *producer.RequestEvent) (*producer.SloClassification, error) {
	classification, err := matcher.get(event.EventKey)
	if err != nil {
		return nil, err
	}

	return classification, nil
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
				if event.IsClassified() {
					// TODO: maybe insert into exact matches cache for future use?
					log.Debugf("skipping event dynamic classification, already classifier: %v", event.SloClassification)
				} else {
					ok, err := dc.Classify(event)
					if err != nil {
						log.Error(err)
					} else {
						if !ok {
							log.Warnf("Unable to classify %s", event.EventKey)
						} else {
							log.Debugf("processed event with EventKey: %s", event.EventKey)
						}
					}
				}
				outputEventsChan <- event
			}
		}
	}()
}

func init() {
	log = logrus.WithFields(logrus.Fields{"component": "dynamicclassifier"})
	prometheus.MustRegister(eventsTotal)

}
