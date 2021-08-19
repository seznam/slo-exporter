package event

import (
	"fmt"
	"github.com/seznam/slo-exporter/pkg/stringmap"
)

type Result string

func (r Result) String() string {
	return string(r)
}

const (
	Success Result = "success"
	Fail    Result = "fail"
)

var (
	PossibleResults = []Result{Success, Fail}
)

type Slo struct {
	// same value as in source event Raw.EventKey()
	Key    string
	Result Result

	Domain string
	Class  string
	App    string

	Metadata stringmap.StringMap
	Quantity float64

	OriginalEvent Raw
}

func (s *Slo) IsClassified() bool {
	return s.Domain != "" && s.Class != "" && s.App != ""
}

func (s *Slo) String() string {
	return fmt.Sprintf("SLO event %q of domain: %q, class: %q, app: %q with metadata: %+v", s.Key, s.Domain, s.Class, s.App, s.Metadata)
}

func (s Slo) Copy() Slo {
	return Slo{
		Key:      s.Key,
		Result:   s.Result,
		Domain:   s.Domain,
		Class:    s.Class,
		App:      s.App,
		Metadata: s.Metadata.Copy(),
		OriginalEvent: s.OriginalEvent,
	}
}
