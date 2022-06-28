package main

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func MustMarshall(input interface{}) string {
	out, err := yaml.Marshal(input)
	if err != nil {
		panic(err)
	}
	return string(out)
}

func TestDomainASRuleGroup(t *testing.T) {
	testTable := []struct {
		name, inputConf string
		expectedOutput  []rulefmt.RuleGroup
	}{
		{
			"Domain without classes",
			`domain-without-classes:
  enabled: false
  namespace: production
  version: 1
  alerting:
    team: team.a@company.org
    escalate: team.sre@company.org`,
			[]rulefmt.RuleGroup{
				{
					Name:     "slo_v1_slo_exporter_domain-without-classes",
					Interval: model.Duration(4 * time.Minute),
					Rules: []rulefmt.RuleNode{
						{
							Record: yamlStr("slo:stable_version"),
							Expr:   yamlStr("1"),
							Labels: Labels{
								"slo_domain":  "domain-without-classes",
								"namespace":   "production",
								"team":        "team.a@company.org",
								"escalate":    "team.sre@company.org",
								"enabled":     "false",
								"slo_version": "1",
							},
						},
					},
				},
			},
		},
		{
			"Domain with single class and type - no burn rate alerting override",
			`test-domain:
  enabled: false
  namespace: production
  version: 1
  alerting:
    team: team.a@company.org
    escalate: team.sre@company.org
  classes:
    critical:
      availability: {slo_threshold: 0.99}`,
			[]rulefmt.RuleGroup{
				{
					Name:     "slo_v1_slo_exporter_test-domain",
					Interval: model.Duration(4 * time.Minute),
					Rules: []rulefmt.RuleNode{
						{
							Record: yamlStr("slo:stable_version"),
							Expr:   yamlStr("1"),
							Labels: Labels{
								"slo_domain":  "test-domain",
								"namespace":   "production",
								"team":        "team.a@company.org",
								"escalate":    "team.sre@company.org",
								"enabled":     "false",
								"slo_version": "1",
							},
						},
					},
				},
				{
					Name:     "slo_v1_slo_exporter_test-domain_critical",
					Interval: model.Duration(4 * time.Minute),
					Rules: []rulefmt.RuleNode{
						{
							Record: yamlStr("slo:burn_rate_threshold"),
							Expr:   yamlStr("13.44"),
							Labels: Labels{
								"slo_domain":     "test-domain",
								"slo_class":      "critical",
								"namespace":      "production",
								"slo_version":    "1",
								"slo_type":       "availability",
								"slo_time_range": "1h",
							},
						},
						{
							Record: yamlStr("slo:burn_rate_threshold"),
							Expr:   yamlStr("5.6"),
							Labels: Labels{
								"slo_domain":     "test-domain",
								"slo_class":      "critical",
								"namespace":      "production",
								"slo_version":    "1",
								"slo_type":       "availability",
								"slo_time_range": "6h",
							},
						},
						{
							Record: yamlStr("slo:burn_rate_threshold"),
							Expr:   yamlStr("2.8"),
							Labels: Labels{
								"slo_domain":     "test-domain",
								"slo_class":      "critical",
								"namespace":      "production",
								"slo_version":    "1",
								"slo_type":       "availability",
								"slo_time_range": "1d",
							},
						},
						{
							Record: yamlStr("slo:burn_rate_threshold"),
							Expr:   yamlStr("1"),
							Labels: Labels{
								"slo_domain":     "test-domain",
								"slo_class":      "critical",
								"namespace":      "production",
								"slo_version":    "1",
								"slo_type":       "availability",
								"slo_time_range": "3d",
							},
						},
						{
							Record: yamlStr("slo:violation_ratio_threshold"),
							Expr:   yamlStr("0.99"),
							Labels: Labels{
								"slo_domain":  "test-domain",
								"slo_class":   "critical",
								"namespace":   "production",
								"slo_version": "1",
								"slo_type":    "availability",
							},
						},
					},
				},
			},
		},
		{
			"Domain with single class and type - burn rate alerting override",
			`test-domain:
  enabled: false
  namespace: production
  version: 1
  alerting:
    team: team.a@company.org
    escalate: team.sre@company.org
    burn_rate_thresholds:
      - condition: {class: 'critical',      slo_type: 'availability',    time_range: '3d'}
        value: 100
  classes:
    critical:
      availability: {slo_threshold: 0.99}`,
			[]rulefmt.RuleGroup{
				{
					Name:     "slo_v1_slo_exporter_test-domain",
					Interval: model.Duration(4 * time.Minute),
					Rules: []rulefmt.RuleNode{
						{
							Record: yamlStr("slo:stable_version"),
							Expr:   yamlStr("1"),
							Labels: Labels{
								"slo_domain":  "test-domain",
								"namespace":   "production",
								"team":        "team.a@company.org",
								"escalate":    "team.sre@company.org",
								"enabled":     "false",
								"slo_version": "1",
							},
						},
					},
				},
				{
					Name:     "slo_v1_slo_exporter_test-domain_critical",
					Interval: model.Duration(4 * time.Minute),
					Rules: []rulefmt.RuleNode{
						{
							Record: yamlStr("slo:burn_rate_threshold"),
							Expr:   yamlStr("100"),
							Labels: Labels{
								"slo_domain":     "test-domain",
								"slo_class":      "critical",
								"namespace":      "production",
								"slo_version":    "1",
								"slo_type":       "availability",
								"slo_time_range": "3d",
							},
						},
						{
							Record: yamlStr("slo:violation_ratio_threshold"),
							Expr:   yamlStr("0.99"),
							Labels: Labels{
								"slo_domain":  "test-domain",
								"slo_class":   "critical",
								"namespace":   "production",
								"slo_version": "1",
								"slo_type":    "availability",
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testTable {
		t.Run(testCase.name, func(t *testing.T) {
			data := SloConfiguration{}
			err := yaml.Unmarshal([]byte(testCase.inputConf), &data)
			if err != nil {
				t.Errorf("Unable to unmarshal input data: %v", err)
			}
			outputRuleGroups := []rulefmt.RuleGroup{}
			for domainName, domainConfig := range data {
				if errs := domainConfig.IsValid(); len(errs) > 0 {
					t.Error(errs)
				}
				outputRuleGroups = append(outputRuleGroups, domainConfig.AsRuleGroups(domainName)...)
			}
			if err != nil {
				t.Error(err)
			}
			assert.Equal(t, MustMarshall(testCase.expectedOutput), MustMarshall(outputRuleGroups))
		})
	}
}
