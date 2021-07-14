package event_metadata_renamer

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"github.com/seznam/slo-exporter/pkg/event"
)

type testCase struct {
	name        string
	inputEvent  *event.Raw
	outputEvent *event.Raw
}

var testCases = []testCase{
	{
		name:        "event with empty metadata",
		inputEvent:  &event.Raw{Metadata: map[string]string{}},
		outputEvent: &event.Raw{Metadata: map[string]string{}},
	},
	{
		name:        "attempt to rename key which is not present in the event's metadata",
		inputEvent:  &event.Raw{Metadata: map[string]string{"sourceX": "bar"}},
		outputEvent: &event.Raw{Metadata: map[string]string{"sourceX": "bar"}},
	},
	{
		name:        "Destination metadata key already exist (collision)",
		inputEvent:  &event.Raw{Metadata: map[string]string{"destination": "destinationCollisionNotOverriden"}},
		outputEvent:  &event.Raw{Metadata: map[string]string{"destination": "destinationCollisionNotOverriden"}},
	},
	{
		name:        "valid rename of metadata key",
		inputEvent:  &event.Raw{Metadata: map[string]string{"source": "bar", "other": "xxx"}},
		outputEvent: &event.Raw{Metadata: map[string]string{"destination": "bar", "other": "xxx"}},
	},
}

func TestRelabel_Run(t *testing.T) {
	configYaml := `
- source: source
  destination: destination
`
	var config []renamerConfig
	err := yaml.UnmarshalStrict([]byte(configYaml), &config)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := NewFromConfig(config, logrus.New())
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.outputEvent, mgr.renameEventMetadata(testCase.inputEvent))
		})
	}
}
