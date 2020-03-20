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

var operatorFactoryRegistry = map[string]operatorFactory{
	"matchesRegexp":      newMatchesRegexp,
	"numberHigherThan":   newNumberHigherThan,
	"durationHigherThan": newDurationHigherThan,
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

func newDurationHigherThan() operator {
	return &durationHigherThan{}
}

type durationHigherThan struct {
	key               string
	thresholdDuration time.Duration
}

func (r *durationHigherThan) String() string {
	return fmt.Sprintf("durationHigherThan operator on key %q with threshold %q", r.key, r.thresholdDuration)
}

func (r *durationHigherThan) LoadOptions(options operatorOptions) error {
	r.key = options.Key
	thresholdDuration, err := time.ParseDuration(options.Value)
	if err != nil {
		return fmt.Errorf("invalid duration threshold for operator durationHigherThan, should be in Go duration format: %w", err)
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

func newNumberHigherThan() operator {
	return &numberHigherThan{}
}

type numberHigherThan struct {
	key       string
	threshold float64
}

func (r *numberHigherThan) String() string {
	return fmt.Sprintf("numberHigherThan operator on key %q with threshold %f", r.key, r.threshold)
}

func (r *numberHigherThan) LoadOptions(options operatorOptions) error {
	r.key = options.Key
	threshold, err := strconv.ParseFloat(options.Value, 64)
	if err != nil {
		return fmt.Errorf("invalid threshold for operator numberHigherThan, should be in float like format: %w", err)
	}
	r.threshold = threshold
	return nil
}

func (r *numberHigherThan) Evaluate(evaluatedEvent *event.HttpRequest) (bool, error) {
	metadataValue, ok := evaluatedEvent.Metadata[r.key]
	if !ok {
		return false, nil
	}
	testedValue, err := strconv.ParseFloat(metadataValue, 64)
	if err != nil {
		return false, fmt.Errorf("invalid metadata value for operator numberHigherThan, should be in float like format: %w", err)
	}
	return testedValue > r.threshold, nil
}

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
