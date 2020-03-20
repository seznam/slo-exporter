package prometheus_ingester

import (
	"context"
	"errors"
	"fmt"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"sync"
	"time"
)

type queryExecutor struct {
	Query        queryOptions
	queryTimeout time.Duration
	api          v1.API
	eventsChan   chan *event.HttpRequest
	logger       *logrus.Entry
}

func (q *queryExecutor) executeQuery() (model.Value, error) {
	timeoutCtx, release := context.WithTimeout(context.Background(), q.queryTimeout)
	result, warnings, err := q.api.Query(timeoutCtx, q.Query.Query, time.Now())
	// releases resources if slowOperation completes before timeout elapses
	release()
	if len(warnings) > 0 {
		q.logger.WithField("query", q.Query.Query).Warnf("warnings in query execution: %+v", warnings)
	}
	return result, err
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
				q.logger.WithField("query", q.Query.Query).Errorf("failed querying Prometheus: '%+v'", err)
				continue
			}

			events, err := ProcessResult(result, q.Query)
			if err != nil {
				unsupportedQueryResultType.WithLabelValues(result.Type().String()).Inc()
				q.logger.WithField("query", q.Query).Errorf("unsupported value %+v", result.Type())
				continue
			}

			for _, queryResult := range events {
				q.logger.WithField("result", result).Debug("pushing the result to the blocking eventsChan")
				q.eventsChan <- queryResult
			}

		case <-ctx.Done():
			wg.Done()
			return
		}
	}

}

func ProcessResult(result model.Value, query queryOptions) ([]*event.HttpRequest, error) {
	switch result.(type) {
	case model.Matrix:
		return processMatrixResult(result.(model.Matrix), query.AdditionalLabels, query.DropLabels), nil
	case model.Vector:
		return processVectorResult(result.(model.Vector), query.AdditionalLabels, query.DropLabels), nil
	case *model.Scalar:
		return []*event.HttpRequest{processScalarResult(result.(*model.Scalar), query.AdditionalLabels)}, nil
	default:
		return nil, errors.New("unsupported Prometheus value type " + result.Type().String())
	}
}

func addAndDropLabels(metric model.Metric, labelsToAdd stringmap.StringMap, labelsToDrop []string) stringmap.StringMap {
	return stringmap.NewFromMetric(metric).Without(labelsToDrop).Merge(labelsToAdd)
}

func addResultToMetadata(metadata stringmap.StringMap, result float64, occured time.Time) stringmap.StringMap {
	return metadata.Merge(stringmap.StringMap{
		metadataValueKey:     fmt.Sprintf("%g", result),
		metadataTimestampKey: fmt.Sprintf("%d", occured.Unix()),
	})
}

func processMatrixResult(matrix model.Matrix, labelsToAdd stringmap.StringMap, labelsToDrop []string) []*event.HttpRequest {
	var results []*event.HttpRequest
	for _, sampleStream := range matrix {
		for _, sample := range sampleStream.Values {
			results = append(results, &event.HttpRequest{
				Metadata: addResultToMetadata(addAndDropLabels(sampleStream.Metric, labelsToAdd, labelsToDrop), float64(sample.Value), sample.Timestamp.Time()),
			})
		}
	}
	return results
}

func processVectorResult(resultVector model.Vector, labelsToAdd stringmap.StringMap, labelsToDrop []string) []*event.HttpRequest {
	var results []*event.HttpRequest
	for _, sample := range resultVector {
		results = append(results, &event.HttpRequest{
			Metadata: addResultToMetadata(addAndDropLabels(sample.Metric, labelsToAdd, labelsToDrop), float64(sample.Value), sample.Timestamp.Time()),
		})
	}
	return results
}

func processScalarResult(scalar *model.Scalar, labelsToAdd stringmap.StringMap) *event.HttpRequest {
	return &event.HttpRequest{
		Metadata: addResultToMetadata(labelsToAdd, float64(scalar.Value), scalar.Timestamp.Time()),
	}
}
