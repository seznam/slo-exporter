package metadata_classifier

import (
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/stringmap"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMetadataClassifier_generateSloClassification(t *testing.T) {
	testCases := []struct {
		name   string
		event  event.Raw
		config metadataClassifierConfig
		result event.SloClassification
	}{
		{
			name:   "non classified event with expected metadata is classified as expected",
			event:  event.NewRaw("", 1, stringmap.StringMap{"domain": "domain", "class": "class", "app": "app"}, &event.SloClassification{Domain: "", Class: "", App: ""}),
			config: metadataClassifierConfig{SloDomainMetadataKey: "domain", SloClassMetadataKey: "class", SloAppMetadataKey: "app", OverrideExistingValues: true},
			result: event.SloClassification{Domain: "domain", Class: "class", App: "app"},
		},
		{
			name:   "with overwrite enabled, metadata classification has precedence over former event classification",
			event:  event.NewRaw("", 1, stringmap.StringMap{"domain": "domain", "class": "class", "app": "app"}, &event.SloClassification{Domain: "xxx", Class: "xxx", App: "xxx"}),
			config: metadataClassifierConfig{SloDomainMetadataKey: "domain", SloClassMetadataKey: "class", SloAppMetadataKey: "app", OverrideExistingValues: true},
			result: event.SloClassification{Domain: "domain", Class: "class", App: "app"},
		},
		{
			name:   "with overwrite disabled, former event classification has precedence over metadata classification",
			event:  event.NewRaw("", 1, stringmap.StringMap{"domain": "domain", "class": "class", "app": "app"}, &event.SloClassification{Domain: "xxx", Class: "xxx", App: "xxx"}),
			config: metadataClassifierConfig{SloDomainMetadataKey: "domain", SloClassMetadataKey: "class", SloAppMetadataKey: "app", OverrideExistingValues: false},
			result: event.SloClassification{Domain: "xxx", Class: "xxx", App: "xxx"},
		},
		{
			name:   "if specified key is not found in metadata, original value of classification is left intact",
			event:  event.NewRaw("", 1, stringmap.StringMap{"domain": "domain", "class": "class"}, &event.SloClassification{Domain: "xxx", Class: "xxx", App: "xxx"}),
			config: metadataClassifierConfig{SloDomainMetadataKey: "domain", SloClassMetadataKey: "class", SloAppMetadataKey: "app", OverrideExistingValues: true},
			result: event.SloClassification{Domain: "domain", Class: "class", App: "xxx"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			generator, err := NewFromConfig(tc.config, logrus.New())
			assert.NoError(t, err)
			assert.Equal(t, tc.result, generator.generateSloClassification(tc.event))
		})
	}
}
