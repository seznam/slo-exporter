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

func NewSlo(eventKey string, quantity float64, result Result, classification SloClassification, metadata stringmap.StringMap) Slo {
	if metadata == nil {
		metadata = stringmap.StringMap{}
	}
	return &slo{
		key:            eventKey,
		result:         result,
		classification: classification,
		metadata:       metadata,
		quantity:       quantity,
	}
}

type slo struct {
	// same value as in source event Raw.EventKey()
	key    string
	result Result

	classification SloClassification

	metadata stringmap.StringMap
	quantity float64
}

func (s slo) Quantity() float64 {
	return s.quantity
}

func (s *slo) SetQuantity(newQuantity float64) {
	s.quantity = newQuantity
}

func (s *slo) SetMetadata(newMetadata stringmap.StringMap) {
	s.metadata = newMetadata
}

func (s slo) Metadata() stringmap.StringMap {
	return s.metadata
}

func (s slo) SloClassification() SloClassification {
	return s.classification
}

func (s *slo) SetSLOClassification(classification SloClassification) {
	s.classification = classification
}

func (s slo) EventKey() string {
	return s.key
}

func (s *slo) SetEventKey(newKey string) {
	s.key = newKey
}

func (s slo) Result() Result {
	return s.result
}

func (s slo) IsClassified() bool {
	return s.classification.IsClassified()
}

func (s slo) String() string {
	return fmt.Sprintf("Slo event %s of %s with metadata: %s", s.EventKey(), s.SloClassification(), s.Metadata())
}

func (s slo) Copy() Slo {
	return &slo{
		key:            s.EventKey(),
		result:         s.Result(),
		classification: s.SloClassification(),
		metadata:       s.Metadata().Copy(),
	}
}
