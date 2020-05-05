//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"fmt"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"regexp"
	"strconv"
	"time"
)

const (
	equalToOperatorName = "equalTo"
)

var operatorFactoryRegistry = map[string]operatorFactory{
	equalToOperatorName:       newEqualsTo,
	"matchesRegexp":           newMatchesRegexp,
	"numberEqualTo":           newNumberEqualTo,
	"numberHigherThan":        newNumberHigherThan,
	"numberEqualOrHigherThan": newNumberEqualOrHigherThan,
	"numberEqualOrLessThan":   newNumberEqualOrLessThan,
	"durationHigherThan":      newDurationHigherThan,
}

type operatorFactory func() operator

type operator interface {
	Evaluate(*event.HttpRequest) (bool, error)
	LoadOptions(operatorOptions) error
}

func newOperator(options operatorOptions) (operator, error) {
	operatorFactory, ok := operatorFactoryRegistry[options.Operator]
	if !ok {
		var allowedKeys []string
		for k := range operatorFactoryRegistry {
			allowedKeys = append(allowedKeys, k)
		}
		return nil, fmt.Errorf("unknown operator %s, possible options are: %s", options.Operator, allowedKeys)
	}
	op := operatorFactory()
	if err := op.LoadOptions(options); err != nil {
		return nil, err
	}
	return op, nil
}

type numberComparisonOperator struct {
	name  string
	key   string
	value float64
}

func (n *numberComparisonOperator) String() string {
	return fmt.Sprintf("%s operator on key %q with value %f", n.name, n.key, n.value)
}

func (n *numberComparisonOperator) LoadOptions(options operatorOptions) error {
	n.key = options.Key
	threshold, err := strconv.ParseFloat(options.Value, 64)
	if err != nil {
		return fmt.Errorf("invalid value for operator %s, should be in float like format: %w", n.name, err)
	}
	n.value = threshold
	return nil
}

func (n *numberComparisonOperator) getKeyNumber(evaluatedEvent *event.HttpRequest) (float64, bool, error) {
	metadataValue, ok := evaluatedEvent.Metadata[n.key]
	if !ok {
		return 0, false, nil
	}
	testedValue, err := strconv.ParseFloat(metadataValue, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid metadata value for operator %s, should be in float like format: %w", n.name, err)
	}
	return testedValue, true, nil
}

// Operator `numberHigherThan`
func newNumberHigherThan() operator {
	return &numberHigherThan{numberComparisonOperator{name: "numberHigherThan"}}
}

type numberHigherThan struct {
	numberComparisonOperator
}

func (r *numberHigherThan) Evaluate(evaluatedEvent *event.HttpRequest) (bool, error) {
	testedValue, ok, err := r.getKeyNumber(evaluatedEvent)
	if !ok {
		return false, err
	}
	return testedValue > r.value, nil
}

// Operator `numberEqualOrHigherThan`
func newNumberEqualOrHigherThan() operator {
	return &numberEqualOrHigherThan{numberComparisonOperator{name: "numberEqualOrHigherThan"}}
}

type numberEqualOrHigherThan struct {
	numberComparisonOperator
}

func (r *numberEqualOrHigherThan) Evaluate(evaluatedEvent *event.HttpRequest) (bool, error) {
	testedValue, ok, err := r.getKeyNumber(evaluatedEvent)
	if !ok {
		return false, err
	}
	return testedValue >= r.value, nil
}

// Operator `numberEqualOrLessThan`
func newNumberEqualOrLessThan() operator {
	return &numberEqualOrLessThan{numberComparisonOperator{name: "numberEqualOrLessThan"}}
}

type numberEqualOrLessThan struct {
	numberComparisonOperator
}

func (r *numberEqualOrLessThan) Evaluate(evaluatedEvent *event.HttpRequest) (bool, error) {
	testedValue, ok, err := r.getKeyNumber(evaluatedEvent)
	if !ok {
		return false, err
	}
	return testedValue <= r.value, nil
}

// Operator `numberEqualTo`
func newNumberEqualTo() operator {
	return &numberEqualTo{numberComparisonOperator{name: "numberEqualTo"}}
}

type numberEqualTo struct {
	numberComparisonOperator
}

func (r *numberEqualTo) Evaluate(evaluatedEvent *event.HttpRequest) (bool, error) {
	testedValue, ok, err := r.getKeyNumber(evaluatedEvent)
	if !ok {
		return false, err
	}
	return testedValue == r.value, nil
}

// Operator `durationHigherThan`
func newDurationHigherThan() operator {
	return &durationHigherThan{}
}

type durationHigherThan struct {
	key               string
	thresholdDuration time.Duration
}

func (r *durationHigherThan) String() string {
	return fmt.Sprintf("durationHigherThan operator on key %q with value %q", r.key, r.thresholdDuration)
}

func (r *durationHigherThan) LoadOptions(options operatorOptions) error {
	r.key = options.Key
	thresholdDuration, err := time.ParseDuration(options.Value)
	if err != nil {
		return fmt.Errorf("invalid duration value for operator durationHigherThan, should be in Go duration format: %w", err)
	}
	r.thresholdDuration = thresholdDuration
	return nil
}

func (r *durationHigherThan) Evaluate(evaluatedEvent *event.HttpRequest) (bool, error) {
	metadataValue, ok := evaluatedEvent.Metadata[r.key]
	if !ok {
		return false, nil
	}
	testedDuration, err := time.ParseDuration(metadataValue)
	if err != nil {
		return false, fmt.Errorf("invalid metadata value for operator durationHigherThan, should be in Go duration format: %w", err)
	}
	return testedDuration > r.thresholdDuration, nil
}

// Operator `equalsTo`
func newEqualsTo() operator {
	return &equalsTo{}
}

type equalsTo struct {
	key   string
	value string
}

func (r *equalsTo) String() string {
	return fmt.Sprintf("equalTo operator on key %q with value %q", r.key, r.value)
}

func (r *equalsTo) LoadOptions(options operatorOptions) error {
	r.key = options.Key
	r.value = options.Value
	return nil
}

func (r *equalsTo) Evaluate(evaluatedEvent *event.HttpRequest) (bool, error) {
	testedValue, ok := evaluatedEvent.Metadata[r.key]
	if !ok {
		return false, nil
	}
	return r.value == testedValue, nil
}

// Operator `matchesRegexp`
func newMatchesRegexp() operator {
	return &matchesRegexp{}
}

type matchesRegexp struct {
	key    string
	regexp *regexp.Regexp
}

func (r *matchesRegexp) String() string {
	return fmt.Sprintf("matchesRegexp operator on key %q with matcher %q", r.key, r.regexp)
}

func (r *matchesRegexp) LoadOptions(options operatorOptions) error {
	var err error
	r.key = options.Key
	if r.regexp, err = regexp.Compile(options.Value); err != nil {
		return fmt.Errorf("invalid regexp matcher for matchesRegexp operator: %w", err)
	}
	return err
}

func (r *matchesRegexp) Evaluate(evaluatedEvent *event.HttpRequest) (bool, error) {
	testedValue, ok := evaluatedEvent.Metadata[r.key]
	if !ok {
		return false, nil
	}
	return r.regexp.MatchString(testedValue), nil
}
