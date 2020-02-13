//revive:disable:var-naming
package dynamic_classifier

//revive:enable:var-naming

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
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
		SloDomain:            "test-domain",
		ExactMatchesCsvFiles: goldenFile(t),
	}
	classifier := newClassifier(t, config)

	expectedExactMatches := newMemoryExactMatcher()
	expectedExactMatches.exactMatches["GET:/testing-endpoint"] = newSloClassification("test-domain", "test-app", "test-class")

	if !reflect.DeepEqual(classifier.exactMatches, expectedExactMatches) {
		t.Errorf("Loaded data from csv and expected data does not match: %v != %v", classifier.exactMatches, expectedExactMatches)
	}

}

func TestLoadRegexpMatchesFromMultipleCSV(t *testing.T) {
	config := classifierConfig{
		SloDomain:             "test-domain",
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
		t.Errorf("Loaded data from csv and expected data does not match: %v != %v", classifier.regexpMatches, expectedExactMatches)
	}

}

func TestClassificationByExactMatches(t *testing.T) {
	config := classifierConfig{
		SloDomain:            "test-domain",
		ExactMatchesCsvFiles: goldenFile(t),
	}
	classifier := newClassifier(t, config)

	data := []struct {
		endpoint               string
		expectedClassification *producer.SloClassification
		expectedOk             bool
	}{
		{"GET:/testing-endpoint", newSloClassification("test-domain", "test-app", "test-class"), true},
		{"non-classified-endpoint", nil, false},
	}

	for _, ec := range data {
		event := &producer.RequestEvent{
			EventKey:          ec.endpoint,
			SloClassification: ec.expectedClassification,
		}

		ok, err := classifier.Classify(event)
		if err != nil {
			t.Fatalf("Failed to classify %v - %v", event, err)
		}

		assert.Equal(t, ec.expectedOk, ok)
		if !reflect.DeepEqual(ec.expectedClassification, event.SloClassification) {
			t.Errorf("Classification does not match %v != %v", ec.expectedClassification, event.SloClassification)
		}
	}
}

func TestClassificationByRegexpMatches(t *testing.T) {
	config := classifierConfig{
		SloDomain:             "test-domain",
		RegexpMatchesCsvFiles: goldenFile(t),
	}
	classifier := newClassifier(t, config)

	data := []struct {
		endpoint               string
		expectedClassification *producer.SloClassification
		expectedOk             bool
	}{
		{"/api/test/asdf", newSloClassification("test-domain", "test-app", "test-class"), true},
		{"/api/asdf", newSloClassification("test-domain", "test-app", "test-class-all"), true},
		{"non-classified-endpoint", nil, false},
	}

	for _, ec := range data {
		event := &producer.RequestEvent{
			EventKey:          ec.endpoint,
			SloClassification: ec.expectedClassification,
		}

		ok, err := classifier.Classify(event)
		if err != nil {
			t.Fatalf("Failed to classify %v - %v", event, err)
		}

		assert.Equal(t, ec.expectedOk, ok)
		if !reflect.DeepEqual(ec.expectedClassification, event.SloClassification) {
			t.Errorf("Classification does not match %v != %v", ec.expectedClassification, event.SloClassification)
		}
	}
}
