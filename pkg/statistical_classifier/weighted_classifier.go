//revive:disable:var-naming
package statistical_classifier

//revive:enable:var-naming

import (
	"fmt"
	"github.com/seznam/slo-exporter/pkg/event"
	"sort"
)

func newClassificationWeights() classificationWeights {
	return classificationWeights{weights: make(map[string]classificationWeight)}
}

type classificationWeights struct {
	weights map[string]classificationWeight
}

func (c *classificationWeights) len() int {
	return len(c.weights)
}

func (c *classificationWeights) listClassificationWeights() []classificationWeight {
	var weights []classificationWeight
	for _, v := range c.weights {
		weights = append(weights, v)
	}
	return weights
}

func (c *classificationWeights) sortedKeys() []string {
	keys := make([]string, len(c.weights))
	i := 0
	for k, _ := range c.weights {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

func (c *classificationWeights) sortedWeights() []float64 {
	weights := make([]float64, len(c.weights))
	i := 0
	for _, v := range c.sortedKeys() {
		weights[i] = c.weights[v].weight
		i++
	}
	return weights
}

func (c *classificationWeights) index(index int) (classificationWeight, error) {
	keys := c.sortedKeys()
	if index > len(keys)-1 {
		return classificationWeight{}, fmt.Errorf("index %d out of range %d", index, len(c.sortedKeys()))
	}
	return c.weights[keys[index]], nil
}

func (c *classificationWeights) add(classification event.SloClassification, value float64) {
	c.weights[classification.String()] = classificationWeight{
		classification: classification,
		weight:         value,
	}
}

func (c *classificationWeights) inc(classification event.SloClassification, value float64) {
	weight, ok := c.weights[classification.String()]
	if !ok {
		c.add(classification, value)
		return
	}
	weight.inc(value)
}

func (c *classificationWeights) merge(other classificationWeights) {
	for _, otherWeight := range other.listClassificationWeights() {
		c.inc(otherWeight.classification, otherWeight.weight)
	}
}

type classificationWeight struct {
	classification event.SloClassification
	weight         float64
}

func (c *classificationWeight) inc(value float64) {
	c.weight += value
}
