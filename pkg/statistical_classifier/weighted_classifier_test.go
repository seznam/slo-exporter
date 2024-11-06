package statistical_classifier

import (
	"fmt"
	"testing"
	"time"

	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestArchive(t *testing.T) {
	record := classificationMapping{
		"test-classifications": &classificationWeight{
			classification: &event.SloClassification{
				Domain: "test-domain",
				App:    "test-app",
				Class:  "test-class",
			},
		},
	}

	s, err := newWeightedClassifier(time.Minute, time.Second, logrus.New())
	if err != nil {
		t.Fatal(err)
	}
	s.recentWeights = record

	// test if history is empty
	assert.Equal(t, 0, s.history.Len())

	err = s.archive()
	if err != nil {
		t.Fatal(err)
	}

	// test if history not empty
	assert.NotEqual(t, 0, s.history.Len())

	// test if 'recentWeights' contains empty classificationMapping
	assert.EqualValues(t, classificationMapping{}, s.recentWeights)

	// test if history contains 'archived record
	archivedRecords := make([]interface{}, 0, s.history.Len())
	for i := range s.history.Stream() {
		archivedRecords = append(archivedRecords, i)
	}
	assert.Equal(t, []interface{}{record}, archivedRecords)
}

func TestRefresh(t *testing.T) {
	s, err := newWeightedClassifier(time.Minute, time.Second, logrus.New())
	if err != nil {
		t.Fatal(err)
	}
	classification1 := &event.SloClassification{Class: "1"}
	classification2 := &event.SloClassification{Class: "2"}
	classification3 := &event.SloClassification{Class: "3"}
	expected := weightedClassificationSet{
		enumeratedClassifications: []classificationWeight{
			{weight: 2, classification: classification1},
			{weight: 4, classification: classification2},
			{weight: 6, classification: classification3},
		},
		classificationWeights: []float64{2, 4, 6},
	}
	data := []classificationMapping{
		{
			classification1.String(): &classificationWeight{weight: 1, classification: classification1},
			classification2.String(): &classificationWeight{weight: 2, classification: classification2},
			classification3.String(): &classificationWeight{weight: 3, classification: classification3},
		},
		{
			classification1.String(): &classificationWeight{weight: 1, classification: classification1},
			classification2.String(): &classificationWeight{weight: 2, classification: classification2},
			classification3.String(): &classificationWeight{weight: 3, classification: classification3},
		},
	}

	for _, v := range data {
		s.history.Add(v)
	}

	err = s.reweight()
	if err != nil {
		t.Fatal(err)
	}
	assert.ElementsMatch(t, expected.enumeratedClassifications, s.totalWeightsOverHistory.enumeratedClassifications)
	assert.ElementsMatch(t, expected.classificationWeights, s.totalWeightsOverHistory.classificationWeights)
}

func TestClassForEvent(t *testing.T) {
	expectedClassification := &event.SloClassification{
		Domain: "test-domain",
		App:    "test-app",
		Class:  "test-class",
	}
	s, err := newWeightedClassifier(1, 1, logrus.New())
	if err != nil {
		t.Fatal(err)
	}
	s.totalWeightsOverHistory = &weightedClassificationSet{
		enumeratedClassifications: []classificationWeight{
			{weight: 3, classification: expectedClassification},
			{weight: 0, classification: &event.SloClassification{}},
			{weight: 0, classification: &event.SloClassification{}},
		},
		classificationWeights: []float64{3, 0, 0},
	}

	classification, err := s.guessClass()
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValues(t, expectedClassification, classification)
}

type guessedClass struct {
	class  event.SloClassification
	weight int
	assert func(t assert.TestingT, e1 interface{}, e2 interface{}, msgAndArgs ...interface{}) bool
	value  int
}

type guessTestCase []guessedClass

func TestGuess(t *testing.T) {
	testCases := []guessTestCase{
		[]guessedClass{
			{class: event.SloClassification{Class: "1"}, weight: 50, assert: assert.GreaterOrEqual, value: 20},
			{class: event.SloClassification{Class: "2"}, weight: 50, assert: assert.GreaterOrEqual, value: 20},
		},
		[]guessedClass{
			{class: event.SloClassification{Class: "1"}, weight: 50, assert: assert.GreaterOrEqual, value: 5},
			{class: event.SloClassification{Class: "2"}, weight: 50, assert: assert.GreaterOrEqual, value: 5},
			{class: event.SloClassification{Class: "3"}, weight: 50, assert: assert.GreaterOrEqual, value: 5},
		},
		[]guessedClass{
			{class: event.SloClassification{Class: "1"}, weight: 0, assert: assert.Equal, value: 0},
			{class: event.SloClassification{Class: "2"}, weight: 100, assert: assert.Equal, value: 100},
		},
		[]guessedClass{
			{class: event.SloClassification{Class: "1"}, weight: 100, assert: assert.Equal, value: 100},
			{class: event.SloClassification{Class: "2"}, weight: 0, assert: assert.Equal, value: 0},
		},
		[]guessedClass{
			{class: event.SloClassification{Class: "1"}, weight: 0, assert: assert.Equal, value: 0},
			{class: event.SloClassification{Class: "2"}, weight: 0, assert: assert.Equal, value: 0},
		},
		[]guessedClass{
			{class: event.SloClassification{Class: "1"}, weight: 100, assert: assert.Equal, value: 100},
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("test-case-%d", i), func(t *testing.T) {
			var classificationCount int
			classifier, err := newWeightedClassifier(time.Minute, time.Second, logrus.New())
			if err != nil {
				t.Fatal(err)
			}
			for _, toBeGuessed := range testCase {
				classificationCount += toBeGuessed.weight
				classifier.increaseWeight(toBeGuessed.class, float64(toBeGuessed.weight))
			}
			err = classifier.archive()
			if err != nil {
				t.Fatal(err)
			}

			result := map[event.SloClassification]int{}
			for i := 0; i < classificationCount; i++ {
				guessedClassification, err := classifier.guessClass()
				if err != nil {
					t.Fatal(err)
				}
				result[*guessedClassification]++
			}

			for _, class := range testCase {
				class.assert(t, result[class.class], class.value, "failed for input %+v", testCase)
			}
		})
	}
}

func TestDefaultWeightsGuess(t *testing.T) {
	testedClassification := event.SloClassification{Class: "test", Domain: "test", App: "test"}
	s, err := newWeightedClassifier(time.Minute, time.Second, logrus.NewEntry(logrus.New()))
	if err != nil {
		t.Fatal(err)
	}
	s.setDefaultWeights(newWeightedClassificationSetFromClassifications(&classificationMapping{
		"test": &classificationWeight{
			weight:         1,
			classification: &testedClassification,
		},
	}))
	guessedClassification, err := s.guessClass()
	assert.NoError(t, err)
	assert.Equal(t, testedClassification, *guessedClassification)
}

func TestEmptyGuess(t *testing.T) {
	s, err := newWeightedClassifier(time.Minute, time.Second, logrus.New())
	if err != nil {
		t.Fatal(err)
	}
	s.totalWeightsOverHistory = newWeightedClassificationSet()
	_, err = s.guessClass()
	assert.Error(t, err)
}
