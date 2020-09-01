package statistical_classifier

import (
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_classificationWeights_add(t *testing.T) {
	tests := []struct {
		name            string
		increments      []classificationWeight
		expectedWeights []classificationWeight
	}{
		{
			name:            "empty weights and no added",
			increments:      []classificationWeight{},
			expectedWeights: []classificationWeight{},
		},
		{
			name: "add only one zero",
			increments: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 0},
			},
			expectedWeights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 0},
			},
		},
		{
			name: "add two distinct",
			increments: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 1},
				{classification: event.SloClassification{Domain: "bar", App: "bar", Class: "bar"}, weight: 2},
			},
			expectedWeights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 1},
				{classification: event.SloClassification{Domain: "bar", App: "bar", Class: "bar"}, weight: 2},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClassificationWeights()
			for _, item := range tt.increments {
				c.add(item.classification, item.weight)
			}
			assert.ElementsMatch(t, tt.expectedWeights, c.listClassificationWeights())
		})
	}
}

func Test_classificationWeights_inc(t *testing.T) {
	tests := []struct {
		name            string
		increments      []classificationWeight
		expectedWeights []classificationWeight
	}{
		{
			name:            "empty weights and no added",
			increments:      []classificationWeight{},
			expectedWeights: []classificationWeight{},
		},
		{
			name: "add only one zero",
			increments: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 0},
			},
			expectedWeights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 0},
			},
		},
		{
			name: "add new one",
			increments: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 1},
			},
			expectedWeights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 1},
			},
		},
		{
			name: "add two distinct",
			increments: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 1},
				{classification: event.SloClassification{Domain: "bar", App: "bar", Class: "bar"}, weight: 2},
			},
			expectedWeights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 1},
				{classification: event.SloClassification{Domain: "bar", App: "bar", Class: "bar"}, weight: 2},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClassificationWeights()
			for _, item := range tt.increments {
				c.inc(item.classification, item.weight)
			}
			assert.ElementsMatch(t, tt.expectedWeights, c.listClassificationWeights())
		})
	}
}

func Test_classificationWeights_index(t *testing.T) {
	tests := []struct {
		name           string
		weights        []classificationWeight
		index          int
		expectedWeight classificationWeight
		expectError    bool
	}{
		{
			name:        "error on empty",
			weights:     []classificationWeight{},
			index:       0,
			expectError: true,
		},
		{
			name: "error on index out of range",
			weights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 0},
			},
			index:       10,
			expectError: true,
		},
		{
			name: "verify alphabetical order",
			weights: []classificationWeight{
				{classification: event.SloClassification{Domain: "b", App: "b", Class: "b"}, weight: 0},
				{classification: event.SloClassification{Domain: "a", App: "a", Class: "a"}, weight: 0},
			},
			index:          0,
			expectedWeight: classificationWeight{classification: event.SloClassification{Domain: "a", App: "a", Class: "a"}, weight: 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClassificationWeights()
			for _, item := range tt.weights {
				c.inc(item.classification, item.weight)
			}
			weight, err := c.index(tt.index)
			if err != nil && !tt.expectError {
				t.Fatalf("resulted in unexpected error: %v", err)
			}
			assert.Equal(t, tt.expectedWeight, weight)
		})
	}
}

func Test_classificationWeights_len(t *testing.T) {
	tests := []struct {
		name        string
		weights     []classificationWeight
		expectedLen int
	}{
		{
			name:        "empty has zero len",
			weights:     []classificationWeight{},
			expectedLen: 0,
		},
		{
			name: "len of 1",
			weights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 0},
			},
			expectedLen: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClassificationWeights()
			for _, item := range tt.weights {
				c.inc(item.classification, item.weight)
			}
			assert.Equal(t, tt.expectedLen, c.len())
		})
	}
}

