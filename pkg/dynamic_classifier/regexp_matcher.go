package dynamic_classifier

import (
	"encoding/csv"
	"fmt"
	"io"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"

	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

const regexpMatcherType = "regexp"

// regexpSloClassification encapsulates combination of regexp and endpoint classification
type regexpSloClassification struct {
	regexpCompiled *regexp.Regexp
	classification *producer.SloClassification
}

// regexpMatcher is list of endpoint classifications
type regexpMatcher struct {
	matchers []*regexpSloClassification
}

// newRegexpMatcher returns new instance of regexpMatcher
func newRegexpMatcher() *regexpMatcher {
	return &regexpMatcher{}
}

// newRegexSloClassification returns new instance of regexpSloClassification
func newRegexSloClassification(regexpString string, classification *producer.SloClassification) (*regexpSloClassification, error) {
	compiledMatcher, err := regexp.Compile(regexpString)
	if err != nil {
		return nil, fmt.Errorf("failed to create new regexp endpoint classification: %w", err)
	}
	rec := &regexpSloClassification{
		regexpCompiled: compiledMatcher,
		classification: classification,
	}
	return rec, nil
}

// set adds new endpoint classification regexp to list
func (rm *regexpMatcher) set(regexpString string, classification *producer.SloClassification) error {
	timer := prometheus.NewTimer(matcherOperationDurationSeconds.WithLabelValues("set", regexpMatcherType))
	defer timer.ObserveDuration()
	regexpClassification, err := newRegexSloClassification(regexpString, classification)
	if err != nil {
		return err
	}
	rm.matchers = append(rm.matchers, regexpClassification)
	log.Tracef("added regex match for '%s' - %v", regexpClassification.regexpCompiled, regexpClassification.classification)
	return nil
}

// get gets through all regexes and returns first endpoint classification which matches it
func (rm *regexpMatcher) get(key string) (*producer.SloClassification, error) {
	timer := prometheus.NewTimer(matcherOperationDurationSeconds.WithLabelValues("get", regexpMatcherType))
	defer timer.ObserveDuration()
	var classification *producer.SloClassification = nil
	for _, r := range rm.matchers {
		// go next if no match
		if !r.regexpCompiled.MatchString(key) {
			continue
		}

		// if already classified, but matches next regex
		if classification != nil {
			log.Warnf("key '%s' is matched by another regexp: '%s'\n", key, r.regexpCompiled.String())
			continue
		}
		classification = r.classification
		log.Tracef("key '%s' is matched by regexp: '%s'\n", key, r.regexpCompiled.String())

	}
	return classification, nil
}

func (rm *regexpMatcher) getType() matcherType {
	return regexpMatcherType
}

func (rm *regexpMatcher) dumpCSV(w io.Writer) error {
	buffer := csv.NewWriter(w)
	defer buffer.Flush()
	for _, v := range rm.matchers {
		err := buffer.Write([]string{v.classification.App, v.classification.Class, v.regexpCompiled.String()})
		if err != nil {
			errorsTotal.WithLabelValues(err.Error()).Inc()
			return fmt.Errorf("Failed to dump csv: %w", err)
		}
		buffer.Flush()
	}
	return nil
}
