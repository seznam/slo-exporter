//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"fmt"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"regexp"
	"strconv"
	"time"
)

var operatorFactoryRegistry = map[string]operatorFactory{
	"isEqualTo":                 newIsEqualTo,
	"isNotEqualTo":              newIsNotEqualTo,
	"isMatchingRegexp":          newIsMatchingRegexp,
	"isNotMatchingRegexp":       newIsNotMatchingRegexp,
	"numberIsEqualTo":           newNumberIsEqualTo,
	"numberIsNotEqualTo":        newNumberIsNotEqualTo,
	"numberIsHigherThan":        newNumberIsHigherThan,
	"numberIsEqualOrHigherThan": newNumberIsEqualOrHigherThan,
	"numberIsEqualOrLessThan":   newNumberIsEqualOrLessThan,
	"durationIsHigherThan":      newDurationIsHigherThan,
}

type operatorFactory func() operator

type operator interface {
	Evaluate(*event.Raw) (bool, error)
	LoadOptions(operatorOptions) error
}

const (
	operatorNameLabel = "operator"
)

type metric struct {
	Labels stringmap.StringMap
	Value  float64
}

// operator which is able to expose itself as a metric
type exposableOperator interface {
	AsMetric() metric
}

// operators which is able to represent itself as a labels of Prometheus metric
type labelsExposableOperator interface {
	Labels() stringmap.StringMap
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

func (n *numberComparisonOperator) getKeyNumber(evaluatedEvent *event.Raw) (float64, bool, error) {
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

func (n *numberComparisonOperator) AsMetric() metric {
	return metric{
		Labels: stringmap.StringMap{operatorNameLabel: n.name},
		Value:  n.value,
	}
}

// Operator `numberIsHigherThan`
func newNumberIsHigherThan() operator {
	return &numberIsHigherThan{numberComparisonOperator{name: "numberIsHigherThan"}}
}

type numberIsHigherThan struct {
	numberComparisonOperator
}

func (r *numberIsHigherThan) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok, err := r.getKeyNumber(evaluatedEvent)
	if !ok {
		return false, err
	}
	return testedValue > r.value, nil
}

// Operator `numberIsEqualOrHigherThan`
func newNumberIsEqualOrHigherThan() operator {
	return &numberIsEqualOrHigherThan{numberComparisonOperator{name: "numberIsEqualOrHigherThan"}}
}

type numberIsEqualOrHigherThan struct {
	numberComparisonOperator
}

func (r *numberIsEqualOrHigherThan) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok, err := r.getKeyNumber(evaluatedEvent)
	if !ok {
		return false, err
	}
	return testedValue >= r.value, nil
}

// Operator `numberIsEqualOrLessThan`
func newNumberIsEqualOrLessThan() operator {
	return &numberIsEqualOrLessThan{numberComparisonOperator{name: "numberIsEqualOrLessThan"}}
}

type numberIsEqualOrLessThan struct {
	numberComparisonOperator
}

func (r *numberIsEqualOrLessThan) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok, err := r.getKeyNumber(evaluatedEvent)
	if !ok {
		return false, err
	}
	return testedValue <= r.value, nil
}

// Operator `numberIsEqualTo`
func newNumberIsEqualTo() operator {
	return &numberIsEqualTo{numberComparisonOperator{name: "numberIsEqualTo"}}
}

type numberIsEqualTo struct {
	numberComparisonOperator
}

func (r *numberIsEqualTo) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok, err := r.getKeyNumber(evaluatedEvent)
	if !ok {
		return false, err
	}
	return testedValue == r.value, nil
}

func (r *numberIsEqualTo) Labels() stringmap.StringMap {
	return stringmap.StringMap{r.key: fmt.Sprintf("%g", r.value)}
}

// Operator `numberIsNotEqualTo`
func newNumberIsNotEqualTo() operator {
	return &numberIsNotEqualTo{numberComparisonOperator{name: "numberIsNotEqualTo"}}
}

type numberIsNotEqualTo struct {
	numberComparisonOperator
}

func (r *numberIsNotEqualTo) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok, err := r.getKeyNumber(evaluatedEvent)
	if !ok {
		return false, err
	}
	return testedValue != r.value, nil
}

// Operator `durationIsHigherThan`
func newDurationIsHigherThan() operator {
	return &durationIsHigherThan{}
}

