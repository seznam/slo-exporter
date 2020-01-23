package dynamicclassifier

import (
	"encoding/csv"
	"github.com/sirupsen/logrus"
	"io"
	"os"

	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

var log *logrus.Entry

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
		return true, nil
	}

	log.Tracef("Event '%s' not matched", event.EventKey)
	return false, nil
}

func (dc *DynamicClassifier) classifyByMatch(matcher matcher, event *producer.RequestEvent) (*producer.SloClassification, error) {
	classification, err := matcher.get(event.EventKey)
	if err != nil {
		return nil, err
	}

	return classification, nil
}

func init() {
	log = logrus.WithFields(logrus.Fields{"boxík": "classifier"})
}
