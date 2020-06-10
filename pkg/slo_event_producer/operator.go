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
	"equalTo":                 newEqualsTo,
	"notEqualTo":              newNotEqualsTo,
	"matchesRegexp":           newMatchesRegexp,
	"notMatchesRegexp":        newNotMatchesRegexp,
	"numberEqualTo":           newNumberEqualTo,
	"numberNotEqualTo":        newNumberNotEqualTo,
	"numberHigherThan":        newNumberHigherThan,
	"numberEqualOrHigherThan": newNumberEqualOrHigherThan,
	"numberEqualOrLessThan":   newNumberEqualOrLessThan,
	"durationHigherThan":      newDurationHigherThan,
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

// Operator `numberHigherThan`
func newNumberHigherThan() operator {
	return &numberHigherThan{numberComparisonOperator{name: "numberHigherThan"}}
}

type numberHigherThan struct {
	numberComparisonOperator
}

func (r *numberHigherThan) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
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

func (r *numberEqualOrHigherThan) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
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

func (r *numberEqualOrLessThan) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
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

func (r *numberEqualTo) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok, err := r.getKeyNumber(evaluatedEvent)
	if !ok {
		return false, err
	}
	return testedValue == r.value, nil
}

func (r *numberEqualTo) Labels() stringmap.StringMap {
	return stringmap.StringMap{r.key: fmt.Sprintf("%g", r.value)}
}

// Operator `numberNotEqualTo`
func newNumberNotEqualTo() operator {
	return &numberNotEqualTo{numberComparisonOperator{name: "numberNotEqualTo"}}
}

type numberNotEqualTo struct {
	numberComparisonOperator
}

func (r *numberNotEqualTo) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok, err := r.getKeyNumber(evaluatedEvent)
	if !ok {
		return false, err
	}
	return testedValue != r.value, nil
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

func (r *durationHigherThan) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
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

func (r *equalsTo) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok := evaluatedEvent.Metadata[r.key]
	if !ok {
		return false, nil
	}
	return r.value == testedValue, nil
}

func (r *equalsTo) Labels() stringmap.StringMap {
	return stringmap.StringMap{r.key: r.value}
}

// Operator `notEqualsTo`
func newNotEqualsTo() operator {
	return &notEqualsTo{}
}

type notEqualsTo struct {
	key   string
	value string
}

func (r *notEqualsTo) String() string {
	return fmt.Sprintf("notEqualTo operator on key %q with value %q", r.key, r.value)
}

func (r *notEqualsTo) LoadOptions(options operatorOptions) error {
	r.key = options.Key
	r.value = options.Value
	return nil
}

func (r *notEqualsTo) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok := evaluatedEvent.Metadata[r.key]
	if !ok {
		return false, nil
	}
	return r.value != testedValue, nil
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

func (r *matchesRegexp) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok := evaluatedEvent.Metadata[r.key]
	if !ok {
		return false, nil
	}
	return r.regexp.MatchString(testedValue), nil
}

// Operator `notMatchesRegexp`
func newNotMatchesRegexp() operator {
	return &notMatchesRegexp{}
}

type notMatchesRegexp struct {
	key    string
	regexp *regexp.Regexp
}

func (r *notMatchesRegexp) String() string {
	return fmt.Sprintf("notMatchesRegexp operator on key %q with matcher %q", r.key, r.regexp)
}

func (r *notMatchesRegexp) LoadOptions(options operatorOptions) error {
	var err error
	r.key = options.Key
	if r.regexp, err = regexp.Compile(options.Value); err != nil {
		return fmt.Errorf("invalid regexp matcher for matchesRegexp operator: %w", err)
	}
	return err
}

func (r *notMatchesRegexp) Evaluate(evaluatedEvent *event.Raw) (bool, error) {
	testedValue, ok := evaluatedEvent.Metadata[r.key]
	if !ok {
		return false, nil
	}
	return !r.regexp.MatchString(testedValue), nil
}
