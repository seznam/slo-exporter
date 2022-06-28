//revive:disable:var-naming
package dynamic_classifier

//revive:enable:var-naming

import (
	"bytes"
	"fmt"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newSloClassification(domain string, app string, class string) *event.SloClassification {
	return &event.SloClassification{
		Domain: domain,
		App:    app,
		Class:  class,
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
		{newMemoryExactMatcher(logger), "test", newSloClassification("test-domain", "test-app", "test-class"), "test", newSloClassification("test-domain", "test-app", "test-class"), "", ""},
		{newMemoryExactMatcher(logger), "", newSloClassification("test-domain", "test-app", "test-class"), "", newSloClassification("test-domain", "test-app", "test-class"), "", ""},
		{newMemoryExactMatcher(logger), "test", newSloClassification("test-domain", "test-app", "test-class"), "aaa", nil, "", ""},
		{newRegexpMatcher(logger), ".*", newSloClassification("test-domain", "test-app", "test-class"), "aaa", newSloClassification("test-domain", "test-app", "test-class"), "", ""},
		{newRegexpMatcher(logger), ".*****", newSloClassification("test-domain", "test-app", "test-class"), "aaa", newSloClassification("test-domain", "test-app", "test-class"), "failed to create new regexp endpoint classification: error parsing regexp: invalid nested repetition operator: `**`", ""},
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
	expectedDataBytes, err := ioutil.ReadFile(expectedDataFilename)
	if err != nil {
		t.Fatal(err)
	}

	var dataBytes []byte
	dataBuffer := bytes.NewBuffer(dataBytes)
	err = matcher.dumpCSV(dataBuffer)
	assert.NoError(t, err)
	assert.EqualValues(t, expectedDataBytes, dataBuffer.Bytes(),
		fmt.Sprintf("expected:\n%s\nactual:\n%s", string(expectedDataBytes), string(dataBuffer.Bytes())))
}

func TestMatcherExactDumpCSV(t *testing.T) {
	matcher := newMemoryExactMatcher(logrus.New())
	matcher.exactMatches["test-endpoint"] = newSloClassification("test-domain", "test-app", "test-class")
	testDumpCSV(t, matcher)
}

func TestMatcherRegexpDumpCSV(t *testing.T) {
	matcher := newRegexpMatcher(logrus.New())
	matcher.matchers = append(matcher.matchers,
		&regexpSloClassification{
			regexpCompiled: regexp.MustCompile(".*"),
			classification: newSloClassification("test-domain", "test-app", "test-class"),
		},
	)
	testDumpCSV(t, matcher)
}
