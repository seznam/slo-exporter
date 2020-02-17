//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"fmt"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"strconv"
	"time"
)

var criteriumFactoryRegistry = map[string]criteriumFactory{
	"requestStatusHigherThan":   newRequestStatusHigherThan,
	"requestDurationHigherThan": newRequestDurationHigherThan,
}

type criteriumFactory func() criterium

type criterium interface {
	Evaluate(*event.HttpRequest) bool
	LoadOptions(criteriumOptions) error
}

func newCriterium(options criteriumOptions) (criterium, error) {
	criteriumFactory, ok := criteriumFactoryRegistry[options.Criterium]
	if !ok {
		var allowedKeys []string
		for k := range criteriumFactoryRegistry {
			allowedKeys = append(allowedKeys, k)
		}
		return nil, fmt.Errorf("unknown criterium %s, possible options are: %s", options.Criterium, allowedKeys)
	}
	crit := criteriumFactory()
	if err := crit.LoadOptions(options); err != nil {
		return nil, err
	}
	return crit, nil
}

// criterium for request status
func newRequestStatusHigherThan() criterium {
	return &requestStatusHigherThan{}
}

type requestStatusHigherThan struct {
	statusThreshold int
}

func (r *requestStatusHigherThan) LoadOptions(options criteriumOptions) error {
	status, err := strconv.Atoi(options.Value)
	if err != nil {
		return fmt.Errorf("invalid status threshold for criterium requestStatusHigherThan: %w", err)
	}
	r.statusThreshold = status
	return nil
}

func (r *requestStatusHigherThan) Evaluate(request *event.HttpRequest) bool {
	return request.StatusCode > r.statusThreshold
}

// criterium for request duration
func newRequestDurationHigherThan() criterium {
	return &requestDurationHigherThan{}
}

type requestDurationHigherThan struct {
	thresholdDuration time.Duration
}

func (r *requestDurationHigherThan) LoadOptions(options criteriumOptions) error {
	duration, err := time.ParseDuration(options.Value)
	if err != nil {
		return fmt.Errorf("invalid duration threshold for criterium requestDurationHigherThan: %w", err)
	}
	r.thresholdDuration = duration
	return nil
}

func (r *requestDurationHigherThan) Evaluate(request *event.HttpRequest) bool {
	return request.Duration > r.thresholdDuration
}
