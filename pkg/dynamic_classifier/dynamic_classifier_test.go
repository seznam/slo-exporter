//revive:disable:var-naming
package dynamic_classifier

//revive:enable:var-naming

import (
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/stringmap"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
)

func newClassifier(t *testing.T, config classifierConfig) *DynamicClassifier {
	classifier, err := New(config, logrus.New())
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

	expectedClassification := newSloClassification("test-domain", "test-app", "test-class")
	expectedExactMatches := newMemoryExactMatcher(logrus.New())
	expectedExactMatches.exactMatches["GET:/testing-endpoint"] = expectedClassification

	classification, err := classifier.exactMatches.get("GET:/testing-endpoint")
	assert.NoError(t, err)
	assert.Equal(t, classification, expectedClassification)
}

func TestLoadRegexpMatchesFromMultipleCSV(t *testing.T) {
	config := classifierConfig{
		RegexpMatchesCsvFiles: goldenFile(t),
	}
	classifier := newClassifier(t, config)

	expectedClassification := newSloClassification("test-domain", "test-app", "test-class")
	expectedRegexpSloClassification := &regexpSloClassification{
		regexpCompiled: regexp.MustCompile(".*"),
		classification: expectedClassification,
	}
	expectedExactMatches := newRegexpMatcher(logrus.New())
	expectedExactMatches.matchers = append(expectedExactMatches.matchers, expectedRegexpSloClassification)

	classification, err := classifier.regexpMatches.get("foo")
	assert.NoError(t, err)
	assert.Equal(t, expectedClassification, classification)

}

