package prometheus_ingester

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
)

const (
	metadataValueKey          = "prometheusQueryResult"
	metadataTimestampKey      = "unixTimestamp"
	metadataHistogramMinValue = "prometheusHistogramMinValue"
	metadataHistogramMaxValue = "prometheusHistogramMaxValue"
)

var (
	unsupportedQueryResultType = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "unsupported_query_result_type_total",
		Help: "Total number of Query results with not supported type.",
	}, []string{"result_type"})
	prometheusQueryFail = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "query_fails_total",
		Help: "Total number of Query fails.",
	}, []string{"query_type"})
	prometheusQueryDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "query_duration_seconds",
		Help:    "Duration of queries on the Prometheus API.",
		Buckets: prometheus.ExponentialBuckets(0.05, 3, 5),
	}, []string{"query_type"})
)

type queryType string

const (
	simpleQueryType    queryType = "simple"
	counterQueryType   queryType = "counter_increase"
	histogramQueryType queryType = "histogram_increase"
)

var validQueryTypes = []queryType{
	simpleQueryType,
	counterQueryType,
	histogramQueryType,
}

func validateQueryType(queryType queryType) error {
	for _, validQueryType := range validQueryTypes {
		if queryType == validQueryType {
			return nil
		}
	}
	return fmt.Errorf("unknown query type specified: %s, valid types are %s", queryType, validQueryTypes)
}

type queryOptions struct {
	Query            string
	Interval         time.Duration
	DropLabels       []string
	AdditionalLabels stringmap.StringMap
	Type             queryType
	ResultAsQuantity *bool
}

type PrometheusIngesterConfig struct {
	ApiUrl       string
	RoundTripper http.RoundTripper
	Queries      []queryOptions
	QueryTimeout time.Duration
}

type PrometheusIngester struct {
	queryExecutors  *[]queryExecutor
	queryTimeout    time.Duration
	client          api.Client
	api             v1.API
	shutdownChannel chan struct{}
	outputChannel   chan *event.HttpRequest
	logger          logrus.FieldLogger
	done            bool
}

func (i *PrometheusIngester) String() string {
	return "prometheusIngester"
}

func (i *PrometheusIngester) RegisterMetrics(_ prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	toRegister := []prometheus.Collector{unsupportedQueryResultType, prometheusQueryFail, prometheusQueryDuration}
	for _, metric := range toRegister {
		if err := wrappedRegistry.Register(metric); err != nil {
			return err
		}
	}
	return nil
}

func (i *PrometheusIngester) Stop() {
	close(i.shutdownChannel)
}

func (i *PrometheusIngester) Done() bool {
	return i.done
}

func (i *PrometheusIngester) OutputChannel() chan *event.HttpRequest {
	return i.outputChannel
}

func NewFromViper(viperAppConfig *viper.Viper, logger logrus.FieldLogger) (*PrometheusIngester, error) {
	config := PrometheusIngesterConfig{}
	if err := viperAppConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	if config.QueryTimeout == time.Duration(0) {
		return nil, errors.New("mandatory config field QueryTimeout is missing in PrometheusIngester configuration")
	}
	if config.ApiUrl == "" {
		return nil, errors.New("mandatory config field ApiUrl is missing in PrometheusIngester configuration")
	}
	return New(config, logger)
}

func newTrue() *bool {
	t := true
	return &t
}

func newFalse() *bool {
	f := false
	return &f
}

func New(initConfig PrometheusIngesterConfig, logger logrus.FieldLogger) (*PrometheusIngester, error) {
	var (
		queryExecutors = []queryExecutor{}
		ingester       = PrometheusIngester{}
	)

	client, err := api.NewClient(api.Config{
		Address:      initConfig.ApiUrl,
		RoundTripper: initConfig.RoundTripper,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating Prometheus client: %w", err)
	}

	ingester = PrometheusIngester{
		queryExecutors:  &queryExecutors,
		queryTimeout:    initConfig.QueryTimeout,
		client:          client,
		api:             v1.NewAPI(client),
		outputChannel:   make(chan *event.HttpRequest),
		done:            false,
		shutdownChannel: make(chan struct{}),
		logger:          logger,
	}

	for _, q := range initConfig.Queries {
		if err := validateQueryType(q.Type); err != nil {
			return nil, err
		}
		if q.ResultAsQuantity == nil {
			switch q.Type {
			case counterQueryType:
				q.ResultAsQuantity = newTrue()
			case histogramQueryType:
				q.ResultAsQuantity = newTrue()
			default:
				q.ResultAsQuantity = newFalse()
			}
		}
		queryExecutors = append(
			queryExecutors,
			queryExecutor{
				Query:        q,
				queryTimeout: ingester.queryTimeout,
				eventsChan:   ingester.outputChannel,
				api:          ingester.api,
				logger:       ingester.logger,
				previousResult: queryResult{
					metrics: make(map[model.Fingerprint]model.SamplePair),
				},
			},
		)
	}

	return &ingester, nil
}

func (i *PrometheusIngester) Run() {
	go func() {
		queriesContext, queriesContextCancel := context.WithCancel(context.Background())
		defer func() {
			close(i.outputChannel)
			i.done = true
		}()

		var wg sync.WaitGroup
		// Start all queries
		for _, queryExecutor := range *i.queryExecutors {
			wg.Add(1)
			// declare local scope variable to prevent shadowing by the next iterations
			qe := queryExecutor
			go qe.run(queriesContext, &wg)
		}

		<-i.shutdownChannel
		queriesContextCancel()
		i.logger.Info("received shutdown request, waiting for all current ongoing request to finish")
		wg.Wait()
		i.logger.Info("all done, finishing")
	}()
}