func Test_classificationWeights_listClassifictionWeights(t *testing.T) {
	tests := []struct {
		name            string
		addedWeights    []classificationWeight
		expectedWeights []classificationWeight
	}{
		{
			name:            "test empty",
			addedWeights:    []classificationWeight{},
			expectedWeights: []classificationWeight{},
		},
		{
			name: "matches one added",
			addedWeights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 0},
			},
			expectedWeights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 0},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClassificationWeights()
			for _, item := range tt.addedWeights {
				c.inc(item.classification, item.weight)
			}
			assert.ElementsMatch(t, tt.expectedWeights, c.listClassificationWeights())
		})
	}
}

func Test_classificationWeights_merge(t *testing.T) {
	tests := []struct {
		name            string
		originalWeights []classificationWeight
		otherWeights    []classificationWeight
		expectedWeights []classificationWeight
	}{
		{
			name:            "merge empty and ampty",
			originalWeights: []classificationWeight{},
			otherWeights:    []classificationWeight{},
			expectedWeights: []classificationWeight{},
		},
		{
			name: "merge full and empty",
			originalWeights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 1},
			},
			otherWeights: []classificationWeight{},
			expectedWeights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 1},
			},
		},
		{
			name:            "merge empty and full",
			originalWeights: []classificationWeight{},
			otherWeights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 1},
			},
			expectedWeights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 1},
			},
		},
		{
			name: "merge both full with distinct classifications",
			originalWeights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 1},
			},
			otherWeights: []classificationWeight{
				{classification: event.SloClassification{Domain: "bar", App: "bar", Class: "bar"}, weight: 2},
			},
			expectedWeights: []classificationWeight{
				{classification: event.SloClassification{Domain: "foo", App: "foo", Class: "foo"}, weight: 1},
				{classification: event.SloClassification{Domain: "bar", App: "bar", Class: "bar"}, weight: 2},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := newClassificationWeights()
			for _, item := range tt.originalWeights {
				original.inc(item.classification, item.weight)
			}
			other := newClassificationWeights()
			for _, item := range tt.otherWeights {
				other.inc(item.classification, item.weight)
			}
			original.merge(other)
			assert.ElementsMatch(t, tt.expectedWeights, original.listClassificationWeights())
		})
	}
}

func Test_classificationWeights_sortedKeys(t *testing.T) {
	classificationA := event.SloClassification{Domain: "a", App: "a", Class: "a"}
	classificationB := event.SloClassification{Domain: "b", App: "b", Class: "b"}
	tests := []struct {
		name         string
		addedWeights []classificationWeight
		expectedKeys []string
	}{
		{
			name:         "test empty",
			addedWeights: []classificationWeight{},
			expectedKeys: []string{},
		},
		{
			name: "test sorted",
			addedWeights: []classificationWeight{
				{classification: classificationB, weight: 0},
				{classification: classificationA, weight: 0},
			},
			expectedKeys: []string{
				classificationA.String(),
				classificationB.String(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClassificationWeights()
			for _, item := range tt.addedWeights {
				c.inc(item.classification, item.weight)
			}
			assert.Equal(t, tt.expectedKeys, c.sortedKeys())
		})
	}
}

func Test_classificationWeights_sortedWeights(t *testing.T) {
	classificationWeightA := classificationWeight{classification: event.SloClassification{Domain: "a", App: "a", Class: "a"}, weight: 8}
	classificationWeightB := classificationWeight{classification: event.SloClassification{Domain: "b", App: "b", Class: "b"}, weight: 3}
	tests := []struct {
		name            string
		addedWeights    []classificationWeight
		expectedWeights []float64
	}{
		{
			name:            "test empty",
			addedWeights:    []classificationWeight{},
			expectedWeights: []float64{},
		},
		{
			name: "test sorted",
			addedWeights: []classificationWeight{
				classificationWeightB,
				classificationWeightA,
			},
			expectedWeights: []float64{
				classificationWeightA.weight,
				classificationWeightB.weight,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClassificationWeights()
			for _, item := range tt.addedWeights {
				c.inc(item.classification, item.weight)
			}
			assert.Equal(t, tt.expectedWeights, c.sortedWeights())
		})
	}
}
