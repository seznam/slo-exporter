package kafka_ingester

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/pipeline"
	"github.com/seznam/slo-exporter/pkg/stringmap"

	"github.com/segmentio/kafka-go"
)

const (
	schemaVersionMessageHeader = "slo-exporter-schema-version"
	schemaVerV1                = "v1"
	defaultSchemaVer           = schemaVerV1
)

var (
	kafkaConnectionInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kafka_connection_info",
		Help: "Metadata metric with information about Kafka connection",
	}, []string{"brokers", "group_id", "topic"})
	messagesReadTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kafka_messages_read_total",
		Help: "Total number of messages read from Kafka.",
	})
	malformedMessagesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "malformed_messages_total",
			Help: "Total number of invalid Kafka messages that failed to parse.",
		},
	)
	kafkaReadErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kafka_read_errors_total",
			Help: "Total number of errors encountered when reading messages from Kafka.",
		},
	)

	fallbackStartOffsetMapping = map[string]int64{
		"LastOffset":  kafka.LastOffset,
		"FirstOffset": kafka.FirstOffset,
	}
)

type IngestedEventV1 struct {
	Metadata          stringmap.StringMap            `json:"metadata"`
	SloClassification *IngestedEventV1Classification `json:"slo_classification"`
	Quantity          *float64                       `json:"quantity"`
}

type IngestedEventV1Classification struct {
	Domain string `json:"domain"`
	App    string `json:"app"`
	Class  string `json:"class"`
}

type kafkaIngesterConfig struct {
	LogKafkaEvents      bool
	LogKafkaErrors      bool
	Brokers             []string
	Topic               string
	GroupID             string
	FallbackStartOffset string
	CommitInterval      time.Duration
	// RetentionTime optionally sets the length of time the consumer group will be saved by the broker.
	RetentionTime time.Duration
}

type KafkaIngester struct {
	kafkaReader     *kafka.Reader
	observer        pipeline.EventProcessingDurationObserver
	outputChannel   chan *event.Raw
	shutdownChannel chan struct{}
	logger          logrus.FieldLogger
	done            bool
}

func (k *KafkaIngester) String() string {
	return "kafkaIngester"
}

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*KafkaIngester, error) {
	var config kafkaIngesterConfig
	viperConfig.SetDefault("logKafkaErrors", true)
	viperConfig.SetDefault("commitInterval", 0)
	viperConfig.SetDefault("retentionTime", 24*time.Hour)
	viperConfig.SetDefault("fallbackStartOffset", "FirstOffset")
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	if _, ok := fallbackStartOffsetMapping[config.FallbackStartOffset]; !ok {
		return nil, fmt.Errorf("invalid 'fallbackStartOffset' value provided: %s", config.FallbackStartOffset)
	}
	return New(config, logger)
}

// New returns an instance of KafkaIngester.
func New(config kafkaIngesterConfig, logger logrus.FieldLogger) (*KafkaIngester, error) {
	var kafkaLogger, kafkaErrorLogger logrus.FieldLogger
	if config.LogKafkaEvents {
		kafkaLogger = logger
	}
	if config.LogKafkaErrors {
		kafkaErrorLogger = logger
	}

	reader := kafka.NewReader(
		kafka.ReaderConfig{
			Brokers:        config.Brokers,
			GroupID:        config.GroupID,
			Topic:          config.Topic,
			CommitInterval: config.CommitInterval,
			RetentionTime:  config.RetentionTime,
			StartOffset:    fallbackStartOffsetMapping[config.FallbackStartOffset],
			Logger:         kafkaLogger,
			ErrorLogger:    kafkaErrorLogger,
		})

	kafkaConnectionInfo.With(prometheus.Labels{
		"brokers":  strings.Join(config.Brokers, ","),
		"group_id": config.GroupID,
		"topic":    config.Topic,
	}).Set(1)

	return &KafkaIngester{
		outputChannel:   make(chan *event.Raw),
		shutdownChannel: make(chan struct{}),
		done:            false,
		logger:          logger,
		kafkaReader:     reader,
	}, nil
}

func (k *KafkaIngester) RegisterEventProcessingDurationObserver(observer pipeline.EventProcessingDurationObserver) {
	k.observer = observer
}

func (k *KafkaIngester) observeDuration(start time.Time) {
	if k.observer != nil {
		k.observer.Observe(time.Since(start).Seconds())
	}
}

func (k *KafkaIngester) RegisterMetrics(_, wrappedRegistry prometheus.Registerer) error {
	toRegister := []prometheus.Collector{kafkaConnectionInfo, messagesReadTotal, malformedMessagesTotal}
	for _, collector := range toRegister {
		if err := wrappedRegistry.Register(collector); err != nil {
			return fmt.Errorf("error registering metric %s: %w", collector, err)
		}
	}
	return nil
}

func (k *KafkaIngester) Done() bool {
	return k.done
}

func (k *KafkaIngester) OutputChannel() chan *event.Raw {
	return k.outputChannel
}

// Run starts to tail the associated file, feeding events to output channel.
func (k *KafkaIngester) Run() {
	ctx, cancel := context.WithCancel(context.Background())

	// Goroutine handling shutdown signal
	go func() {
		defer func() {
			close(k.outputChannel)
			k.done = true
		}()
		<-k.shutdownChannel
		cancel()
		k.kafkaReader.Close()
	}()

	// Main goroutine for reading messages from Kafka
	go func() {
		for {
			m, err := k.kafkaReader.ReadMessage(ctx)
			start := time.Now()
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
					return
				}
				k.logger.Errorf("error while reading message from Kafka: %w", err)
				kafkaReadErrorsTotal.Inc()
				continue
			}
			k.logger.Debug(m)
			messagesReadTotal.Inc()
			e, err := processMessage(m)
			if err != nil {
				k.logger.Errorf("Error while parsing the message: %w", err)
				malformedMessagesTotal.Inc()
			} else {
				k.outputChannel <- e
			}
			k.observeDuration(start)
		}
	}()
}

func getSchemaVersionFromHeaders(headers []kafka.Header) (string, bool) {
	for _, header := range headers {
		if header.Key == schemaVersionMessageHeader {
			return string(header.Value), true
		}
	}
	return "", false
}

func processMessage(m kafka.Message) (*event.Raw, error) {
	schemaVer, ok := getSchemaVersionFromHeaders(m.Headers)
	if !ok {
		schemaVer = defaultSchemaVer
	}
	switch schemaVer {
	case schemaVerV1:
		return processEventV1(m)
	default:
		return nil, fmt.Errorf("unknown schema version: %s", schemaVer)
	}
}

func processEventV1(m kafka.Message) (*event.Raw, error) {
	ingestedEvent := IngestedEventV1{}
	err := json.Unmarshal(m.Value, &ingestedEvent)
	if err != nil {
		return nil, err
	}
	var quantity float64
	if ingestedEvent.Quantity == nil {
		// Quantity not specified in the ingested data, default to 1
		quantity = 1
	} else {
		quantity = *ingestedEvent.Quantity
	}
	var classification *event.SloClassification
	if ingestedEvent.SloClassification != nil {
		classification = &event.SloClassification{
			Domain: ingestedEvent.SloClassification.Domain,
			App:    ingestedEvent.SloClassification.App,
			Class:  ingestedEvent.SloClassification.Class,
		}
	}
	outputEvent := event.Raw{
		Metadata:          ingestedEvent.Metadata,
		Quantity:          quantity,
		SloClassification: classification,
	}
	return &outputEvent, nil
}

func (k *KafkaIngester) Stop() {
	if !k.done {
		k.shutdownChannel <- struct{}{}
	}
}
