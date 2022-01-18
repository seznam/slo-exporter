package main

import "fmt"

var (
	defaultTimerangeThresholds = []BurnRateThreshold{
		{
			Condition: Condition{TimeRange: "1h"},
			Value:     13.44,
		},
		{
			Condition: Condition{TimeRange: "6h"},
			Value:     5.6,
		},
		{
			Condition: Condition{TimeRange: "1d"},
			Value:     2.8,
		},
		{
			Condition: Condition{TimeRange: "3d"},
			Value:     1,
		},
	}
)

type Alerting struct{
	Team      string
	Escalate  string
	BurnRateThresholds []BurnRateThreshold `yaml:"burn_rate_thresholds"`
}

func (a Alerting) IsValid() []error {
	errs := []error{}
	for _, t := range a.BurnRateThresholds {
		if thresholdsErrs := t.IsValid(); len(thresholdsErrs) > 0 {
			errs = append(errs, thresholdsErrs...)
		}
	}
	return errs
}

type BurnRateThreshold struct{
	Condition Condition
	Value float32
}

// Returns subset of thresholds which matches given class and slo type
func getMatchingSubset(thresholds []BurnRateThreshold, className, sloType string) []BurnRateThreshold {
	matchingBurnRateThresholds := []BurnRateThreshold{}
	for _, t := range thresholds {
		if t.Condition.Matches(className, sloType) {
			matchingBurnRateThresholds = append(matchingBurnRateThresholds, t)
		}
	}
	return matchingBurnRateThresholds
}

func (t BurnRateThreshold) IsValid() []error {
	errs := []error{}
	if t.Value <= 0 {
		errs = append(errs, fmt.Errorf("burn-rate treshold must be greater than 0"))
	}
	if err := t.Condition.IsValid(); err != nil {
		errs = append(errs, err)
	}
	return errs
}

type Condition struct {
	Class string
	Type string `yaml:"slo_type"`
	TimeRange BurnRateTimeRange `yaml:"time_range"`
}

func (c Condition) Matches(class, sloType string) bool {
	return (c.Class == "" || c.Class == class) && (c.Type == "" || c.Type == sloType)
}

func (c Condition) IsValid() error {
	// Class and Type needs to be checked at global context, here we just validate the timerange
	return c.TimeRange.IsValid()
}

type BurnRateTimeRange string

func (t BurnRateTimeRange) IsValid() error {
	var found bool
	for _, burnRateTreshold := range defaultTimerangeThresholds {
		if burnRateTreshold.Condition.TimeRange == t {
			// given timerange matches one of timeranges in the default set
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid burn-rate timerange: %s.", string(t))
	}
	return nil
}

