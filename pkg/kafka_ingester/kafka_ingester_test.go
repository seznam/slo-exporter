package kafka_ingester

import (
	"testing"

	"github.com/segmentio/kafka-go"

	"github.com/stretchr/testify/assert"

	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/stringmap"
)

func Test_processMessage(t *testing.T) {
	tests := []struct {
		Name         string
		KafkaMessage kafka.Message
		OutputEvent  *event.Raw
		ErrExpected  bool
	}{
		{
			Name: "Empty event",
			KafkaMessage: kafka.Message{
				Topic: "topic",
				Value: []byte(`{}`),
			},
			OutputEvent: &event.Raw{Quantity: 1},
		},
		{
			Name: "Event without classification",
			KafkaMessage: kafka.Message{
				Topic: "topic",
				Value: []byte(`{
					"quantity": 1,
					"metadata": {"foo": "bar"}
				}`),
			},
			OutputEvent: &event.Raw{Quantity: 1, Metadata: stringmap.StringMap{"foo": "bar"}},
		},
		{
			Name: "Event with classification",
			KafkaMessage: kafka.Message{
				Topic: "topic",
				Value: []byte(`{
					"quantity": 1,
					"metadata": {"foo": "bar"},
					"slo_classification": {"app": "fooApp", "class": "fooClass", "domain": "fooDomain"}
				}`),
			},
			OutputEvent: &event.Raw{
				Quantity:          1,
				Metadata:          stringmap.StringMap{"foo": "bar"},
				SloClassification: &event.SloClassification{App: "fooApp", Class: "fooClass", Domain: "fooDomain"},
			},
		},
		{
			Name: "Kafka message containing only unknown fields",
			KafkaMessage: kafka.Message{
				Topic: "topic",
				Value: []byte(`{"unknown_field": 1111}`),
			},
			OutputEvent: &event.Raw{Quantity: 1},
		},
		{
			Name: "Valid event data accompanied with unknown fields",
			KafkaMessage: kafka.Message{
				Topic: "topic",
				Value: []byte(`{
					"quantity": 1,
					"metadata": {"foo": "bar"},
					"slo_classification": {"app": "fooApp", "class": "fooClass", "domain": "fooDomain"},
					"unknown_fields": "foo"
				}`),
			},
			OutputEvent: &event.Raw{
				Quantity:          1,
				Metadata:          stringmap.StringMap{"foo": "bar"},
				SloClassification: &event.SloClassification{App: "fooApp", Class: "fooClass", Domain: "fooDomain"},
			},
		},
		{
			Name: "Valid event data, explicit schema version",
			KafkaMessage: kafka.Message{
				Headers: []kafka.Header{{Key: schemaVersionMessageHeader, Value: []byte(schemaVerV1)}},
				Topic:   "topic",
				Value: []byte(`{
					"quantity": 1,
					"metadata": {"foo": "bar"},
					"slo_classification": {"app": "fooApp", "class": "fooClass", "domain": "fooDomain"}
				}`),
			},
			OutputEvent: &event.Raw{
				Quantity:          1,
				Metadata:          stringmap.StringMap{"foo": "bar"},
				SloClassification: &event.SloClassification{App: "fooApp", Class: "fooClass", Domain: "fooDomain"},
			},
		},
		{
			Name: "Valid event data, unknown schema version",
			KafkaMessage: kafka.Message{
				Headers: []kafka.Header{{Key: schemaVersionMessageHeader, Value: []byte("unknown")}},
				Topic:   "topic",
				Value: []byte(`{
					"quantity": 1,
					"metadata": {"foo": "bar"},
					"slo_classification": {"app": "fooApp", "class": "fooClass", "domain": "fooDomain"}
				}`),
			},
			OutputEvent: nil,
			ErrExpected: true,
		},
		{
			Name: "Invalid event data, invalid quantity type",
			KafkaMessage: kafka.Message{
				Headers: []kafka.Header{{Key: schemaVersionMessageHeader, Value: []byte("unknown")}},
				Topic:   "topic",
				Value: []byte(`{
					"quantity": "1",
				}`),
			},
			OutputEvent: nil,
			ErrExpected: true,
		},
		{
			Name: "Invalid event data, list of structs rather than struct",
			KafkaMessage: kafka.Message{
				Headers: []kafka.Header{{Key: schemaVersionMessageHeader, Value: []byte("unknown")}},
				Topic:   "topic",
				Value:   []byte(`[{}]`),
			},
			OutputEvent: nil,
			ErrExpected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			e, err := processMessage(test.KafkaMessage)
			if err != nil {
				if !test.ErrExpected {
					t.Errorf("Unexpected error while processing kafka message: %v", err)
				}
			}
			if test.ErrExpected && err == nil {
				t.Errorf("Event processing was expected to result in error, but none occurred")
			}
			assert.Equal(t, test.OutputEvent, e)
		})
	}
}

func Test_getSchemaVersionFromHeaders(t *testing.T) {
	tests := []struct {
		name          string
		headers       []kafka.Header
		schemaVersion string
		ok            bool
	}{
		{
			"Schema version missing from the headers",
			[]kafka.Header{},
			"",
			false,
		},
		{
			"Schema version present in the headers",
			[]kafka.Header{
				{
					Key:   schemaVersionMessageHeader,
					Value: []byte(schemaVerV1),
				},
			},
			schemaVerV1,
			true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schemaVersion, ok := getSchemaVersionFromHeaders(test.headers)
			assert.Equal(t, test.ok, ok, "Checking whether schema version was retrieved from the headers")
			assert.Equal(t, test.schemaVersion, schemaVersion, "Checking whether schemaVersion was correctly retrieved from the headers")
		})
	}
}
