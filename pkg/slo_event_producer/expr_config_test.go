package slo_event_producer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExprConfig_loadFromFile(t *testing.T) {
	ruleGroups := exprRuleGroups{}
	got, err := ruleGroups.loadFromFile("testdata/expr_rules.yaml.golden")

	groupExprProgram, _ := NewExprProgram(`version == "1" && event.slo_domain == "userportal"`)
	sloResultProgam, _ := NewExprProgram(`Int(event.status_code) < 500`)

	want := &exprRuleGroups{
		RuleGroup: []exprRuleGroup{
			{
				GroupExpr: groupExprProgram,
				Rules: []exprRule{
					{
						SloType:       "availability",
						SloResultExpr: sloResultProgam,
					},
				},
			},
		},
	}
	if err != nil {
		t.Fatalf("Error not expected but got one: %q", err)
	}
	assert.Equal(t, got, want)
}
