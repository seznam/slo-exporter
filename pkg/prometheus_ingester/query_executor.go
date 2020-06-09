package prometheus_ingester

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
)

type queryExecutor struct {
	Query             queryOptions
	queryTimeout      time.Duration
	eventsChan        chan *event.Raw
	logger            logrus.FieldLogger
	api               v1.API
	previousResult    queryResult
	previousResultMtx sync.RWMutex
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
	rangeSelector = rangeSelector.Round(time.Second)
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
	case histogramQueryType:
		query = q.withRangeSelector(ts)
	case counterQueryType:
		query = q.withRangeSelector(ts)
	case simpleQueryType:
		query = q.Query.Query
	default:
		return nil, fmt.Errorf("unknown query type: '%s'", q.Query.Type)
	}
	apiQueryTimer := prometheus.NewTimer(prometheusQueryDuration.WithLabelValues(string(q.Query.Type)))
	result, warnings, err = q.api.Query(timeoutCtx, query, ts)
	apiQueryTimer.ObserveDuration()
	if len(warnings) > 0 {
		q.logger.WithField("query", query).Warnf("warnings in query execution: %+v", warnings)
	}

	return result, err
}

func (q *queryExecutor) run(ctx context.Context, wg *sync.WaitGroup) {
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
				prometheusQueryFail.WithLabelValues(string(q.Query.Type)).Inc()
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
	case histogramQueryType:
		switch result.(type) {
		case model.Matrix:
			if err := q.processHistogramIncrease(result.(model.Matrix), ts); err != nil {
				return err
			}
			return nil
		default:
			unsupportedQueryResultType.WithLabelValues(result.Type().String()).Inc()
			return fmt.Errorf("unsupported Prometheus value type '%s' for query type '%s'", result.Type().String(), q.Query.Type)
		}
	case counterQueryType:
		switch result.(type) {
		case model.Matrix:
			q.processCountersIncrease(result.(model.Matrix), ts)
			return nil
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

func (q *queryExecutor) emitEvent(ts time.Time, result float64, metadata stringmap.StringMap) {
	var quantity float64 = 1
	if *q.Query.ResultAsQuantity {
		quantity = result
	}
	if quantity == 0 {
		return
	}
	e := &event.Raw{
		Metadata: metadata,
		Quantity: quantity,
	}
	e.Metadata = e.Metadata.Merge(stringmap.StringMap{
		metadataValueKey:     fmt.Sprintf("%g", result),
		metadataTimestampKey: fmt.Sprintf("%d", ts.Unix()),
	})
	e.Metadata = e.Metadata.Without(q.Query.DropLabels).Merge(q.Query.AdditionalLabels)
	q.eventsChan <- e
}

// increaseBetweenSamples calculates value between two samples
func increaseBetweenSamples(previousSample, sample model.SamplePair) float64 {
	if sample.Value < previousSample.Value {
		// counter reset
		return float64(sample.Value)
	}
	return float64(sample.Value) - float64(previousSample.Value)
}

type metricIncrease struct {
	occurred time.Time
	value    float64
	metric   model.Metric
}

func (q *queryExecutor) processHistogramIncrease(matrix model.Matrix, ts time.Time) error {
	bucketIncreases := make(map[float64]metricIncrease)
	var (
		buckets []float64
		errors  error
	)
	for increase := range q.processMatrixResultAsIncrease(matrix, ts) {
		bucket, ok := increase.metric["le"]
		if !ok {
			errors = multierror.Append(errors, fmt.Errorf("metric %s missing `le` bucket", increase.metric))
			continue
		}
		bucketValue, err := strconv.ParseFloat(string(bucket), 64)
		if err != nil {
			errors = multierror.Append(errors, fmt.Errorf("histogram metric `%s` has invalid `le` label", increase.metric))
			continue
		}
		bucketIncreases[bucketValue] = increase
		buckets = append(buckets, bucketValue)
	}
	if errors != nil {
		return errors
	}
	sort.Float64s(buckets)
	var previousBucketKey float64 = math.Inf(-1)
	// Iterate over all buckets and report event with quantity equal to difference of it's and preceding bucket increase.
	for _, key := range buckets {
		maxValue := key
		minValue := previousBucketKey
		intervalIncrease := bucketIncreases[key].value - bucketIncreases[previousBucketKey].value
		q.emitEvent(
			ts,
			intervalIncrease,
			stringmap.NewFromMetric(bucketIncreases[key].metric).Merge(stringmap.StringMap{
				metadataHistogramMinValue: fmt.Sprintf("%g", minValue),
				metadataHistogramMaxValue: fmt.Sprintf("%g", maxValue),
			}))
		previousBucketKey = key
	}
	return nil
}

func (q *queryExecutor) processCountersIncrease(matrix model.Matrix, ts time.Time) {
	for increase := range q.processMatrixResultAsIncrease(matrix, ts) {
		q.emitEvent(increase.occurred, increase.value, stringmap.NewFromMetric(increase.metric))
	}
}

func (q *queryExecutor) processMatrixResultAsIncrease(matrix model.Matrix, ts time.Time) chan metricIncrease {
	outChan := make(chan metricIncrease)
	go func() {
		defer close(outChan)
		currentResult := queryResult{
			timestamp: ts,
			metrics:   make(map[model.Fingerprint]model.SamplePair),
		}
		q.previousResultMtx.Lock()
		defer q.previousResultMtx.Unlock()
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
			// iterate over samples of given newMetric
			for _, sample = range singleMetricSampleStream.Values {
				increase += increaseBetweenSamples(previousSample, sample)
				previousSample = sample
			}
			currentResult.metrics[metricKey] = sample
			outChan <- metricIncrease{
				occurred: ts,
				value:    increase,
				metric:   singleMetricSampleStream.Metric,
			}
		}
		q.previousResult = currentResult
	}()
	return outChan
}

func (q *queryExecutor) processMatrixResult(matrix model.Matrix) error {
	for _, sampleStream := range matrix {
		for _, sample := range sampleStream.Values {
			q.emitEvent(sample.Timestamp.Time(), float64(sample.Value), stringmap.NewFromMetric(sampleStream.Metric))
		}
	}
	return nil
}

func (q *queryExecutor) processVectorResult(resultVector model.Vector) error {
	for _, sample := range resultVector {
		q.emitEvent(sample.Timestamp.Time(), float64(sample.Value), stringmap.NewFromMetric(sample.Metric))
	}
	return nil
}

func (q *queryExecutor) processScalarResult(scalar *model.Scalar) error {
	q.emitEvent(scalar.Timestamp.Time(), float64(scalar.Value), make(stringmap.StringMap, 0))
	return nil
}
