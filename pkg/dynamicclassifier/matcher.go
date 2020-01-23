package dynamicclassifier

import "gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"

type matcher interface {
	set(key string, classification *producer.SloClassification) error
	get(key string) (*producer.SloClassification, error)
}
