package dynamic_classifier

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

func newSloClassification(domain string, app string, class string) *producer.SloClassification {
	return &producer.SloClassification{
		Domain: domain,
		App:    app,
		Class:  class,
	}
}

func TestMatcher(t *testing.T) {
	cases := []struct {
		matcher     matcher
		key         string
		value       *producer.SloClassification
		wantedKey   string
		wantedValue *producer.SloClassification
		setErr      string
		getErr      string
	}{
		{newMemoryExactMatcher(), "test", newSloClassification("test-domain", "test-app", "test-class"), "test", newSloClassification("test-domain", "test-app", "test-class"), "", ""},
		{newMemoryExactMatcher(), "", newSloClassification("test-domain", "test-app", "test-class"), "", newSloClassification("test-domain", "test-app", "test-class"), "", ""},
		{newMemoryExactMatcher(), "test", newSloClassification("test-domain", "test-app", "test-class"), "aaa", nil, "", ""},
		{newRegexpMatcher(), ".*", newSloClassification("test-domain", "test-app", "test-class"), "aaa", newSloClassification("test-domain", "test-app", "test-class"), "", ""},
		{newRegexpMatcher(), ".*****", newSloClassification("test-domain", "test-app", "test-class"), "aaa", newSloClassification("test-domain", "test-app", "test-class"), "failed to create new regexp endpoint classification: error parsing regexp: invalid nested repetition operator: `**`", ""},
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
			t.Errorf("Get returned non-expected value %v != %v", value, v.wantedValue)
		}

	}
}

func TestMarcherExactDumpCSV(t *testing.T) {
	expectedDataFilename := filepath.Join("testdata", t.Name()+".golden")
	expectedDataBytes, err := ioutil.ReadFile(expectedDataFilename)
	if err != nil {
		t.Fatal(err)
	}

	var dataBytes []byte
	dataBuffer := bytes.NewBuffer(dataBytes)

	matcher := newMemoryExactMatcher()
	matcher.exactMatches["test-endpoint"] = newSloClassification("test-domain", "test-app", "test-class")
	matcher.dumpCSV(dataBuffer)

	assert.EqualValues(t, expectedDataBytes, dataBuffer.Bytes())
}

func TestMarcherRegexpDumpCSV(t *testing.T) {
	expectedDataFilename := filepath.Join("testdata", t.Name()+".golden")
	expectedDataBytes, err := ioutil.ReadFile(expectedDataFilename)
	if err != nil {
		t.Fatal(err)
	}

	var dataBytes []byte
	dataBuffer := bytes.NewBuffer(dataBytes)

	matcher := newRegexpMatcher()
	matcher.matchers = append(matcher.matchers,
		&regexpSloClassification{
			regexpCompiled: regexp.MustCompile(".*"),
			classification: newSloClassification("test-domain", "test-app", "test-class"),
		},
	)

	matcher.dumpCSV(dataBuffer)

	assert.EqualValues(t, expectedDataBytes, dataBuffer.Bytes())
}
