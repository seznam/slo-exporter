//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"testing"
	"time"
)

type testCriteriumOpts struct {
	opts              criteriumOptions
	expectedCriterium criterium
	expectedErr       bool
}

func TestCriteria_newCriterium(t *testing.T) {
	testCases := []testCriteriumOpts{
		{opts: criteriumOptions{Criterium: "requestDurationHigherThan", Value: "1s"}, expectedCriterium: &requestDurationHigherThan{thresholdDuration: time.Second}, expectedErr: false},
		{opts: criteriumOptions{Criterium: "requestDurationHigherThan", Value: "xxx"}, expectedCriterium: nil, expectedErr: true},

		{opts: criteriumOptions{Criterium: "requestStatusHigherThan", Value: "500"}, expectedCriterium: &requestStatusHigherThan{statusThreshold: 500}, expectedErr: false},
		{opts: criteriumOptions{Criterium: "requestStatusHigherThan", Value: "xxx"}, expectedCriterium: nil, expectedErr: true},

		{opts: criteriumOptions{Criterium: "xxx", Value: "xxx"}, expectedCriterium: nil, expectedErr: true},
	}
	for _, c := range testCases {
		newCriterium, err := newCriterium(c.opts)
		if c.expectedErr {
			assert.Error(t, err)
			continue
		}
		assert.Equal(t, newCriterium, c.expectedCriterium)
	}
}

type testEvent struct {
	event     event.HttpRequest
	criterium criterium
	failed    bool
}

func TestCriteria_requestStatusHigherThan(t *testing.T) {
	testCases := []testEvent{
		{event: event.HttpRequest{StatusCode: 200}, criterium: &requestStatusHigherThan{statusThreshold: 500}, failed: false},
		{event: event.HttpRequest{StatusCode: 500}, criterium: &requestStatusHigherThan{statusThreshold: 500}, failed: false},
		{event: event.HttpRequest{StatusCode: 503}, criterium: &requestStatusHigherThan{statusThreshold: 500}, failed: true},
		{event: event.HttpRequest{}, criterium: &requestStatusHigherThan{statusThreshold: 500}, failed: false},
	}
	for _, c := range testCases {
		assert.Equal(t, c.failed, c.criterium.Evaluate(&c.event))
	}
}

func TestCriteria_requestDurationHigherThan(t *testing.T) {
	testCases := []testEvent{
		{event: event.HttpRequest{Duration: time.Millisecond}, criterium: &requestDurationHigherThan{thresholdDuration: time.Second}, failed: false},
		{event: event.HttpRequest{Duration: time.Second}, criterium: &requestDurationHigherThan{thresholdDuration: time.Second}, failed: false},
		{event: event.HttpRequest{Duration: 2 * time.Second}, criterium: &requestDurationHigherThan{thresholdDuration: time.Second}, failed: true},
		{event: event.HttpRequest{}, criterium: &requestDurationHigherThan{thresholdDuration: time.Second}, failed: false},
	}
	for _, c := range testCases {
		assert.Equal(t, c.failed, c.criterium.Evaluate(&c.event))
	}
}
