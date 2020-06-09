package relabel

import (
	"github.com/prometheus/prometheus/pkg/relabel"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gopkg.in/yaml.v2"
	"testing"
)

type testCase struct {
	name        string
	inputEvent  *event.HttpRequest
	outputEvent *event.HttpRequest
}

var testCases = []testCase{
	{
		name:        "relabel event with empty metadata",
		inputEvent:  &event.HttpRequest{Metadata: map[string]string{}},
		outputEvent: &event.HttpRequest{Metadata: map[string]string{}},
	},
	{
		name:        "relabel event with simple metadata that will not be modified",
		inputEvent:  &event.HttpRequest{Metadata: map[string]string{"foo": "bar"}},
		outputEvent: &event.HttpRequest{Metadata: map[string]string{"foo": "bar"}},
	},
	{
		name:        "relabel event which should be dropped",
		inputEvent:  &event.HttpRequest{Metadata: map[string]string{"to_be_dropped": "true"}},
		outputEvent: nil,
	},
	{
		name:        "relabel event where label should be dropped",
		inputEvent:  &event.HttpRequest{Metadata: map[string]string{"foo": "bar", "label_to_be_dropped": "xxx"}},
		outputEvent: &event.HttpRequest{Metadata: map[string]string{"foo": "bar"}},
	},
	{
		name:        "relabel event where get parameter of url is parsed out to new label",
		inputEvent:  &event.HttpRequest{Metadata: map[string]string{"url": "http://foo.bar:8080?operationName=test-operation"}},
		outputEvent: &event.HttpRequest{Metadata: map[string]string{"url": "http://foo.bar:8080?operationName=test-operation", "operation_name": "test-operation"}},
	},
	{
		name:        "relabel event to add all labels with prefix http_ as new labels without the prefix",
		inputEvent:  &event.HttpRequest{Metadata: map[string]string{"http_status": "200", "http_method": "POST"}},
		outputEvent: &event.HttpRequest{Metadata: map[string]string{"http_status": "200", "http_method": "POST", "status": "200", "method": "POST"}},
	},
}

func TestRelabel_Run(t *testing.T) {
	configYaml := `
- source_labels: ["to_be_dropped"]
  regex: "true"
  action: drop
- regex: "label_to_be_dropped"
  action: labeldrop
- source_labels: ["url"]
  regex: ".*operationName=(.*)(&.*)?$"
  target_label: operation_name
  replacement: "$1"
- source_labels: ["url"]
  regex: ".*operationName=(.*)(&.*)?$"
  action: replace
  target_label: operation_name
  replacement: "$1"
- action: labelmap
  regex: "http_(.*)"
  replacement: "$1"  
`
	var config []relabel.Config
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
			assert.Equal(t, testCase.outputEvent, mgr.relabelEvent(testCase.inputEvent))
		})
	}
}
