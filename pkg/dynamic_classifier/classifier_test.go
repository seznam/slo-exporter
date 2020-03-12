//revive:disable:var-naming
package dynamic_classifier

//revive:enable:var-naming

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
)

func newClassifier(t *testing.T, config classifierConfig) *DynamicClassifier {
	classifier, err := New(config)
	if err != nil {
		t.Error(err)
	}
	return classifier
}

func goldenFile(t *testing.T) []string {
	return []string{filepath.Join("testdata", t.Name()+".golden")}
}

func TestLoadExactMatchesFromMultipleCSV(t *testing.T) {
	config := classifierConfig{
		ExactMatchesCsvFiles: goldenFile(t),
	}
	classifier := newClassifier(t, config)

	expectedExactMatches := newMemoryExactMatcher()
	expectedExactMatches.exactMatches["GET:/testing-endpoint"] = newSloClassification("test-domain", "test-app", "test-class")

	if !reflect.DeepEqual(classifier.exactMatches, expectedExactMatches) {
		t.Errorf("Loaded data from csv and expected data does not match: %+v != %+v", classifier.exactMatches, expectedExactMatches)
	}

}

func TestLoadRegexpMatchesFromMultipleCSV(t *testing.T) {
	config := classifierConfig{
		RegexpMatchesCsvFiles: goldenFile(t),
	}
	classifier := newClassifier(t, config)

	expectedRegexpSloClassification := &regexpSloClassification{
		regexpCompiled: regexp.MustCompile(".*"),
		classification: newSloClassification("test-domain", "test-app", "test-class"),
	}
	expectedExactMatches := newRegexpMatcher()
	expectedExactMatches.matchers = append(expectedExactMatches.matchers, expectedRegexpSloClassification)

	if !reflect.DeepEqual(classifier.regexpMatches, expectedExactMatches) {
		t.Errorf("Loaded data from csv and expected data does not match: %+v != %+v", classifier.regexpMatches, expectedExactMatches)
	}

}

func TestClassificationByExactMatches(t *testing.T) {
	config := classifierConfig{
		ExactMatchesCsvFiles: goldenFile(t),
	}
	classifier := newClassifier(t, config)

	data := []struct {
		endpoint               string
		expectedClassification *event.SloClassification
		expectedOk             bool
	}{
		{"GET:/testing-endpoint", newSloClassification("test-domain", "test-app", "test-class"), true},
		{"non-classified-endpoint", nil, false},
	}

	for _, ec := range data {
		event := &event.HttpRequest{
			EventKey:          ec.endpoint,
			SloClassification: ec.expectedClassification,
		}

		ok, err := classifier.Classify(event)
		if err != nil {
			t.Fatalf("Failed to classify %+v - %+v", event, err)
		}

		assert.Equal(t, ec.expectedOk, ok)
		if !reflect.DeepEqual(ec.expectedClassification, event.SloClassification) {
			t.Errorf("Classification does not match %+v != %+v", ec.expectedClassification, event.SloClassification)
		}
	}
}

func TestClassificationByRegexpMatches(t *testing.T) {
	config := classifierConfig{
		RegexpMatchesCsvFiles: goldenFile(t),
	}
	classifier := newClassifier(t, config)

	data := []struct {
		endpoint               string
		expectedClassification *event.SloClassification
		expectedOk             bool
	}{
		{"/api/test/asdf", newSloClassification("test-domain", "test-app", "test-class"), true},
		{"/api/asdf", newSloClassification("test-domain", "test-app", "test-class-all"), true},
		{"non-classified-endpoint", nil, false},
	}

	for _, ec := range data {
		event := &event.HttpRequest{
			EventKey:          ec.endpoint,
			SloClassification: ec.expectedClassification,
		}

		ok, err := classifier.Classify(event)
		if err != nil {
			t.Fatalf("Failed to classify %+v - %+v", event, err)
		}

		assert.Equal(t, ec.expectedOk, ok)
		if !reflect.DeepEqual(ec.expectedClassification, event.SloClassification) {
			t.Errorf("Classification does not match %+v != %+v", ec.expectedClassification, event.SloClassification)
		}
	}
}

