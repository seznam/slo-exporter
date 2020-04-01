package prometheus_ingester

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

var metricObject = metric("test_metric", map[string]string{"job": "kubernetes", "locality": "nagano"})
var metricStringMap = stringmap.NewFromMetric(metricObject)

func metric(name model.LabelValue, labels map[string]string) model.Metric {
	m := map[model.LabelName]model.LabelValue{
		"__name__": name,
	}
	if labels != nil {
		for k, v := range labels {
			m[model.LabelName(k)] = model.LabelValue(v)
		}
	}
	return m
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
			query: queryOptions{
				Query: "1",
				Type:  simpleQueryType,
			},
			eventsProduced: []*event.HttpRequest{
				{
					Metadata: addResultToMetadata(metricStringMap, 1, time.Unix(0, 1000000)),
					Quantity: 1,
				},
				{
					Metadata: addResultToMetadata(metricStringMap, 2, time.Unix(0, 1000000)),
					Quantity: 1,
				},
				{
					Metadata: addResultToMetadata(metricStringMap, 3, time.Unix(0, 1000000)),
					Quantity: 1,
				},
				{
					Metadata: addResultToMetadata(metricStringMap, 4, time.Unix(0, 1000000)),
					Quantity: 1,
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
			query: queryOptions{
				Type: simpleQueryType,
			}, eventsProduced: []*event.HttpRequest{
				{
					Metadata: addResultToMetadata(metricStringMap, 1, time.Unix(0, 1000000)),
					Quantity: 1,
				},
				{
					Metadata: addResultToMetadata(metricStringMap, 2, time.Unix(0, 1000000)),
					Quantity: 1,
				},
			},
		},
		{
			// Test of Vector ingestion
			prometheusResult: &model.Scalar{
				Timestamp: model.Time(1),
				Value:     model.SampleValue(1),
			},
			query: queryOptions{
				Type: simpleQueryType,
			}, eventsProduced: []*event.HttpRequest{
				{
					Metadata: addResultToMetadata(stringmap.StringMap{}, 1, time.Unix(0, 1000000)),
					Quantity: 1,
				},
			},
		},
	}

	for _, tc := range testCases {
		q := queryExecutor{
			Query:      tc.query,
			eventsChan: make(chan *event.HttpRequest),
		}
		go func() {
			q.ProcessResult(tc.prometheusResult, time.Now())
			close(q.eventsChan)
		}()

		actualEventResult := []*event.HttpRequest{}
		for event := range q.eventsChan {
			actualEventResult = append(actualEventResult, event)
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
				Type:             simpleQueryType,
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
					Metadata: addResultToMetadata(stringmap.StringMap{"a": "1", "job": "kubernetes", "locality": "nagano", "__name__": "test_metric"}, 1, time.Unix(0, 1000000)),
					Quantity: 1,
				},
			},
		},
		{
			// Tests addition of existent label
			query: queryOptions{
				AdditionalLabels: map[string]string{"locality": "osaka"},
				Type:             simpleQueryType,
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
					Metadata: addResultToMetadata(stringmap.StringMap{"job": "kubernetes", "locality": "osaka", "__name__": "test_metric"}, 1, time.Unix(0, 1000000)),
					Quantity: 1,
				},
			},
		},
		{
			// Tests dropping existing label
			query: queryOptions{
				DropLabels: []string{"job"},
				Type:       simpleQueryType,
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
					Metadata: addResultToMetadata(stringmap.StringMap{"locality": "nagano", "__name__": "test_metric"}, 1, time.Unix(0, 1000000)),
					Quantity: 1,
				},
			},
		},
		{
			// Tests dropping non-existing label
			query: queryOptions{
				DropLabels: []string{"a"},
				Type:       simpleQueryType,
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
					Metadata: addResultToMetadata(stringmap.StringMap{"job": "kubernetes", "locality": "nagano", "__name__": "test_metric"}, 1, time.Unix(0, 1000000)),
					Quantity: 1,
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
				Type: simpleQueryType,
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
					Metadata: addResultToMetadata(stringmap.StringMap{"job": "openshift", "locality": "nagano", "__name__": "test_metric"}, 1, time.Unix(0, 1000000)),
					Quantity: 1,
				},
			},
		},
	}

	for _, tc := range testCases {
		q := queryExecutor{
			Query:      tc.query,
			eventsChan: make(chan *event.HttpRequest),
		}
		go func() {
			q.ProcessResult(tc.prometheusResult, time.Now())
			close(q.eventsChan)
		}()

		actualEventResult := []*event.HttpRequest{}
		for event := range q.eventsChan {
			actualEventResult = append(actualEventResult, event)
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
				Type:     simpleQueryType,
			},
			{
				Query: "2",
				// After query 1 ran 4 times, run query 2 once, completing the tests, ensuring the interval works properly
				Interval: 500 * time.Millisecond,
				Type:     simpleQueryType,
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

func TestGetMetricIncrease(t *testing.T) {
	type testCase struct {
		previous model.SamplePair
		current  model.SamplePair
		result   float64
	}
	testCases := []testCase{
		testCase{
			previous: model.SamplePair{Value: 10},
			current:  model.SamplePair{Value: 11},
			result:   1,
		},
		testCase{
			previous: model.SamplePair{Value: 10},
			current:  model.SamplePair{Value: 5},
			result:   5,
		},
	}
	for _, c := range testCases {
		assert.Equal(t, c.result, metricIncrease(c.previous, c.current))
	}
}

func TestGetQueryWithRangeSelector(t *testing.T) {
	type testCase struct {
		query         *queryExecutor
		ts            time.Time
		expectedQuery string
	}

	query := "up{}"
	ts := time.Now()
	interval := time.Duration(20 * time.Second)
	testCases := []testCase{
		testCase{
			&queryExecutor{
				Query: queryOptions{
					Query:    query,
					Type:     increaseQueryType,
					Interval: interval,
				},
				previousResult: queryResult{},
			},
			ts,
			query + fmt.Sprintf("[%s]", interval),
		},
		testCase{
			&queryExecutor{
				Query: queryOptions{
					Query:    query,
					Type:     increaseQueryType,
					Interval: interval,
				},
				previousResult: queryResult{timestamp: ts.Add(time.Hour * -1),
					metrics: map[model.Fingerprint]model.SamplePair{
						model.Fingerprint(0): model.SamplePair{model.Time(0), 0},
					},
				},
			},
			ts,
			query + fmt.Sprintf("[%s]", "3600s"),
		},
	}

	for _, testCase := range testCases {
		result := testCase.query.withRangeSelector(testCase.ts)
		assert.Equal(t, testCase.expectedQuery, result)
	}

}

func Test_processMatrixResultAsCounter(t *testing.T) {
	type testCase struct {
		q              queryExecutor
		ts             time.Time
		result         []*model.SampleStream // == type Matrix
		expectedEvents []*event.HttpRequest
	}

	x := metric("x", nil)
	y := metric("y", nil)

	ts := time.Now()
	q := queryExecutor{
		Query: queryOptions{
			Interval: time.Second * 20,
			Type:     increaseQueryType,
		},
		previousResult: queryResult{
			ts.Add(time.Hour * -1),
			map[model.Fingerprint]model.SamplePair{
				x.Fingerprint(): model.SamplePair{0, 0},
				y.Fingerprint(): model.SamplePair{0, 10},
			},
		},
	}

	testCases := []testCase{
		// monotonic increase of x to 10 (in two samples)
		testCase{
			q:  q,
			ts: ts,
			result: []*model.SampleStream{
				&model.SampleStream{
					Metric: x,
					Values: []model.SamplePair{
						model.SamplePair{
							Timestamp: model.Time(ts.Add(time.Hour * -2).Unix()),
							Value:     model.SampleValue(5),
						},
						model.SamplePair{
							Timestamp: model.Time(ts.Add(time.Hour * -1).Unix()),
							Value:     model.SampleValue(10),
						},
					},
				},
			},
			expectedEvents: []*event.HttpRequest{
				&event.HttpRequest{
					Metadata: addResultToMetadata(q.addAndDropLabels(x), 10, ts),
					Quantity: 10,
				},
			},
		},
		// increase with reset on x, monotonic increase for y
		testCase{
			q:  q,
			ts: ts,
			result: []*model.SampleStream{
				&model.SampleStream{
					Metric: x,
					Values: []model.SamplePair{
						model.SamplePair{
							Timestamp: model.Time(ts.Add(time.Hour * -3).Unix()),
							Value:     model.SampleValue(5),
						},
						model.SamplePair{
							Timestamp: model.Time(ts.Add(time.Hour * -2).Unix()),
							Value:     model.SampleValue(1),
						},
						model.SamplePair{
							Timestamp: model.Time(ts.Add(time.Hour * -1).Unix()),
							Value:     model.SampleValue(5),
						},
					},
				},
				&model.SampleStream{
					Metric: y,
					Values: []model.SamplePair{
						model.SamplePair{
							Timestamp: model.Time(ts.Add(time.Hour * -2).Unix()),
							Value:     model.SampleValue(1),
						},
						model.SamplePair{
							Timestamp: model.Time(ts.Add(time.Hour * -1).Unix()),
							Value:     model.SampleValue(2),
						},
					},
				},
			},
			expectedEvents: []*event.HttpRequest{
				&event.HttpRequest{
					Metadata: addResultToMetadata(q.addAndDropLabels(x), 10, ts),
					Quantity: 10,
				},
				&event.HttpRequest{
					Metadata: addResultToMetadata(q.addAndDropLabels(y), 2, ts),
					Quantity: 2,
				},
			},
		},
	}

	for _, testCase := range testCases {
		testCase.q.eventsChan = make(chan *event.HttpRequest)

		generatedEvents := []*event.HttpRequest{}
		done := make(chan struct{})
		go func() {
			for e := range testCase.q.eventsChan {
				generatedEvents = append(generatedEvents, e)
			}
			done <- struct{}{}
		}()
		testCase.q.processMatrixResultAsCounter(testCase.result, testCase.ts)
		close(testCase.q.eventsChan)
		<-done

		assert.Equal(t, len(testCase.expectedEvents), len(generatedEvents), "Result processing did not generated expected number of events")
		for i, _ := range generatedEvents {
			assert.Equal(t, testCase.expectedEvents[i], generatedEvents[i], "Newly created event #%s does not match", i)
		}
	}
}
