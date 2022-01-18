package main

import (
	"fmt"
	"sort"

	"github.com/prometheus/prometheus/pkg/rulefmt"
)

const (
	classLabel = "slo_class"
	sloTypeLabel = "slo_type"
)

type Classes map[string]Class // Key is SloClass name


// Returns a sorted list of classes names
func (c Classes) Names() []string {
	res := []string{}
	for name,_ := range c {
		res = append(res, name)
	}
	sort.Strings(res)
	return res
}

type Class map[string]SloType // Key is SloType

// Returns a sorted list of class' SLO types names
func (c Class) Names() []string {
	res := []string{}
	for name,_ := range c {
		res = append(res, name)
	}
	sort.Strings(res)
	return res
}

func (c Class) IsValid() []error {
	errs := []error{}
	for sloTypeName, Threshold := range c {
		if err := Threshold.IsValid(); err != nil {
			errs = append(errs,fmt.Errorf("error validating '%s': %w", sloTypeName, err))
		}
	}
	return errs
}

// Returns SLO class representation as a list of Prometheus rules
// If provided burnRateThresholds are nil, defaultTimerangeThresholds are used
func (c Class) AsRules(className string, commonLabels Labels, burnRateThresholds []BurnRateThreshold) []rulefmt.RuleNode {
	rules := []rulefmt.RuleNode{}
	if burnRateThresholds == nil {
		burnRateThresholds = defaultTimerangeThresholds
	}
	commonLabels = commonLabels.Merge(Labels{classLabel: className})
	for _, sloTypeName := range c.Names() {
		burnRateThresholdsForType := getMatchingSubset(burnRateThresholds, className, sloTypeName)
		rules = append(rules,
			c[sloTypeName].AsRules(sloTypeName, commonLabels, burnRateThresholdsForType)...
		)

	}
	return rules
}

type SloType struct {
	Value float32 `yaml:"slo_threshold"`
	Metadata Labels `yaml:"slo_threshold_metadata"`
}

func (t SloType) IsValid() error {
	if t.Value < 0 || t.Value > 1 {
		return fmt.Errorf("slo threshold must be 0-1, not: %f", t.Value)
	}
	return nil
}

func (t SloType) AsRules(sloTypeName string, commonLabels Labels, burnRateThresholds []BurnRateThreshold) []rulefmt.RuleNode {
	rules := []rulefmt.RuleNode{}
	commonLabels = commonLabels.Merge(Labels{sloTypeLabel: sloTypeName})

	rules = append(rules,
		t.burnRateThresholdRules(commonLabels, burnRateThresholds)...
	)
	rules = append(rules, t.violationRatioThresholdRule(commonLabels))
	return rules
}

func (t SloType) violationRatioThresholdRule(commonLabels Labels) rulefmt.RuleNode {
	return rulefmt.RuleNode{
		Record:      yamlStr("slo:violation_ratio_threshold"),
		Expr:        yamlStr(fmt.Sprint(t.Value)),
		Labels:      commonLabels.Merge(t.Metadata),
	}
}

func (t SloType) burnRateThresholdRules(commonLabels Labels, burnRateThresholds []BurnRateThreshold) []rulefmt.RuleNode {
	rules := []rulefmt.RuleNode{}
	for _, burnRateThreshold := range burnRateThresholds {
		rules = append(rules,
			rulefmt.RuleNode{
				Record:      yamlStr("slo:burn_rate_threshold"),
				Expr:        yamlStr(fmt.Sprint(burnRateThreshold.Value)),
				Labels:      commonLabels.Merge(Labels{"slo_time_range": string(burnRateThreshold.Condition.TimeRange)}),
			})
	}
	return rules
}


