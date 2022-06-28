//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/seznam/slo-exporter/pkg/stringmap"
	"github.com/stretchr/testify/assert"
	"testing"
)

type configTestCase struct {
	name           string
	path           string
	expectedConfig rulesConfig
	expectedError  bool
}

func TestConfig_loadFromFile(t *testing.T) {
	testCases := []configTestCase{
		{
			name: "slo rules file with valid syntax",
			path: "testdata/slo_rules_valid.yaml.golden",
			expectedConfig: rulesConfig{Rules: []ruleOptions{
				{
					SloMatcher: sloMatcher{DomainRegexp: "domain"},
					FailureConditionsOptions: []operatorOptions{
						operatorOptions{
							Operator: "numberIsHigherThan", Key: "statusCode", Value: "500",
						},
					},
					AdditionalMetadata: stringmap.StringMap{"slo_type": "availability"},
				}}},
			expectedError: false,
		},
		{
			name:           "slo_rules file with invalid syntax",
			path:           "testdata/slo_rules_invalid.yaml.golden",
			expectedConfig: rulesConfig{},
			expectedError:  true,
		},
		{
			name:           "invalid path",
			path:           "?????",
			expectedConfig: rulesConfig{},
			expectedError:  true,
		},
	}

	for _, c := range testCases {
		t.Run(
			c.name,
			func(t *testing.T) {
				var config rulesConfig
				var _, err = config.loadFromFile(c.path)
				if c.expectedError {
					assert.Error(t, err)
					return
				}
				assert.Equal(t, c.expectedConfig, config, "failed config test for path %s", c.path)
			},
		)
	}
}
