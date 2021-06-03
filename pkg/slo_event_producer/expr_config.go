package slo_event_producer

import (
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"github.com/seznam/slo-exporter/pkg/stringmap"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type SloEnv struct {
	version string
	event   stringmap.StringMap
}

func (se SloEnv) Int(number string) int {
	i, err := strconv.Atoi(number)
	if err != nil {
		i = -1
	}
	return i
}

type ExprEventEvaluator struct {
	ruleGroups []*exprRuleGroups
	logger     logrus.FieldLogger
}

type exprRuleGroups struct {
	RuleGroup []exprRuleGroup `yaml:"rule_groups"`
}

type exprRuleGroup struct {
	GroupExpr exprProgram `yaml:"group_expr"`
	Rules     []exprRule  `yaml:"rules"`
}

// func (rg *exprRuleGroup) Evaluate(newEvent *event.Raw, outChan chan<- *event.Slo) (int, error) {
// 	res, err := GroupExpr.Run(newEvent)
// 	if err != nil {
// 		return 0, err
// 	}
// 	if !res {
// 		return 0, nil
// 	}
//
// 	var errs error
// 	matchedRulesCount := 0
//
// 	for _, rule := range rg.Rules {
// 		newMatchedCount, err := rule.Evaluate(newEvent, outChan)
// 		if err != nil {
// 			errs = multierror.Append(errs, err)
// 		}
// 		matchedRulesCount += newMatchedCount
// 	}
//
// 	return matchedRulesCount, errs
// }

type exprRule struct {
	SloType       string      `yaml:"slo_type"`
	SloResultExpr exprProgram `yaml:"slo_result_expr"`
}

// func (r *exprRule) Evaluate(newEvent *event.Raw, outChan chan<- *event.Slo) (int, error) {
// 	res, err := GroupExpr.Run(newEvent)
// 	if err != nil {
// 		return 0, err
// 	}
// 	if !res {
// 		return 0, nil
// 	}
//
// 	//TODO ...poslat do chan
// 	return 1, nil
// }

type exprProgram struct {
	source  string
	program *vm.Program
}

func NewExprProgram(source string) (exprProgram, error) {
	ep := exprProgram{
		source: source,
	}
	program, err := expr.Compile(ep.source, expr.Env(SloEnv{}))
	if err != nil {
		return ep, err
	}
	ep.program = program
	return ep, nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (ep *exprProgram) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&ep.source); err != nil {
		return err
	}
	program, err := expr.Compile(ep.source, expr.Env(SloEnv{}))
	if err != nil {
		return err
	}
	ep.program = program
	return nil
}

func (rc *exprRuleGroups) loadFromFile(path string) (*exprRuleGroups, error) {
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration file: %w", err)
	}
	err = yaml.UnmarshalStrict(yamlFile, rc)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall configuration file: %w", err)
	}
	return rc, nil
}
