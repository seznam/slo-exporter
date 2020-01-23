package dynamicclassifier

import (
	"fmt"
	"regexp"

	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

// regexpSloClassification encapsules combination of regexp and endpoint classification
type regexpSloClassification struct {
	regexpCompiled *regexp.Regexp
	classification *producer.SloClassification
}

// regexpMatcher is list of endpoint classifications
type regexpMatcher []*regexpSloClassification

// newRegexpMatcher returns new instance of regexpMatcher
func newRegexpMatcher() *regexpMatcher {
	return &regexpMatcher{}
}

// newRegexSloClassification returns new instance of regexpSloClassification
func newRegexSloClassification(regexpString string, classification *producer.SloClassification) (*regexpSloClassification, error) {
	regexp, err := regexp.Compile(regexpString)
	if err != nil {
		return nil, fmt.Errorf("Failed to create new regexp endpoint classification: %w", err)
	}

	rec := &regexpSloClassification{
		regexpCompiled: regexp,
		classification: classification,
	}

	return rec, nil
}

// set adds new endpoint classification regexp to list
func (rm *regexpMatcher) set(regexpString string, classification *producer.SloClassification) error {
	regexpClassification, err := newRegexSloClassification(regexpString, classification)
	if err != nil {
		return err
	}
	*rm = append(*rm, regexpClassification)
	log.Tracef("Added regex match for '%s' - %v", regexpClassification.regexpCompiled, regexpClassification.classification)
	return nil
}

// get gets through all regexes and returns first endpoint classification which matches it
func (rm *regexpMatcher) get(key string) (*producer.SloClassification, error) {
	var classification *producer.SloClassification = nil
	for _, r := range *rm {
		// go next if no match
		if !r.regexpCompiled.MatchString(key) {
			continue
		}

		// if already classified, but matches next regex
		if classification != nil {
			log.Warnf("Key '%s' is matched by another regexp: '%s'\n", key, r.regexpCompiled.String())
			continue
		}
		classification = r.classification
		log.Tracef("Key '%s' is matched by regexp: '%s'\n", key, r.regexpCompiled.String())

	}

	return classification, nil
}
