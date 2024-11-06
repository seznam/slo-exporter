package slo_event_producer

import (
	"fmt"
	"os"

	"github.com/seznam/slo-exporter/pkg/stringmap"
	"gopkg.in/yaml.v2"
)

type sloMatcher struct {
	DomainRegexp string `yaml:"domain"`
	ClassRegexp  string `yaml:"class"`
	AppRegexp    string `yaml:"app"`
}

type operatorOptions struct {
	Operator string `yaml:"operator"`
	Key      string `yaml:"key"`
	Value    string `yaml:"value"`
}

type ruleOptions struct {
	MetadataMatcherConditionsOptions []operatorOptions   `yaml:"metadata_matcher"`
	SloMatcher                       sloMatcher          `yaml:"slo_matcher"`
	FailureConditionsOptions         []operatorOptions   `yaml:"failure_conditions"`
	AdditionalMetadata               stringmap.StringMap `yaml:"additional_metadata,omitempty"`
}

type rulesConfig struct {
	Rules []ruleOptions `yaml:"rules"`
}

func (rc *rulesConfig) loadFromFile(path string) error {
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to load configuration file: %w", err)
	}
	err = yaml.UnmarshalStrict(yamlFile, rc)
	if err != nil {
		return fmt.Errorf("failed to unmarshall configuration file: %w", err)
	}
	return nil
}
