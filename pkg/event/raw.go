package event

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/seznam/slo-exporter/pkg/stringmap"
)

const (
	eventKeyMetadataKey = "__eventKey"
)

func NewRaw(id string, quantity float64, metadata stringmap.StringMap, classification *SloClassification) Raw {
	if id == "" {
		id = uuid.New().String()
	}
	if classification == nil {
		classification = &SloClassification{}
	}
	if metadata == nil {
		metadata = stringmap.StringMap{}
	}
	return &raw{
		id:                id,
		metadata:          metadata,
		sloClassification: *classification,
		quantity:          quantity,
	}
}

// raw represents single event as received by an EventsProcessor instance
type raw struct {
	id                string
	metadata          stringmap.StringMap
	sloClassification SloClassification
	quantity          float64
}

func (r raw) Quantity() float64 {
	return r.quantity
}

func (r *raw) SetQuantity(newQuantity float64) {
	r.quantity = newQuantity
}

func (r raw) Id() string {
	return r.id
}

func (r *raw) SetId(newId string) {
	r.id = newId
}

func (r raw) EventKey() string {
	return r.metadata[eventKeyMetadataKey]
}

func (r *raw) SetEventKey(k string) {
	if r.metadata == nil {
		r.metadata = make(stringmap.StringMap)
	}
	r.metadata[eventKeyMetadataKey] = k
}

func (r raw) Metadata() stringmap.StringMap {
	return r.metadata
}

func (r *raw) SetMetadata(metadata stringmap.StringMap) {
	r.metadata = metadata
}

// IsClassified check if all SloClassification fields are set
func (r raw) IsClassified() bool {
	return r.sloClassification.IsClassified()
}

func (r raw) SloClassification() SloClassification {
	return r.sloClassification
}

// SetSLOClassification updates SloClassification field
func (r *raw) SetSLOClassification(classification SloClassification) {
	r.sloClassification = classification
}

func (r raw) String() string {
	return fmt.Sprintf("id:%s, key: %s, metadata: %s, classification: %s", r.Id(), r.EventKey(), r.Metadata(), r.SloClassification())
}
