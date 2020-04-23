package prometheus_ingester

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
)

type MockedRoundTripper struct {
	t      *testing.T
	result model.Value
}

func (m *MockedRoundTripper) resultFabricator() string {
	resultBytes, err := json.Marshal(m.result)
	if err != nil {
		m.t.Fatalf("failed marshalling the result")
	}
	return `{
		"status": "success",
		"data": {
			"resultType": "` + m.result.Type().String() + `",
			"result": ` + string(resultBytes) + `
		}
	}`
}

func (m *MockedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(req.Body); err != nil {
		m.t.Error(err)
		return nil, err
	}

	response := m.resultFabricator()

	return &http.Response{
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewBufferString(response)),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}, nil
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
					Metadata: metricStringMap.Merge(stringmap.StringMap{metadataValueKey: "1", metadataTimestampKey: "0"}),
					Quantity: 1,
				},
				{
					Metadata: metricStringMap.Merge(stringmap.StringMap{metadataValueKey: "2", metadataTimestampKey: "0"}),
					Quantity: 1,
				},
				{
					Metadata: metricStringMap.Merge(stringmap.StringMap{metadataValueKey: "3", metadataTimestampKey: "0"}),
					Quantity: 1,
				},
				{
					Metadata: metricStringMap.Merge(stringmap.StringMap{metadataValueKey: "4", metadataTimestampKey: "0"}),
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
					Metadata: metricStringMap.Merge(stringmap.StringMap{metadataValueKey: "1", metadataTimestampKey: "0"}),
					Quantity: 1,
				},
				{
					Metadata: metricStringMap.Merge(stringmap.StringMap{metadataValueKey: "2", metadataTimestampKey: "0"}),
					Quantity: 1,
				},
			},
		},
		{
			// Test of Scalar ingestion
			prometheusResult: &model.Scalar{
				Timestamp: model.Time(1),
				Value:     model.SampleValue(1),
			},
			query: queryOptions{
				Type: simpleQueryType,
			}, eventsProduced: []*event.HttpRequest{
				{
					Metadata: stringmap.NewFromMetric(make(model.Metric)).Merge(stringmap.StringMap{metadataValueKey: "1", metadataTimestampKey: "0"}),
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
	m := stringmap.NewFromMetric(metric("test_metric", nil))
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
				&event.HttpRequest{
					Metadata: m.Merge(stringmap.StringMap{"a": "1", "job": "kubernetes", "locality": "nagano", "__name__": "test_metric", metadataValueKey: "1", metadataTimestampKey: "0"}),
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
				&event.HttpRequest{
					Metadata: m.Merge(stringmap.StringMap{"job": "kubernetes", "locality": "osaka", "__name__": "test_metric", metadataValueKey: "1", metadataTimestampKey: "0"}),
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
				&event.HttpRequest{
					Metadata: m.Merge(stringmap.StringMap{"locality": "nagano", "__name__": "test_metric", metadataValueKey: "1", metadataTimestampKey: "0"}),
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
				&event.HttpRequest{
					Metadata: m.Merge(stringmap.StringMap{"job": "kubernetes", "locality": "nagano", "__name__": "test_metric", metadataValueKey: "1", metadataTimestampKey: "0"}),
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
				&event.HttpRequest{
					Metadata: m.Merge(stringmap.StringMap{"job": "openshift", "locality": "nagano", "__name__": "test_metric", metadataValueKey: "1", metadataTimestampKey: "0"}),
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

// Tests roughly that configured query interval generates expected number of events for given period
func TestIngesterScalar_Interval_run(t *testing.T) {
	interval := 900 * time.Millisecond
	runFor := 2000 * time.Millisecond
	expectedEventsCount := 2

	roundTripper := &MockedRoundTripper{
		t: t,
		result: &model.Scalar{
			Value:     1,
			Timestamp: 0,
		},
	}

	ingester, err := New(PrometheusIngesterConfig{
		RoundTripper: roundTripper,
		QueryTimeout: 400 * time.Millisecond,
		Queries: []queryOptions{
			{
				Query: "1",
				// The interval is considered for query 1 to run 4 times before the query 2 runs
				Interval: interval,
				Type:     simpleQueryType,
			},
		},
	}, logrus.New())

	if err != nil {
		t.Error(err)
		return
	}

	genEvents := []*event.HttpRequest{}
	done := make(chan struct{})
	go func() {
		for e := range ingester.outputChannel {
			genEvents = append(genEvents, e)
		}
		done <- struct{}{}
	}()

	// The whole test should take no longer than two seconds (we have only 500ms of intentionally blocking time)
	ctx, cancelFunc := context.WithTimeout(context.Background(), runFor)
	defer cancelFunc()
	go func() {
		<-ctx.Done()
		ingester.shutdownChannel <- struct{}{}
	}()
	ingester.Run()

	<-done
	assert.Equal(t, expectedEventsCount, len(genEvents), "Running query every '%v' for '%v' was expected to generate %v events", interval, runFor, expectedEventsCount)
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
					Metadata: stringmap.NewFromMetric(x).Merge(stringmap.StringMap{metadataValueKey: "10", metadataTimestampKey: fmt.Sprintf("%d", ts.Unix())}),
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
					Metadata: stringmap.NewFromMetric(x).Merge(stringmap.StringMap{metadataValueKey: "10", metadataTimestampKey: fmt.Sprintf("%d", ts.Unix())}),
					Quantity: 10,
				},
				&event.HttpRequest{
					Metadata: stringmap.NewFromMetric(y).Merge(stringmap.StringMap{metadataValueKey: "2", metadataTimestampKey: fmt.Sprintf("%d", ts.Unix())}),
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
			assert.Equal(t, testCase.expectedEvents[i], generatedEvents[i], "Newly created event #%d does not match", i)
		}
	}
}
