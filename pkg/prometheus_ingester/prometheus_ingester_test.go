package prometheus_ingester

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"testing"
	"time"
)

type MockedRoundTripper struct {
	t              *testing.T
	v1QueryHandler func(*testing.T, float64, int64) string
}

var requestExtractor = regexp.MustCompile(`query=(?P<Query>[^&]+)&time=(?P<Timestamp>[^&]+)`)

var metricObject = mockedMetric()
var metricStringMap = stringmap.NewFromMetric(metricObject)

func mockedMetric() model.Metric {
	obj := model.Metric{}
	obj["job"] = "kubernetes"
	obj["locality"] = "nagano"
	return obj
}

func HttpRequestsToString(rawResults []*event.HttpRequest) []string {
	stringResults := make([]string, len(rawResults))
	for i, rawResult := range rawResults {
		marshalledBytes, err := json.Marshal(*rawResult)
		if err != nil {
			marshalledBytes = []byte{}
		}
		stringResults[i] = string(marshalledBytes)
	}
	return stringResults
}

type modelTypeIngestTestCase struct {
	prometheusResult model.Value
	query            queryOptions
	eventsProduced   []*event.HttpRequest
}

func Test_Ingests_Various_ModelTypes(t *testing.T) {
	testCases := []modelTypeIngestTestCase{
		{
			// Test of Matrix ingestion
			prometheusResult: model.Matrix{
				{
					Metric: metricObject,
					Values: []model.SamplePair{
						{
							Timestamp: model.Time(1),
							Value:     model.SampleValue(1),
						},
						{
							Timestamp: model.Time(1),
							Value:     model.SampleValue(2),
						},
					},
				},
				{
					Metric: metricObject,
					Values: []model.SamplePair{
						{
							Timestamp: model.Time(1),
							Value:     model.SampleValue(3),
						},
						{
							Timestamp: model.Time(1),
							Value:     model.SampleValue(4),
						},
					},
				},
			},
			query: queryOptions{},
			eventsProduced: []*event.HttpRequest{
				{
					Metadata: addResultToMetadata(metricStringMap, 1, time.Unix(0, 1000000)),
				},
				{
					Metadata: addResultToMetadata(metricStringMap, 2, time.Unix(0, 1000000)),
				},
				{
					Metadata: addResultToMetadata(metricStringMap, 3, time.Unix(0, 1000000)),
				},
				{
					Metadata: addResultToMetadata(metricStringMap, 4, time.Unix(0, 1000000)),
				},
			},
		},
		{
			// Test of Vector ingestion
			prometheusResult: model.Vector{
				{
					Metric:    metricObject,
					Timestamp: model.Time(1),
					Value:     model.SampleValue(1),
				},
				{
					Metric:    metricObject,
					Timestamp: model.Time(1),
					Value:     model.SampleValue(2),
				},
			},
			query: queryOptions{},
			eventsProduced: []*event.HttpRequest{
				{
					Metadata: addResultToMetadata(metricStringMap, 1, time.Unix(0, 1000000)),
				},
				{
					Metadata: addResultToMetadata(metricStringMap, 2, time.Unix(0, 1000000)),
				},
			},
		},
		{
			// Test of Vector ingestion
			prometheusResult: &model.Scalar{
				Timestamp: model.Time(1),
				Value:     model.SampleValue(1),
			},
			query: queryOptions{},
			eventsProduced: []*event.HttpRequest{
				{
					Metadata: addResultToMetadata(stringmap.StringMap{}, 1, time.Unix(0, 1000000)),
				},
			},
		},
	}

	for _, tc := range testCases {
		actualEventResult, err := ProcessResult(tc.prometheusResult, tc.query)
		if err != nil {
			t.Errorf("failed processing result. %+v", err)
		}

		// Prepare the string interpretation of the actual results
		assert.ElementsMatchf(t, tc.eventsProduced, actualEventResult, "Produced events doesnt match expected events", "actual", HttpRequestsToString(actualEventResult))
	}
}

type labelAddOrDropTestCase struct {
	prometheusResult model.Value
	query            queryOptions
	eventsProduced   []*event.HttpRequest
}

