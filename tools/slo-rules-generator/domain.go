package main

import (
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"
)

const (
	domainLabel = "slo_domain"
	namespaceLabel = "namespace"
	versionLabel = "slo_version"
	enabledLabel = "enabled"
	teamLabel = "team"
	escalateLabel = "escalate"
)

type Domain struct {
	Namespace string
	Enabled   bool
	Version   int
	Alerting Alerting
	Classes   Classes
}

func (d Domain) AsRuleGroups(domainName string) []rulefmt.RuleGroup{
	domainRulegroup := rulefmt.RuleGroup{
		Name:     fmt.Sprintf("slo_v%d_slo_exporter_%s", d.Version, domainName),
		Interval: model.Duration(4 * time.Minute),
		Rules:    []rulefmt.RuleNode{},
	}
	domainRulegroup.Rules = append(domainRulegroup.Rules, d.stableVersionRule(domainName))
	out := []rulefmt.RuleGroup{
		domainRulegroup,
	}

	for _, className := range d.Classes.Names() {
		domainClassRulegroup := rulefmt.RuleGroup{
			Name:     fmt.Sprintf("slo_v%d_slo_exporter_%s_%s", d.Version, domainName, className),
			Interval: model.Duration(4 * time.Minute),
			Rules:    []rulefmt.RuleNode{},
		}
		domainClassRulegroup.Rules = append(
			domainClassRulegroup.Rules,
			d.Classes[className].AsRules(className, d.commonLabels(domainName), d.Alerting.BurnRateThresholds)...
		)
		out = append(out, domainClassRulegroup)
	}



	return out
}

func (d Domain) commonLabels(domainName string) Labels {
	return Labels{
		domainLabel: domainName,
		versionLabel: fmt.Sprint(d.Version),
		namespaceLabel: fmt.Sprint(d.Namespace),
	}
}

func (d Domain) stableVersionRule(domainName string) rulefmt.RuleNode {
	return rulefmt.RuleNode{
		Record:      yamlStr("slo:stable_version"),
		Expr:        yamlStr("1"),
		Labels: d.commonLabels(domainName).Merge(Labels{
			teamLabel: d.Alerting.Team,
			escalateLabel: d.Alerting.Escalate,
			enabledLabel: fmt.Sprint(d.Enabled),
		}),
	}
}

func (d Domain) IsValid() []error {
	errs := []error{}
	if err := d.Alerting.IsValid(); len(err) > 0 {
		errs = append(errs, fmt.Errorf("alerting validation failed: %v", err))
	}
	for className, classConf := range d.Classes {
		if err := classConf.IsValid(); len(err) > 0 {
			errs = append(errs, fmt.Errorf("class '%s' validation failed: %v",className, err))
		}
	}
	return append(errs, d.validateReferences()...)
}

// Validates whether classes and slo_types references in alerting..conditions are defined in classes section
func (d Domain) validateReferences() []error {
	errs := []error{}
	for _, threshold := range d.Alerting.BurnRateThresholds {
		class := threshold.Condition.Class
		if class != "" {
			if _, ok := d.Classes[class]; !ok {
				errs = append(errs, fmt.Errorf("class '%s' referenced in condition not defined", class))
			}
		}
		if sloType := threshold.Condition.Type; sloType != "" {
			if class != "" {
				if _, typeFound := d.Classes[class][sloType]; !typeFound {
					errs = append(errs, fmt.Errorf("slo type '%s' referenced in condition not defined for class '%s'", sloType, class))
				}
			} else {
				sloTypeFound := false
				for _, class := range d.Classes {
					if _, ok := class[sloType]; ok {
						sloTypeFound = true
						break
					}
				}
				if !sloTypeFound {
					errs = append(errs, fmt.Errorf("slo type '%s' referenced in condition not defined in any class", sloType))
				}
			}
		}
	}
	return errs
}
