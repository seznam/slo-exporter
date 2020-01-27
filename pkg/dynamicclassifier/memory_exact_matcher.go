package dynamic_classifier

import (
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
