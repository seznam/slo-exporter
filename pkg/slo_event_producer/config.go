//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"fmt"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type sloMatcher struct {
	Domain string `yaml:"domain"`
	Class  string `yaml:"class"`
	App    string `yaml:"app"`
}

type criteriumOptions struct {
	Criterium string `yaml:"criterium"`
	Value     string `yaml:"value"`
}

type ruleOptions struct {
	EventType              string              `yaml:"event_type"`
	SloMatcher             sloMatcher          `yaml:"slo_matcher"`
	FailureCriteriaOptions []criteriumOptions  `yaml:"failure_criteria"`
	AdditionalMetadata     stringmap.StringMap `yaml:"additional_metadata,omitempty"`
}

type rulesConfig struct {
	Rules []ruleOptions `yaml:"rules"`
}

func (rc *rulesConfig) loadFromFile(path string) (*rulesConfig, error) {
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to load configuration file: %w", err)
	}
	err = yaml.UnmarshalStrict(yamlFile, rc)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshall configuration file: %w", err)
	}
	return rc, nil
}
