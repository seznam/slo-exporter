package prometheus_ingester

import (
	"context"
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"net/http"
	"sync"
	"time"
)

const (
	component = "prometheus_ingester"
)

var (
	log                        *logrus.Entry
	unsupportedQueryResultType = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "slo_exporter",
		Subsystem: "prometheus_ingester",
		Name:      "unsupported_query_result_type_total",
		Help:      "Total number of Query results with not supported type.",
	}, []string{"result_type"})
	prometheusQueryFail = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "slo_exporter",
		Subsystem: "prometheus_ingester",
		Name:      "prometheus_query_fails_total",
		Help:      "Total number of Query fails.",
	})
)

func init() {

	log = logrus.WithFields(logrus.Fields{"component": component})
	prometheus.MustRegister(unsupportedQueryResultType)
	prometheus.MustRegister(prometheusQueryFail)
}

type queryExecutor struct {
	Query        queryOptions
	queryTimeout time.Duration
	api          v1.API
	eventsChan   chan<- *event.PrometheusQueryResult
}

func (q *queryExecutor) executeQuery() (model.Value, error) {
	timeoutCtx, release := context.WithTimeout(context.Background(), q.queryTimeout)
	result, warnings, err := q.api.Query(timeoutCtx, q.Query.Query, time.Now())
	// releases resources if slowOperation completes before timeout elapses
	release()
	if len(warnings) > 0 {
		log.WithField("query", q.Query.Query).Warnf("warnings in query execution: %+v", warnings)
	}
	return result, err
}

func ProcessResult(result model.Value, query queryOptions) ([]*event.PrometheusQueryResult, error) {
	switch result.(type) {
	case model.Matrix:
		return processMatrixResult(result.(model.Matrix), query.AdditionalLabels, query.DropLabels), nil
	case model.Vector:
		return processVectorResult(result.(model.Vector), query.AdditionalLabels, query.DropLabels), nil
	case *model.Scalar:
		return []*event.PrometheusQueryResult{processScalarResult(result.(*model.Scalar), query.AdditionalLabels)}, nil
	default:
		return nil, errors.New("unsupported Prometheus value type " + result.Type().String())
	}
}

func (q queryExecutor) run(ctx context.Context, wg *sync.WaitGroup) {
	ticker := time.NewTicker(q.Query.Interval)
	defer ticker.Stop()
	for {
		select {
		// Wait for the tick
		case <-ticker.C:

			result, err := q.executeQuery()
			if err != nil {
				prometheusQueryFail.Inc()
				log.WithField("query", q.Query.Query).Errorf("failed querying Prometheus: '%+v'", err)
				continue
			}

			events, err := ProcessResult(result, q.Query)
			if err != nil {
				unsupportedQueryResultType.WithLabelValues(result.Type().String()).Inc()
				log.WithField("query", q.Query).Errorf("unsupported value %+v", result.Type())
				continue
			}

			for _, queryResult := range events {
				log.WithField("result", result).Debug("pushing the result to the blocking eventsChan")
				q.eventsChan <- queryResult
			}

		case <-ctx.Done():
			wg.Done()
			return
		}
	}

}

type PrometheusIngesterConfig struct {
	ApiUrl       string
	RoundTripper http.RoundTripper
	Queries      []queryOptions
	QueryTimeout time.Duration
}

type PrometheusIngester struct {
	queries      []queryOptions
	queryTimeout time.Duration
	client       api.Client
	api          v1.API
}

func NewFromViper(viperAppConfig *viper.Viper) (*PrometheusIngester, error) {
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
	return New(config)
}

func New(initConfig PrometheusIngesterConfig) (*PrometheusIngester, error) {

	client, err := api.NewClient(api.Config{
		Address:      initConfig.ApiUrl,
		RoundTripper: initConfig.RoundTripper,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating Prometheus client: %w", err)
	}

	return &PrometheusIngester{
		queries:      initConfig.Queries,
		queryTimeout: initConfig.QueryTimeout,
		client:       client,
		api:          v1.NewAPI(client),
	}, nil
}

func (i *PrometheusIngester) Run(ctx context.Context, eventsChan chan<- *event.PrometheusQueryResult) {
	go func() {
		defer close(eventsChan)
		var wg sync.WaitGroup

		// Create tickers for each of the query interval
		for _, query := range i.queries {

			wg.Add(1)
			go queryExecutor{
				Query:        query,
				queryTimeout: i.queryTimeout,
				api:          i.api,
				eventsChan:   eventsChan,
			}.run(ctx, &wg)
		}

		wg.Wait()
		log.Info("input channel closed, finishing")
	}()
}

func addAndDropLabels(metric model.Metric, labelsToAdd stringmap.StringMap, labelsToDrop []string) stringmap.StringMap {
	return stringmap.NewFromMetric(metric).Without(labelsToDrop).Merge(labelsToAdd)
}

func processMatrixResult(matrix model.Matrix, labelsToAdd stringmap.StringMap, labelsToDrop []string) []*event.PrometheusQueryResult {
	var results []*event.PrometheusQueryResult
	for _, sampleStream := range matrix {
		for _, sample := range sampleStream.Values {
			results = append(results, &event.PrometheusQueryResult{
				Value:     float64(sample.Value),
				Timestamp: sample.Timestamp.Time(),
				Labels:    addAndDropLabels(sampleStream.Metric, labelsToAdd, labelsToDrop),
			})
		}
	}
	return results
}

func processVectorResult(resultVector model.Vector, labelsToAdd stringmap.StringMap, labelsToDrop []string) []*event.PrometheusQueryResult {
	var results []*event.PrometheusQueryResult
	for _, sample := range resultVector {
		results = append(results, &event.PrometheusQueryResult{
			Value:     float64(sample.Value),
			Timestamp: sample.Timestamp.Time(),
			Labels:    addAndDropLabels(sample.Metric, labelsToAdd, labelsToDrop),
		})
	}
	return results
}

func processScalarResult(scalar *model.Scalar, labelsToAdd stringmap.StringMap) *event.PrometheusQueryResult {
	return &event.PrometheusQueryResult{
		Value:     float64(scalar.Value),
		Timestamp: scalar.Timestamp.Time(),
		// Scalar does not have any metrics, so we just append the additional ones
		Labels: labelsToAdd,
	}
}
