package event_key_generator

import (
	"testing"

	"github.com/seznam/slo-exporter/pkg/stringmap"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestEventKeyGenerator_generateEventKey(t *testing.T) {
	testCases := []struct {
		metadata stringmap.StringMap
		config   eventKeyGeneratorConfig
		result   string
	}{
		{metadata: stringmap.StringMap{"foo": "foo"}, config: eventKeyGeneratorConfig{FiledSeparator: ":", MetadataKeys: []string{}}, result: ""},
		{metadata: stringmap.StringMap{"foo": "foo"}, config: eventKeyGeneratorConfig{FiledSeparator: ":", MetadataKeys: []string{"bar"}}, result: ""},
		{metadata: stringmap.StringMap{"foo": "foo"}, config: eventKeyGeneratorConfig{FiledSeparator: ":", MetadataKeys: []string{"foo"}}, result: "foo"},
		{metadata: stringmap.StringMap{"foo": "foo", "bar": "bar"}, config: eventKeyGeneratorConfig{FiledSeparator: ":", MetadataKeys: []string{"foo"}}, result: "foo"},
		{metadata: stringmap.StringMap{"foo": "foo", "bar": "bar"}, config: eventKeyGeneratorConfig{FiledSeparator: ":", MetadataKeys: []string{"foo", "bar"}}, result: "foo:bar"},
		{metadata: stringmap.StringMap{"foo": "foo", "bar": ""}, config: eventKeyGeneratorConfig{FiledSeparator: ":", MetadataKeys: []string{"foo", "bar"}}, result: "foo:"},
		{metadata: stringmap.StringMap{"foo": "foo", "bar": "bar"}, config: eventKeyGeneratorConfig{FiledSeparator: "|", MetadataKeys: []string{"foo", "bar"}}, result: "foo|bar"},
		{metadata: stringmap.StringMap{"foo": "foo", "bar": "bar"}, config: eventKeyGeneratorConfig{FiledSeparator: ":", MetadataKeys: []string{"xxx", "bar"}}, result: "bar"},
	}
	for _, tc := range testCases {
		generator, err := NewFromConfig(tc.config, logrus.New())
		assert.NoError(t, err)
		assert.Equal(t, tc.result, generator.generateEventKey(tc.metadata))
	}
}
