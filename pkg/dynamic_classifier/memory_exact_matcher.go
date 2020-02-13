//revive:disable:var-naming
package dynamic_classifier

//revive:enable:var-naming

import (
	"encoding/csv"
	"fmt"
	"io"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

const exactMatcherType = "exact"

type memoryExactMatcher struct {
	exactMatches  map[string]*producer.SloClassification
	matchersCount prometheus.Counter
	mtx           sync.RWMutex
}

// newMemoryExactMatcher returns instance of memoryCache
func newMemoryExactMatcher() *memoryExactMatcher {
	exactMatches := map[string]*producer.SloClassification{}
	return &memoryExactMatcher{
		exactMatches: exactMatches,
		mtx:          sync.RWMutex{},
	}
}

// set sets endpoint classification in cache
func (c *memoryExactMatcher) set(key string, classification *producer.SloClassification) error {
	timer := prometheus.NewTimer(matcherOperationDurationSeconds.WithLabelValues("set", exactMatcherType))
	defer timer.ObserveDuration()
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.exactMatches[key] = classification
	return nil
}

// get gets endpoint classification from cache
func (c *memoryExactMatcher) get(key string) (*producer.SloClassification, error) {
	timer := prometheus.NewTimer(matcherOperationDurationSeconds.WithLabelValues("get", exactMatcherType))
	defer timer.ObserveDuration()
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	value := c.exactMatches[key]
	return value, nil
}

func (c *memoryExactMatcher) getType() matcherType {
	return exactMatcherType
}

func (c *memoryExactMatcher) dumpCSV(w io.Writer) error {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	buffer := csv.NewWriter(w)
	defer buffer.Flush()
	for k, v := range c.exactMatches {
		err := buffer.Write([]string{v.App, v.Class, k})
		if err != nil {
			errorsTotal.WithLabelValues("dumpExactMatchersToCSV").Inc()
			return fmt.Errorf("failed to dump csv: %w", err)
		}
		buffer.Flush()
	}
	return nil
}