func TestClassificationByExactMatches(t *testing.T) {
	config := classifierConfig{
		ExactMatchesCsvFiles: goldenFile(t),
	}
	classifier := newClassifier(t, config)

	data := []struct {
		endpoint               string
		expectedClassification event.SloClassification
		expectedOk             bool
	}{
		{"GET:/testing-endpoint", newSloClassification("test-domain", "test-app", "test-class"), true},
		{"non-classified-endpoint", event.SloClassification{}, false},
	}

	for _, ec := range data {
		newEvent := event.NewRaw("", 1, stringmap.StringMap{}, &ec.expectedClassification)
		newEvent.SetEventKey(ec.endpoint)

		ok, err := classifier.Classify(newEvent)
		if err != nil {
			t.Fatalf("Failed to classify %+v - %+v", newEvent, err)
		}

		assert.Equal(t, ec.expectedOk, ok)
		if !reflect.DeepEqual(ec.expectedClassification, newEvent.SloClassification()) {
			t.Errorf("Classification does not match %+v != %+v", ec.expectedClassification, newEvent.SloClassification())
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
		expectedClassification event.SloClassification
		expectedOk             bool
	}{
		{"/api/test/asdf", newSloClassification("test-domain", "test-app", "test-class"), true},
		{"/api/asdf", newSloClassification("test-domain", "test-app", "test-class-all"), true},
		{"non-classified-endpoint", event.SloClassification{}, false},
	}

	for _, ec := range data {
		newEvent := event.NewRaw("", 1, stringmap.StringMap{}, &ec.expectedClassification)
		newEvent.SetEventKey(ec.endpoint)

		ok, err := classifier.Classify(newEvent)
		if err != nil {
			t.Fatalf("Failed to classify %+v - %+v", newEvent, err)
		}

		assert.Equal(t, ec.expectedOk, ok)
		if !reflect.DeepEqual(ec.expectedClassification, newEvent.SloClassification()) {
			t.Errorf("Classification does not match %+v != %+v", ec.expectedClassification, newEvent.SloClassification())
		}
	}
}

func Test_DynamicClassifier_Classify_UpdatesEmptyCache(t *testing.T) {
	eventKey := "GET:/testing-endpoint"
	classifiedEvent := event.NewRaw("", 1, stringmap.StringMap{}, &event.SloClassification{
		Domain: "domain",
		App:    "app",
		Class:  "class",
	})
	classifiedEvent.SetEventKey(eventKey)

	// test that classified event updates an empty exact matches cache
	classifier := newClassifier(t, classifierConfig{})
	ok, err := classifier.Classify(classifiedEvent)
	if !ok || err != nil {
		t.Fatalf("unable to classify tested event %+v: %v", classifiedEvent, err)
	}
	classification, err := classifier.exactMatches.get(eventKey)
	if err != nil {
		t.Fatalf("error while getting the tested event key from exact Matches classifier: %v", err)
	}
	if !reflect.DeepEqual(classifiedEvent.SloClassification(), classification) {
		t.Errorf("event classification '%+v' did not propagate to classifier exact matches cache: %+v", classifiedEvent.SloClassification(), classification)
	}
}

// test that classified event updates dynamic classifier cache as initialized from golden file
func Test_DynamicClassifier_Classify_OverridesCacheFromConfig(t *testing.T) {
	eventKey := "GET:/testing-endpoint"
	classifiedEvent := event.NewRaw("", 1, stringmap.StringMap{}, &event.SloClassification{
		Domain: "domain",
		App:    "app",
		Class:  "class",
	})
	classifiedEvent.SetEventKey(eventKey)

	classifier := newClassifier(t, classifierConfig{RegexpMatchesCsvFiles: goldenFile(t)})
	classification, err := classifier.exactMatches.get(eventKey)
	if err != nil {
		t.Fatalf("error while getting the tested event key from exact Matches classifier: %v", err)
	}

	ok, err := classifier.Classify(classifiedEvent)
	if !ok || err != nil {
		t.Fatalf("unable to classify tested event %+v: %v", classifiedEvent, err)
	}
	classification, err = classifier.exactMatches.get(eventKey)
	if err != nil {
		t.Fatalf("error while getting the tested event key from exact Matches classifier: %v", err)
	}
	if !reflect.DeepEqual(classifiedEvent.SloClassification(), classification) {
		t.Errorf("classifier cache '%+v' for event_key '%s' was not updated with classification from the classified event '%+v'.", classifiedEvent.SloClassification(), eventKey, classification)
	}
}

// test that classified event updates dynamic classifier cache build from previous classified events
func Test_DynamicClassifier_Classify_OverridesCacheFromPreviousClassifiedEvent(t *testing.T) {
	eventKey := "GET:/testing-endpoint"
	eventClasses := []string{"class1", "class2"}

	classifier := newClassifier(t, classifierConfig{})
	for _, eventClass := range eventClasses {
		classifiedEvent := event.NewRaw("", 1, stringmap.StringMap{}, &event.SloClassification{
			Domain: "domain",
			App:    "app",
			Class:  eventClass,
		})
		classifiedEvent.SetEventKey(eventKey)

		ok, err := classifier.Classify(classifiedEvent)
		if !ok || err != nil {
			t.Fatalf("unable to classify tested event %+v: %v", classifiedEvent, err)
		}
		classification, err := classifier.exactMatches.get(eventKey)
		if err != nil {
			t.Fatalf("error while getting the tested event key from exact Matches classifier: %v", err)
		}
		if !reflect.DeepEqual(classifiedEvent.SloClassification(), classification) {
			t.Errorf("classifier cache '%+v' for event_key '%s' was not updated with classification from the classified event '%+v'.", classifiedEvent.SloClassification(), eventKey, classification)
		}
	}
}

func Test_DynamicClassifier_Classify_metadataKeyToLabel(t *testing.T) {
	testCases := map[string]string{
		"foo":    "metadata_foo",
		"fooBar": "metadata_foo_bar",
		"FooBar": "metadata_foo_bar",
		"foobar": "metadata_foobar",
		"":       "metadata_",
	}
	for input, output := range testCases {
		assert.Equal(t, output, metadataKeyToLabel(input))
	}
}