func Test_Add_Or_Drop_Labels(t *testing.T) {
	testCases := []labelAddOrDropTestCase{
		{
			// Tests addition of non-existent label
			query: queryOptions{
				AdditionalLabels: map[string]string{"a": "1"},
			},
			prometheusResult: model.Vector{
				{
					Metric:    metricObject,
					Timestamp: model.Time(1),
					Value:     model.SampleValue(1),
				},
			},
			eventsProduced: []*event.HttpRequest{
				{
					Metadata: addResultToMetadata(stringmap.StringMap{"a": "1", "job": "kubernetes", "locality": "nagano",}, 1, time.Unix(0, 1000000)),
				},
			},
		},
		{
			// Tests addition of existent label
			query: queryOptions{
				AdditionalLabels: map[string]string{"locality": "osaka"},
			},
			prometheusResult: model.Vector{
				{
					Metric:    metricObject,
					Timestamp: model.Time(1),
					Value:     model.SampleValue(1),
				},
			},
			eventsProduced: []*event.HttpRequest{
				{
					Metadata: addResultToMetadata(stringmap.StringMap{"job": "kubernetes", "locality": "osaka",}, 1, time.Unix(0, 1000000)),
				},
			},
		},
		{
			// Tests dropping existing label
			query: queryOptions{
				DropLabels: []string{"job"},
			},
			prometheusResult: model.Vector{
				{
					Metric:    metricObject,
					Timestamp: model.Time(1),
					Value:     model.SampleValue(1),
				},
			},
			eventsProduced: []*event.HttpRequest{
				{
					Metadata: addResultToMetadata(stringmap.StringMap{"locality": "nagano"}, 1, time.Unix(0, 1000000)),
				},
			},
		},
		{
			// Tests dropping non-existing label
			query: queryOptions{
				DropLabels: []string{"a"},
			},
			prometheusResult: model.Vector{
				{
					Metric:    metricObject,
					Timestamp: model.Time(1),
					Value:     model.SampleValue(1),
				},
			},
			eventsProduced: []*event.HttpRequest{
				{
					Metadata: addResultToMetadata(stringmap.StringMap{"job": "kubernetes", "locality": "nagano"}, 1, time.Unix(0, 1000000)),
				},
			},
		},
		{
			// Tests that dropping the label being added does not drop the added label :shrug:
			query: queryOptions{
				DropLabels: []string{"job"},
				AdditionalLabels: map[string]string{
					"job": "openshift",
				},
			},
			prometheusResult: model.Vector{
				{
					Metric:    metricObject,
					Timestamp: model.Time(1),
					Value:     model.SampleValue(1),
				},
			},
			eventsProduced: []*event.HttpRequest{
				{
					Metadata: addResultToMetadata(stringmap.StringMap{"job": "openshift", "locality": "nagano"}, 1, time.Unix(0, 1000000)),
				},
			},
		},
	}

	for _, tc := range testCases {
		actualEventResult, err := ProcessResult(tc.prometheusResult, tc.query)
		if err != nil {
			t.Errorf("failed processing result. %+v", err)
		}

		// Prepare the string interpretation of the actual results
		assert.ElementsMatchf(t, tc.eventsProduced, actualEventResult, "Produced events doesnt match expected events", "actual", HttpRequestsToString(actualEventResult))
	}
}

func (m *MockedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(req.Body); err != nil {
		m.t.Error(err)
		return nil, err
	}

	request := buf.String()
	var response = ""
	if req.Method == http.MethodPost && req.URL.Path == "/api/v1/query" {
		match := requestExtractor.FindStringSubmatch(request)
		if len(match) < 3 {
			err := "the Query does not contain query & time! Or we have bad regexp, please check this out"
			m.t.Error(err)
			return nil, errors.New(err)
		}
		requestQuery, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			m.t.Fatalf("Failed to parse float from query '%s'", match[1])
		}
		requestTimestamp, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			m.t.Fatalf("Failed to parse int from timestamp '%s'", match[2])
		}

		response = m.v1QueryHandler(m.t, requestQuery, int64(requestTimestamp*1000000000))
	} else {
		return nil, errors.New("unknown endpoint called " + req.URL.Path)
	}

	return &http.Response{
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewBufferString(response)),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}, nil
}

func vectorResultFabricator(t *testing.T, requestQuery float64, requestTimestamp int64) string {
	result := model.Vector{
		&model.Sample{
			Metric:    metricObject,
			Value:     model.SampleValue(requestQuery),
			Timestamp: model.TimeFromUnixNano(requestTimestamp),
		},
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed marshalling the result")
	}
	return `{
		"status": "success",
		"data": {
			"resultType": "` + result.Type().String() + `",
			"result": ` + string(resultBytes) + `
		}
	}`
}

// Tests whole functionality of assigning a Query and getting a result inside a channel
// Also asserts that the interval works correctly
func TestIngesterScalar_Interval_run(t *testing.T) {

	roundTripper := &MockedRoundTripper{
		t:              t,
		v1QueryHandler: vectorResultFabricator,
	}

	ingester, err := New(PrometheusIngesterConfig{
		RoundTripper: roundTripper,
		QueryTimeout: 400 * time.Millisecond,
		Queries: []queryOptions{
			{
				Query: "1",
				// The interval is considered for query 1 to run 4 times before the query 2 runs
				Interval: 120 * time.Millisecond,
			},
			{
				Query: "2",
				// After query 1 ran 4 times, run query 2 once, completing the tests, ensuring the interval works properly
				Interval: 500 * time.Millisecond,
			},
		},
	}, logrus.New())

	if err != nil {
		t.Error(err)
		return
	}

	// The whole test should take no longer than two seconds (we have only 500ms of intentionally blocking time)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelFunc()

	ingester.Run()

	var (
		queryOneCount = 0
		queryTwoCount = 0
	)

	// We read only 5 events written to the producer channel to validate that the first query was executed 4 times
	// and the second query only one time
	for i := 0; i < 5; i++ {
		select {
		case result := <-ingester.OutputChannel():
			switch result.Metadata[metadataValueKey] {
			case "1":
				queryOneCount++
			case "2":
				queryTwoCount++
			default:
				t.Errorf("Unknown value was written to the Prometheus producer channel: %s", result.Metadata[metadataValueKey])
			}
		case <-ctx.Done():
			t.Errorf("Failed to process 5 events in one second. Actually processed %d/5 events", queryOneCount+queryTwoCount)
			return
		}
	}

	cancelFunc()
	if queryTwoCount != 1 {
		t.Errorf("Query 2 should complete exactly once out of five reads. It actually completed %d times", queryTwoCount)
	}

	if queryOneCount != 4 {
		t.Errorf("Query 2 should complete exactly four out of five reads. It actually completed %d times", queryOneCount)
	}
}
