package event

import (
	"fmt"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"net/url"
)

// Raw represents single event as received by an EventsProcessor instance
type Raw struct {
	StatusCode        int
	URL               *url.URL
	Metadata          stringmap.StringMap
	Method            string
	SloResult         string
	SloClassification *SloClassification
	Quantity          float64
}

const (
	eventKeyMetadataKey = "__eventKey"
)

func (r *Raw) EventKey() string {
	return r.Metadata[eventKeyMetadataKey]
}

func (r *Raw) SetEventKey(k string) {
	if r.Metadata == nil {
		r.Metadata = make(stringmap.StringMap)
	}
	r.Metadata[eventKeyMetadataKey] = k
}

// UpdateSLOClassification updates SloClassification field
func (r *Raw) UpdateSLOClassification(classification *SloClassification) {
	r.SloClassification = classification
}

// IsClassified check if all SloClassification fields are set
func (r *Raw) IsClassified() bool {
	if r.SloClassification != nil &&
		r.SloClassification.Domain != "" &&
		r.SloClassification.App != "" &&
		r.SloClassification.Class != "" {

		return true
	}
	return false
}

func (r Raw) GetSloMetadata() stringmap.StringMap {
	if r.SloClassification == nil {
		return nil
	}
	metadata := r.SloClassification.GetMetadata()
	return metadata
}

func (r Raw) GetSloClassification() *SloClassification {
	return r.SloClassification
}

func (r Raw) String() string {
	return fmt.Sprintf("key: %s, metadata: %s, classification: %s", r.EventKey(), r.Metadata, r.GetSloMetadata())
}
