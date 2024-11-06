package dynamic_classifier

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func newTestSloClassification() *event.SloClassification {
	return &event.SloClassification{
		Domain: "test-domain",
		App:    "test-app",
		Class:  "test-class",
	}
}

func TestMatcher(t *testing.T) {
	logger := logrus.New()
	cases := []struct {
		matcher     matcher
		key         string
		value       *event.SloClassification
		wantedKey   string
		wantedValue *event.SloClassification
		setErr      string
		getErr      string
	}{
		{newMemoryExactMatcher(logger), "test", newTestSloClassification(), "test", newTestSloClassification(), "", ""},
		{newMemoryExactMatcher(logger), "", newTestSloClassification(), "", newTestSloClassification(), "", ""},
		{newMemoryExactMatcher(logger), "test", newTestSloClassification(), "aaa", nil, "", ""},
		{newRegexpMatcher(logger), ".*", newTestSloClassification(), "aaa", newTestSloClassification(), "", ""},
		{newRegexpMatcher(logger), ".*****", newTestSloClassification(), "aaa", newTestSloClassification(), "failed to create new regexp endpoint classification: error parsing regexp: invalid nested repetition operator: `**`", ""},
	}

	for _, v := range cases {
		err := v.matcher.set(v.key, v.value)
		if err != nil && v.setErr != "" {
			assert.EqualError(t, err, v.setErr)
			return
		}
		value, err := v.matcher.get(v.wantedKey)
		if err != nil && v.setErr != "" {
			assert.EqualError(t, err, v.getErr)
			return
		}

		if !reflect.DeepEqual(value, v.wantedValue) {
			t.Errorf("Get returned non-expected value %+v != %+v", value, v.wantedValue)
		}

	}
}

func testDumpCSV(t *testing.T, matcher matcher) {
	expectedDataFilename := filepath.Join("testdata", t.Name()+".golden")
	expectedDataBytes, err := os.ReadFile(expectedDataFilename)
	if err != nil {
		t.Fatal(err)
	}

	var dataBytes []byte
	dataBuffer := bytes.NewBuffer(dataBytes)
	err = matcher.dumpCSV(dataBuffer)
	assert.NoError(t, err)
	assert.EqualValues(t, expectedDataBytes, dataBuffer.Bytes(),
		fmt.Sprintf("expected:\n%s\nactual:\n%s", string(expectedDataBytes), dataBuffer.String()))
}

func TestMatcherExactDumpCSV(t *testing.T) {
	matcher := newMemoryExactMatcher(logrus.New())
	matcher.exactMatches["test-endpoint"] = newTestSloClassification()
	testDumpCSV(t, matcher)
}

func TestMatcherRegexpDumpCSV(t *testing.T) {
	matcher := newRegexpMatcher(logrus.New())
	matcher.matchers = append(matcher.matchers,
		&regexpSloClassification{
			regexpCompiled: regexp.MustCompile(".*"),
			classification: newTestSloClassification(),
		},
	)
	testDumpCSV(t, matcher)
}
