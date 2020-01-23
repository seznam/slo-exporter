package dynamicclassifier

import "gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"

type memoryExactMatcher struct {
	exactMatches map[string]*producer.SloClassification
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
	c.exactMatches[key] = classification
	return nil
}

// get gets endpoint classification from cache
func (c *memoryExactMatcher) get(key string) (*producer.SloClassification, error) {
	value := c.exactMatches[key]
	return value, nil
}
