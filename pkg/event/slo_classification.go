package event

import (
	"fmt"

	"github.com/seznam/slo-exporter/pkg/stringmap"
)

type SloClassification struct {
	Domain string
	App    string
	Class  string
}

func (sc *SloClassification) Matches(other SloClassification) bool {
	if sc.Domain != "" && (sc.Domain != other.Domain) {
		return false
	}
	if sc.Class != "" && (sc.Class != other.Class) {
		return false
	}
	if sc.App != "" && (sc.App != other.App) {
		return false
	}
	return true
}

func (sc *SloClassification) GetMetadata() stringmap.StringMap {
	return stringmap.StringMap{
		"slo_domain": sc.Domain,
		"slo_class":  sc.Class,
		"app":        sc.App,
	}
}

func (sc *SloClassification) Copy() SloClassification {
	return SloClassification{
		Domain: sc.Domain,
		App:    sc.App,
		Class:  sc.Class,
	}
}

func (sc *SloClassification) String() string {
	return fmt.Sprintf("%s:%s:%s", sc.Domain, sc.App, sc.Class)
}
