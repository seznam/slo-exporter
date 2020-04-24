package prometheus_ingester

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
)

type queryExecutor struct {
	Query          queryOptions
	queryTimeout   time.Duration
	eventsChan     chan *event.HttpRequest
	logger         logrus.FieldLogger
	api            v1.API
	previousResult queryResult
}

type queryResult struct {
	// timestamp of the query execution
	timestamp time.Time
	// metric: <most recent sample>
	metrics map[model.Fingerprint]model.SamplePair
}

// withRangeSelector returns q.query concatenated with desired range selector
func (q *queryExecutor) withRangeSelector(ts time.Time) string {
	var rangeSelector time.Duration
	if len(q.previousResult.metrics) == 0 {
		rangeSelector = q.Query.Interval
	} else {
		rangeSelector = ts.Sub(q.previousResult.timestamp)
	}
	rangeSelector = rangeSelector.Round(time.Duration(time.Second))
	return q.Query.Query + fmt.Sprintf("[%ds]", int64(rangeSelector.Seconds()))
}

// execute query at provided timestamp ts
func (q *queryExecutor) execute(ts time.Time) (model.Value, error) {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), q.queryTimeout)
	defer cancel()
	var (
		err      error
		result   model.Value
		warnings v1.Warnings
		query    string
	)
	switch q.Query.Type {
	case increaseQueryType:
		query = q.withRangeSelector(ts)
	case simpleQueryType:
		query = q.Query.Query
	default:
		return nil, fmt.Errorf("unknown query type: '%s'", q.Query.Type)
	}
	result, warnings, err = q.api.Query(timeoutCtx, query, ts)
	if len(warnings) > 0 {
		q.logger.WithField("query", query).Warnf("warnings in query execution: %+v", warnings)
	}

	return result, err
}

func (q queryExecutor) run(ctx context.Context, wg *sync.WaitGroup) {
	ticker := time.NewTicker(q.Query.Interval)
	defer ticker.Stop()
	defer wg.Done()

	for {
		select {
		// Wait for the tick
		case <-ticker.C:
			ts := time.Now()
			result, err := q.execute(ts)
			if err != nil {
				prometheusQueryFail.Inc()
				q.logger.WithField("query", q.Query.Query).Errorf("failed querying Prometheus: '%+v'", err)
				continue
			}
			err = q.ProcessResult(result, ts)
			if err != nil {
				q.logger.WithField("query", q.Query.Query).Errorf("failed processing the query result: '%+v'", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (q *queryExecutor) ProcessResult(result model.Value, ts time.Time) error {
	switch q.Query.Type {
	case increaseQueryType:
		switch result.(type) {
		case model.Matrix:
			return q.processMatrixResultAsCounter(result.(model.Matrix), ts)
		default:
			unsupportedQueryResultType.WithLabelValues(result.Type().String()).Inc()
			return fmt.Errorf("unsupported Prometheus value type '%s' for query type '%s'", result.Type().String(), q.Query.Type)
		}

	case simpleQueryType:
		switch result.(type) {
		case model.Matrix:
			return q.processMatrixResult(result.(model.Matrix))
		case model.Vector:
			return q.processVectorResult(result.(model.Vector))
		case *model.Scalar:
			return q.processScalarResult(result.(*model.Scalar))
		default:
			unsupportedQueryResultType.WithLabelValues(result.Type().String()).Inc()
			return fmt.Errorf("unsupported Prometheus value type '%s' for query type '%s'", result.Type().String(), q.Query.Type)
		}
	default:
		return fmt.Errorf("unknown query type: %s", q.Query.Type)
	}
}

func (q *queryExecutor) emitEvent(ts time.Time, quantity float64, result float64, metric model.Metric) {
	e := &event.HttpRequest{
		Metadata: stringmap.NewFromMetric(metric),
		Quantity: quantity,
	}
	e.Metadata = e.Metadata.Merge(stringmap.StringMap{
		metadataValueKey:     fmt.Sprintf("%g", result),
		metadataTimestampKey: fmt.Sprintf("%d", ts.Unix()),
	})
	e.Metadata = e.Metadata.Without(q.Query.DropLabels).Merge(q.Query.AdditionalLabels)
	q.eventsChan <- e
}

// metricIncrease calculates increase between two samples
func metricIncrease(previousSample, sample model.SamplePair) float64 {
	if sample.Value < previousSample.Value {
		// counter reset
		return float64(sample.Value)
	}
	return float64(sample.Value) - float64(previousSample.Value)
}

func (q *queryExecutor) processMatrixResultAsCounter(matrix model.Matrix, ts time.Time) error {
	currentResult := queryResult{
		timestamp: ts,
		metrics:   make(map[model.Fingerprint]model.SamplePair),
	}

	// iterate over individual metrics
	for _, singleMetricSampleStream := range matrix {
		var (
			previousSample model.SamplePair
			ok             bool
			increase       float64
			sample         model.SamplePair
		)

		metricKey := singleMetricSampleStream.Metric.Fingerprint()

		previousSample, ok = q.previousResult.metrics[metricKey]
		if !ok {
			// we have no previous result available, use first of the fetched samples
			previousSample = singleMetricSampleStream.Values[0]
		}
		// iterate over samples of given metric
		for _, sample = range singleMetricSampleStream.Values {
			increase += metricIncrease(previousSample, sample)
			previousSample = sample
		}
		currentResult.metrics[metricKey] = sample
		if increase == 0 {
			continue
		}
		q.emitEvent(ts, increase, increase, singleMetricSampleStream.Metric)
	}
	q.previousResult = currentResult
	return nil
}

func (q *queryExecutor) processMatrixResult(matrix model.Matrix) error {
	for _, sampleStream := range matrix {
		for _, sample := range sampleStream.Values {
			q.emitEvent(sample.Timestamp.Time(), 1, float64(sample.Value), sampleStream.Metric)
		}
	}
	return nil
}

func (q *queryExecutor) processVectorResult(resultVector model.Vector) error {
	for _, sample := range resultVector {
		q.emitEvent(sample.Timestamp.Time(), 1, float64(sample.Value), sample.Metric)
	}
	return nil
}

func (q *queryExecutor) processScalarResult(scalar *model.Scalar) error {
	q.emitEvent(scalar.Timestamp.Time(), 1, float64(scalar.Value), make(model.Metric))
	return nil
}
