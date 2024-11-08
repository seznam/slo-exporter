package dynamic_classifier

import (
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/sirupsen/logrus"
)

const regexpMatcherType = "regexp"

// regexpSloClassification encapsulates combination of regexp and endpoint classification.
type regexpSloClassification struct {
	regexpCompiled *regexp.Regexp
	classification *event.SloClassification
}

// regexpMatcher is list of endpoint classifications.
type regexpMatcher struct {
	matchers []*regexpSloClassification
	mtx      sync.RWMutex
	logger   logrus.FieldLogger
}

// newRegexpMatcher returns new instance of regexpMatcher.
func newRegexpMatcher(logger logrus.FieldLogger) *regexpMatcher {
	return &regexpMatcher{
		mtx:    sync.RWMutex{},
		logger: logger,
	}
}

// newRegexSloClassification returns new instance of regexpSloClassification.
func newRegexSloClassification(regexpString string, classification *event.SloClassification) (*regexpSloClassification, error) {
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

// set adds new endpoint classification regexp to list.
func (rm *regexpMatcher) set(regexpString string, classification *event.SloClassification) error {
	timer := prometheus.NewTimer(matcherOperationDurationSeconds.WithLabelValues("set", regexpMatcherType))
	defer timer.ObserveDuration()
	rm.mtx.Lock()
	defer rm.mtx.Unlock()

	regexpClassification, err := newRegexSloClassification(regexpString, classification)
	if err != nil {
		return err
	}
	rm.matchers = append(rm.matchers, regexpClassification)
	return nil
}

// get gets through all regexes and returns first endpoint classification which matches it.
func (rm *regexpMatcher) get(key string) (*event.SloClassification, error) {
	timer := prometheus.NewTimer(matcherOperationDurationSeconds.WithLabelValues("get", regexpMatcherType))
	defer timer.ObserveDuration()
	rm.mtx.RLock()
	defer rm.mtx.RUnlock()

	var classification *event.SloClassification
	for _, r := range rm.matchers {
		// go next if no match
		if !r.regexpCompiled.MatchString(key) {
			continue
		}

		// if already classified, but matches next regex
		if classification != nil {
			rm.logger.Warnf("key '%s' is matched by another regexp: '%s'\n", key, r.regexpCompiled.String())
			continue
		}
		classification = r.classification
	}
	return classification, nil
}

func (rm *regexpMatcher) getType() matcherType {
	return regexpMatcherType
}

func (rm *regexpMatcher) dumpCSV(w io.Writer) error {
	rm.mtx.RLock()
	defer rm.mtx.RUnlock()

	buffer := csv.NewWriter(w)
	defer buffer.Flush()
	for _, v := range rm.matchers {
		err := buffer.Write([]string{v.classification.Domain, v.classification.App, v.classification.Class, v.regexpCompiled.String()})
		if err != nil {
			errorsTotal.WithLabelValues("dumpRegexpMatchersToCSV").Inc()
			return fmt.Errorf("failed to dump csv: %w", err)
		}
		buffer.Flush()
	}
	return nil
}
