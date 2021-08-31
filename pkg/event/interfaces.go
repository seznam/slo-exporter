package event

import (
	"fmt"
	"github.com/seznam/slo-exporter/pkg/stringmap"
)

type WithEventKey interface {
	EventKey() string
	SetEventKey(string)
}

type Classifiable interface {
	IsClassified() bool
	SloClassification() SloClassification
	SetSLOClassification(classification SloClassification)
}

type WithMetadata interface {
	SetMetadata(stringmap.StringMap)
	Metadata() stringmap.StringMap
}

type WithQuantity interface {
	Quantity() float64
	SetQuantity(float64)
}

type WithId interface {
	Id() string
	SetId(string)
}

type Raw interface {
	fmt.Stringer
	Classifiable
	WithMetadata
	WithEventKey
	WithQuantity
	WithId
}

type Slo interface {
	fmt.Stringer
	Classifiable
	WithMetadata
	WithEventKey
	WithQuantity
	Result() Result
	Copy() Slo
}
