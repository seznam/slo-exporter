package elasticsearch_ingester

import (
	"context"
	"fmt"
	"github.com/seznam/slo-exporter/pkg/elasticsearch_client"
	"regexp"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/pipeline"
)

type elasticSearchIngesterConfig struct {
	ApiVersion             string
	Addresses              []string
	Username               string
	Password               string
	Timeout                time.Duration
	InsecureSkipVerify     bool
	Healthchecks           bool
	Sniffing               bool
	ClientCertFile         string
	ClientKeyFile          string
	CaCertFile             string
	Interval               time.Duration
	Index                  string
	TimestampField         string
	TimestampFormat        string
	RawLogField            string
	RawLogParseRegexp      string
	RawLogEmptyGroupRegexp string
	MaxBatchSize           int
	Query                  string
	Debug                  bool
}

type ElasticSearchIngester struct {
	tailer   tailer
	interval time.Duration

	observer        pipeline.EventProcessingDurationObserver
	outputChannel   chan *event.Raw
	shutdownChannel chan struct{}
	logger          logrus.FieldLogger
	done            bool
}

func (e *ElasticSearchIngester) String() string {
	return "elasticSearchIngester"
}

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*ElasticSearchIngester, error) {
	var config elasticSearchIngesterConfig
	viperConfig.SetDefault("apiVersion", "v7")
	viperConfig.SetDefault("addresses", nil)
	viperConfig.SetDefault("username", "")
	viperConfig.SetDefault("password", "")
	viperConfig.SetDefault("clientCertFile", "")
	viperConfig.SetDefault("clientKeyFile", "")
	viperConfig.SetDefault("caCertFile", "")
	viperConfig.SetDefault("timeout", time.Second*5)
	viperConfig.SetDefault("healthchecks", true)
	viperConfig.SetDefault("sniffing", true)
	viperConfig.SetDefault("debug", false)
	viperConfig.SetDefault("maxBatchSize", 100)
	viperConfig.SetDefault("interval", time.Second*5)
	viperConfig.SetDefault("timestampField", "@timestamp")
	viperConfig.SetDefault("timestampFormat", "2006-01-02T15:04:05Z07:00")
	viperConfig.SetDefault("rawLogField", "")
	viperConfig.SetDefault("rawLogParseRegexp", ".*")
	viperConfig.SetDefault("rawLogEmptyGroupRegexp", "^-$")
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	if config.Index == "" {
		return nil, fmt.Errorf("you have to specify the elastic search index")
	}
	if config.Addresses == nil {
		return nil, fmt.Errorf("you have to specify elastic search addresses")
	}
	return New(config, logger)
}

// New returns an instance of ElasticSearchIngester
func New(config elasticSearchIngesterConfig, logger logrus.FieldLogger) (*ElasticSearchIngester, error) {
	client, err := elasticsearch_client.NewClient(
		config.ApiVersion,
		elasticsearch_client.Config{
			Addresses:          config.Addresses,
			Username:           config.Username,
			Password:           config.Password,
			Timeout:            config.Timeout,
			Healtchecks:        config.Healthchecks,
			Sniffing:           config.Sniffing,
			InsecureSkipVerify: config.InsecureSkipVerify,
			ClientCertFile:     config.ClientCertFile,
			ClientKeyFile:      config.ClientKeyFile,
			CaCertFile:         config.CaCertFile,
			Debug:              config.Debug,
		},
		logger,
	)
	if err != nil {
		return nil, err
	}

	rawLogRegexp, err := regexp.Compile(config.RawLogParseRegexp)
	if err != nil {
		return nil, fmt.Errorf("invalid RawLogParseRegexp: %w", err)
	}
	rawEmptyGroupRegexp, err := regexp.Compile(config.RawLogParseRegexp)
	if err != nil {
		return nil, fmt.Errorf("invalid RawLogParseRegexp: %w", err)
	}

	return &ElasticSearchIngester{
		tailer:          newTailer(logger, client, config.Index, config.TimestampField, config.TimestampFormat, config.RawLogField, rawLogRegexp, rawEmptyGroupRegexp, config.Query, config.Timeout, config.MaxBatchSize),
		interval:        config.Interval,
		outputChannel:   make(chan *event.Raw, config.MaxBatchSize),
		shutdownChannel: make(chan struct{}),
		logger:          logger,
		done:            false,
	}, nil
}

func (e *ElasticSearchIngester) RegisterEventProcessingDurationObserver(observer pipeline.EventProcessingDurationObserver) {
	e.observer = observer
}

func (e *ElasticSearchIngester) observeDuration(start time.Time) {
	if e.observer != nil {
		e.observer.Observe(time.Since(start).Seconds())
	}
}

func (e *ElasticSearchIngester) RegisterMetrics(_ prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	toRegister := []prometheus.Collector{
		lastSearchTimestamp,
		elasticsearch_client.ElasticApiCall,
		searchedDocuments,
		missingRawLogField,
		invalidRawLogFormat,
		missingTimestampField,
		invalidTimestampFormat,
		documentsLeftMetric,
	}
	for _, collector := range toRegister {
		if err := wrappedRegistry.Register(collector); err != nil {
			return fmt.Errorf("error registering metric %s: %w", collector, err)
		}
	}
	return nil
}

func (e *ElasticSearchIngester) Done() bool {
	return e.done
}

func (e *ElasticSearchIngester) OutputChannel() chan *event.Raw {
	return e.outputChannel
}

// Run starts to tail the associated file, feeding events to output channel.
func (e *ElasticSearchIngester) Run() {
	done := make(chan bool)
	ctx, cancel := context.WithCancel(context.Background())
	// Goroutine handling shutdown signal
	go func() {
		defer func() {
			<-done
			close(e.outputChannel)
			e.done = true
		}()
		for {
			select {
			case <-e.shutdownChannel:
				cancel()
				return
			}
		}
	}()

	docsChan := e.tailer.run(ctx, e.interval)

	// Main goroutine for reading messages from Kafka
	go func() {
		for doc := range docsChan {
			start := time.Now()
			e.outputChannel <- &event.Raw{
				Metadata:          doc.fields,
				SloClassification: nil,
				Quantity:          1,
			}
			e.observeDuration(start)
		}
		close(done)
	}()
}

func (e *ElasticSearchIngester) Stop() {
	if !e.done {
		e.shutdownChannel <- struct{}{}
	}
}