func Test_DynamiClassifier_Classify_UpdatesEmptyCache(t *testing.T) {
	eventKey := "GET:/testing-endpoint"
	classifiedEvent := &event.HttpRequest{
		EventKey: eventKey,
		SloClassification: &event.SloClassification{
			Domain: "domain",
			App:    "app",
			Class:  "class",
		},
	}

	// test that classified event updates an empty exact matches cache
	classifier := newClassifier(t, classifierConfig{})
	ok, err := classifier.Classify(classifiedEvent)
	if !ok || err != nil {
		t.Fatalf("unable to classify tested event %+v: %w", classifiedEvent, err)
	}
	classification, err := classifier.exactMatches.get(eventKey)
	if err != nil {
		t.Fatalf("error while getting the tested event key from exact Matches classifier: %w", err)
	}
	if !reflect.DeepEqual(classifiedEvent.SloClassification, classification) {
		t.Errorf("event classification '%+v' did not propagate to classifier exact matches cache: %+v", classifiedEvent.SloClassification, classification)
	}
}

// test that classified event updates dynamic classifier cache as initialized from golden file
func Test_DynamiClassifier_Classify_OverridesCacheFromConfig(t *testing.T) {
	eventKey := "GET:/testing-endpoint"
	classifiedEvent := &event.HttpRequest{
		EventKey: eventKey,
		SloClassification: &event.SloClassification{
			Domain: "domain",
			App:    "app",
			Class:  "class",
		},
	}

	classifier := newClassifier(t, classifierConfig{RegexpMatchesCsvFiles: goldenFile(t)})
	classification, err := classifier.exactMatches.get(eventKey)
	if err != nil {
		t.Fatalf("error while getting the tested event key from exact Matches classifier: %w", err)
	}

	ok, err := classifier.Classify(classifiedEvent)
	if !ok || err != nil {
		t.Fatalf("unable to classify tested event %+v: %w", classifiedEvent, err)
	}
	classification, err = classifier.exactMatches.get(eventKey)
	if err != nil {
		t.Fatalf("error while getting the tested event key from exact Matches classifier: %w", err)
	}
	if !reflect.DeepEqual(classifiedEvent.SloClassification, classification) {
		t.Errorf("classifier cache '%+v' for event_key '%s' was not updated with classification from the classified event '%+v'.", classifiedEvent.SloClassification, eventKey, classification)
	}
}

// test that classified event updates dynamic classifier cache build from previous classified events
func Test_DynamiClassifier_Classify_OverridesCacheFromPreviousClassifiedEvent(t *testing.T) {
	eventKey := "GET:/testing-endpoint"
	eventClasses := []string{"class1", "class2"}

	classifier := newClassifier(t, classifierConfig{})
	for _, eventClass := range eventClasses {
		classifiedEvent := &event.HttpRequest{
			EventKey: eventKey,
			SloClassification: &event.SloClassification{
				Domain: "domain",
				App:    "app",
				Class:  eventClass,
			},
		}

		ok, err := classifier.Classify(classifiedEvent)
		if !ok || err != nil {
			t.Fatalf("unable to classify tested event %+v: %w", classifiedEvent, err)
		}
		classification, err := classifier.exactMatches.get(eventKey)
		if err != nil {
			t.Fatalf("error while getting the tested event key from exact Matches classifier: %w", err)
		}
		if !reflect.DeepEqual(classifiedEvent.SloClassification, classification) {
			t.Errorf("classifier cache '%+v' for event_key '%s' was not updated with classification from the classified event '%+v'.", classifiedEvent.SloClassification, eventKey, classification)
		}
	}
}
