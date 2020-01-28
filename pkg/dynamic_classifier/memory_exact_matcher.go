package dynamic_classifier

import (
	"encoding/csv"
	"io"

	"github.com/prometheus/client_golang/prometheus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

const exactMatcherType = "exact"

type memoryExactMatcher struct {
	exactMatches map[string]*producer.SloClassification

	matchersCount prometheus.Counter
}

// newMemoryExactMatcher returns instance of memoryCache
func newMemoryExactMatcher() *memoryExactMatcher {
	exactMatches := map[string]*producer.SloClassification{}
	return &memoryExactMatcher{
		exactMatches: exactMatches,
	}
}

// set sets endpoint classification in cache
func (c *memoryExactMatcher) set(key string, classification *producer.SloClassification) error {
	timer := prometheus.NewTimer(matcherOperationDurationSeconds.WithLabelValues("set", exactMatcherType))
	defer timer.ObserveDuration()
	c.exactMatches[key] = classification
	return nil
}

// get gets endpoint classification from cache
func (c *memoryExactMatcher) get(key string) (*producer.SloClassification, error) {
	timer := prometheus.NewTimer(matcherOperationDurationSeconds.WithLabelValues("get", exactMatcherType))
	defer timer.ObserveDuration()
	value := c.exactMatches[key]
	return value, nil
}

func (c *memoryExactMatcher) getType() matcherType {
	return exactMatcherType
}

func (c *memoryExactMatcher) dumpCSV(w io.Writer) {
	buffer := csv.NewWriter(w)
	defer buffer.Flush()
	for k, v := range c.exactMatches {
		err := buffer.Write([]string{v.App, v.Class, k})
		if err != nil {
			errorsTotal.WithLabelValues(err.Error()).Inc()
			log.Error(err)
		}
		buffer.Flush()
	}
}