type durationIsHigherThan struct {
	key               string
	thresholdDuration time.Duration
}

func (r *durationIsHigherThan) String() string {
	return fmt.Sprintf("durationIsHigherThan operator on key %q with value %q", r.key, r.thresholdDuration)
}

func (r *durationIsHigherThan) LoadOptions(options operatorOptions) error {
	r.key = options.Key
	thresholdDuration, err := time.ParseDuration(options.Value)
	if err != nil {
		return fmt.Errorf("invalid duration value for operator durationIsHigherThan, should be in Go duration format: %w", err)
	}
	r.thresholdDuration = thresholdDuration
	return nil
}

func (r *durationIsHigherThan) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	metadataValue, ok := evaluatedEvent.Metadata[r.key]
	if !ok {
		return false, nil
	}
	testedDuration, err := time.ParseDuration(metadataValue)
	if err != nil {
		return false, fmt.Errorf("invalid metadata value for operator durationIsHigherThan, should be in Go duration format: %w", err)
	}
	return testedDuration > r.thresholdDuration, nil
}

// Operator `isEqualTo`
func newIsEqualTo() operator {
	return &isEqualTo{}
}

type isEqualTo struct {
	key   string
	value string
}

func (r *isEqualTo) String() string {
	return fmt.Sprintf("isEqualTo operator on key %q with value %q", r.key, r.value)
}

func (r *isEqualTo) LoadOptions(options operatorOptions) error {
	r.key = options.Key
	r.value = options.Value
	return nil
}

func (r *isEqualTo) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok := evaluatedEvent.Metadata[r.key]
	if !ok {
		return false, nil
	}
	return r.value == testedValue, nil
}

func (r *isEqualTo) Labels() stringmap.StringMap {
	return stringmap.StringMap{r.key: r.value}
}

// Operator `isNotEqualTo`
func newIsNotEqualTo() operator {
	return &isNotEqualTo{}
}

type isNotEqualTo struct {
	key   string
	value string
}

func (r *isNotEqualTo) String() string {
	return fmt.Sprintf("isNotEqualTo operator on key %q with value %q", r.key, r.value)
}

func (r *isNotEqualTo) LoadOptions(options operatorOptions) error {
	r.key = options.Key
	r.value = options.Value
	return nil
}

func (r *isNotEqualTo) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok := evaluatedEvent.Metadata[r.key]
	if !ok {
		return false, nil
	}
	return r.value != testedValue, nil
}

// Operator `isMatchingRegexp`
func newIsMatchingRegexp() operator {
	return &isMatchingRegexp{}
}

type isMatchingRegexp struct {
	key    string
	regexp *regexp.Regexp
}

func (r *isMatchingRegexp) String() string {
	return fmt.Sprintf("newIsMatchingRegexp operator on key %q with matcher %q", r.key, r.regexp)
}

func (r *isMatchingRegexp) LoadOptions(options operatorOptions) error {
	var err error
	r.key = options.Key
	if r.regexp, err = regexp.Compile(options.Value); err != nil {
		return fmt.Errorf("invalid regexp matcher for isMatchingRegexp operator: %w", err)
	}
	return err
}

func (r *isMatchingRegexp) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok := evaluatedEvent.Metadata[r.key]
	if !ok {
		return false, nil
	}
	return r.regexp.MatchString(testedValue), nil
}

// Operator `isNotMatchingRegexp`
func newIsNotMatchingRegexp() operator {
	return &isNotMatchingRegexp{}
}

type isNotMatchingRegexp struct {
	key    string
	regexp *regexp.Regexp
}

func (r *isNotMatchingRegexp) String() string {
	return fmt.Sprintf("isNotMatchRegexp operator on key %q with matcher %q", r.key, r.regexp)
}

func (r *isNotMatchingRegexp) LoadOptions(options operatorOptions) error {
	var err error
	r.key = options.Key
	if r.regexp, err = regexp.Compile(options.Value); err != nil {
		return fmt.Errorf("invalid regexp matcher for isMatchingRegexp operator: %w", err)
	}
	return err
}

func (r *isNotMatchingRegexp) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok := evaluatedEvent.Metadata[r.key]
	if !ok {
		return false, nil
	}
	return !r.regexp.MatchString(testedValue), nil
}
