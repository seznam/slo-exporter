//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"testing"
)

type configTestCase struct {
	path           string
	expectedConfig rulesConfig
	expectedError  bool
}

func TestConfig_loadFromFile(t *testing.T) {
	testCases := []configTestCase{
		{
			path: "testdata/slo_rules_valid.yaml.golden",
			expectedConfig: rulesConfig{Rules: []ruleOptions{
				{
					EventType:       "request",
					SloMatcher: sloMatcher{Domain:"domain"},
					FailureCriteriaOptions: []criteriumOptions{
						{Criterium: "requestStatusHigherThan", Value: "500"},
					},
					AdditionalMetadata: stringmap.StringMap{"slo_type": "availability"},
				}}},
			expectedError: false,
		},
		{
			path:           "testdata/slo_rules_invalid.yaml.golden",
			expectedConfig: rulesConfig{},
			expectedError:  true,
		},
		{
			path:           "?????",
			expectedConfig: rulesConfig{},
			expectedError:  true,
		},
	}

	for _, c := range testCases {
		var config rulesConfig
		_, err := config.loadFromFile(c.path)
		if c.expectedError {
			assert.Error(t, err)
			continue
		}
		assert.Equal(t, config, c.expectedConfig)
	}
}
