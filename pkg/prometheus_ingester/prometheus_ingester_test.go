package prometheus_ingester

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
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

var metricObject = newMetric("test_metric", map[string]string{"job": "kubernetes", "locality": "nagano"})
var metricStringMap = stringmap.NewFromMetric(metricObject)

func newMetric(name model.LabelValue, labels map[string]string) model.Metric {
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

func HttpRequestsToString(rawResults []*event.Raw) []string {
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
	eventsProduced   []*event.Raw
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
				Query:            "1",
				Type:             simpleQueryType,
				ResultAsQuantity: newFalse(),
			},
			eventsProduced: []*event.Raw{
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
				Type:             simpleQueryType,
				ResultAsQuantity: newFalse(),
			}, eventsProduced: []*event.Raw{
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
				Type:             simpleQueryType,
				ResultAsQuantity: newFalse(),
			}, eventsProduced: []*event.Raw{
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
			eventsChan: make(chan *event.Raw),
		}
		go func() {
			err := q.ProcessResult(tc.prometheusResult, time.Now())
			assert.NoError(t, err)
			close(q.eventsChan)
		}()
		var actualEventResult []*event.Raw
		for newEvent := range q.eventsChan {
			actualEventResult = append(actualEventResult, newEvent)
		}

		// Prepare the string interpretation of the actual results
		assert.ElementsMatchf(t, tc.eventsProduced, actualEventResult, "Produced events doesnt match expected events", "actual", HttpRequestsToString(actualEventResult))
	}
}

type labelAddOrDropTestCase struct {
	prometheusResult model.Value
	query            queryOptions
	eventsProduced   []*event.Raw
}

func Test_Add_Or_Drop_Labels(t *testing.T) {
	m := stringmap.NewFromMetric(newMetric("test_metric", nil))
	testCases := []labelAddOrDropTestCase{
		{
			// Tests addition of non-existent label
			query: queryOptions{
				AdditionalLabels: map[string]string{"a": "1"},
				Type:             simpleQueryType,
				ResultAsQuantity: newFalse(),
			},
			prometheusResult: model.Vector{
				{
					Metric:    metricObject,
					Timestamp: model.Time(1),
					Value:     model.SampleValue(1),
				},
			},
			eventsProduced: []*event.Raw{
				{
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
				ResultAsQuantity: newFalse(),
			},
			prometheusResult: model.Vector{
				{
					Metric:    metricObject,
					Timestamp: model.Time(1),
					Value:     model.SampleValue(1),
				},
			},
			eventsProduced: []*event.Raw{
				{
					Metadata: m.Merge(stringmap.StringMap{"job": "kubernetes", "locality": "osaka", "__name__": "test_metric", metadataValueKey: "1", metadataTimestampKey: "0"}),
					Quantity: 1,
				},
			},
		},
		{
			// Tests dropping existing label
			query: queryOptions{
				DropLabels:       []string{"job"},
				Type:             simpleQueryType,
				ResultAsQuantity: newFalse(),
			},
			prometheusResult: model.Vector{
				{
					Metric:    metricObject,
					Timestamp: model.Time(1),
					Value:     model.SampleValue(1),
				},
			},
			eventsProduced: []*event.Raw{
				{
					Metadata: m.Merge(stringmap.StringMap{"locality": "nagano", "__name__": "test_metric", metadataValueKey: "1", metadataTimestampKey: "0"}),
					Quantity: 1,
				},
			},
		},
		{
			// Tests dropping non-existing label
			query: queryOptions{
				DropLabels:       []string{"a"},
				Type:             simpleQueryType,
				ResultAsQuantity: newFalse(),
			},
			prometheusResult: model.Vector{
				{
					Metric:    metricObject,
					Timestamp: model.Time(1),
					Value:     model.SampleValue(1),
				},
			},
			eventsProduced: []*event.Raw{
				{
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
				Type:             simpleQueryType,
				ResultAsQuantity: newFalse(),
			},
			prometheusResult: model.Vector{
				{
					Metric:    metricObject,
					Timestamp: model.Time(1),
					Value:     model.SampleValue(1),
				},
			},
			eventsProduced: []*event.Raw{
				{
					Metadata: m.Merge(stringmap.StringMap{"job": "openshift", "locality": "nagano", "__name__": "test_metric", metadataValueKey: "1", metadataTimestampKey: "0"}),
					Quantity: 1,
				},
			},
		},
	}

	for _, tc := range testCases {
		q := queryExecutor{
			Query:      tc.query,
			eventsChan: make(chan *event.Raw),
		}
		go func() {
			err := q.ProcessResult(tc.prometheusResult, time.Now())
			assert.NoError(t, err)
			close(q.eventsChan)
		}()

		var actualEventResult []*event.Raw
		for newEvent := range q.eventsChan {
			actualEventResult = append(actualEventResult, newEvent)
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
				Query:            "1",
				Interval:         interval,
				Type:             simpleQueryType,
				ResultAsQuantity: newFalse(),
			},
		},
	}, logrus.New())

	if err != nil {
		t.Error(err)
		return
	}

	var genEvents []*event.Raw
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
		{
			previous: model.SamplePair{Value: 10},
			current:  model.SamplePair{Value: 11},
			result:   1,
		},
		{
			previous: model.SamplePair{Value: 10},
			current:  model.SamplePair{Value: 5},
			result:   5,
		},
	}
	for _, c := range testCases {
		assert.Equal(t, c.result, increaseBetweenSamples(c.previous, c.current))
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
	interval := 20 * time.Second
	testCases := []testCase{
		{
			&queryExecutor{
				Query: queryOptions{
					Query:            query,
					Type:             counterQueryType,
					Interval:         interval,
					ResultAsQuantity: newTrue(),
				},
				previousResult: queryResult{},
			},
			ts,
			query + fmt.Sprintf("[%s]", interval),
		},
		{
			&queryExecutor{
				Query: queryOptions{
					Query:            query,
					Type:             counterQueryType,
					Interval:         interval,
					ResultAsQuantity: newTrue(),
				},
				previousResult: queryResult{timestamp: ts.Add(time.Hour * -1),
					metrics: map[model.Fingerprint]model.SamplePair{
						model.Fingerprint(0): {model.Time(0), 0},
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

func Test_processMetricsIncrease(t *testing.T) {
	type testCase struct {
		q              *queryExecutor
		ts             time.Time
		result         []*model.SampleStream // == type Matrix
		expectedEvents []*event.Raw
	}

	x := newMetric("x", nil)
	y := newMetric("y", nil)

	ts := time.Now()
	q := &queryExecutor{
		Query: queryOptions{
			Interval:         time.Second * 20,
			Type:             counterQueryType,
			ResultAsQuantity: newTrue(),
		},
		previousResult: queryResult{
			ts.Add(time.Hour * -1),
			map[model.Fingerprint]model.SamplePair{
				x.Fingerprint(): {0, 0},
				y.Fingerprint(): {0, 10},
			},
		},
	}

	testCases := []testCase{
		// monotonic value of x to 10 (in two samples)
		{
			q:  q,
			ts: ts,
			result: []*model.SampleStream{
				{
					Metric: x,
					Values: []model.SamplePair{
						{
							Timestamp: model.Time(ts.Add(time.Hour * -2).Unix()),
							Value:     model.SampleValue(5),
						},
						{
							Timestamp: model.Time(ts.Add(time.Hour * -1).Unix()),
							Value:     model.SampleValue(10),
						},
					},
				},
			},
			expectedEvents: []*event.Raw{
				{
					Metadata: stringmap.NewFromMetric(x).Merge(stringmap.StringMap{metadataValueKey: "10", metadataTimestampKey: fmt.Sprintf("%d", ts.Unix())}),
					Quantity: 10,
				},
			},
		},
		// value with reset on x, monotonic value for y
		{
			q:  q,
			ts: ts,
			result: []*model.SampleStream{
				{
					Metric: x,
					Values: []model.SamplePair{
						{
							Timestamp: model.Time(ts.Add(time.Hour * -3).Unix()),
							Value:     model.SampleValue(5),
						},
						{
							Timestamp: model.Time(ts.Add(time.Hour * -2).Unix()),
							Value:     model.SampleValue(1),
						},
						{
							Timestamp: model.Time(ts.Add(time.Hour * -1).Unix()),
							Value:     model.SampleValue(5),
						},
					},
				},
				{
					Metric: y,
					Values: []model.SamplePair{
						{
							Timestamp: model.Time(ts.Add(time.Hour * -2).Unix()),
							Value:     model.SampleValue(1),
						},
						{
							Timestamp: model.Time(ts.Add(time.Hour * -1).Unix()),
							Value:     model.SampleValue(2),
						},
					},
				},
			},
			expectedEvents: []*event.Raw{
				{
					Metadata: stringmap.NewFromMetric(x).Merge(stringmap.StringMap{metadataValueKey: "10", metadataTimestampKey: fmt.Sprintf("%d", ts.Unix())}),
					Quantity: 10,
				},
				{
					Metadata: stringmap.NewFromMetric(y).Merge(stringmap.StringMap{metadataValueKey: "1", metadataTimestampKey: fmt.Sprintf("%d", ts.Unix())}),
					Quantity: 1,
				},
			},
		},
	}

	for _, testCase := range testCases {
		testCase.q.eventsChan = make(chan *event.Raw)

		var generatedEvents []*event.Raw
		done := make(chan struct{})
		go func() {
			for e := range testCase.q.eventsChan {
				generatedEvents = append(generatedEvents, e)
			}
			done <- struct{}{}
		}()
		testCase.q.processCountersIncrease(testCase.result, testCase.ts)
		close(testCase.q.eventsChan)
		<-done

		assert.ElementsMatchf(t, testCase.expectedEvents, generatedEvents, "expected events:\n%s\n\nresult:\n%s", testCase.expectedEvents, generatedEvents)
	}
}

func resultFromSampleStreams(ts time.Time, streams []*model.SampleStream, value int) queryResult {
	res := queryResult{
		timestamp: ts,
		metrics:   map[model.Fingerprint]model.SamplePair{},
	}
	for _, stream := range streams {
		res.metrics[stream.Metric.Fingerprint()] = model.SamplePair{Value: model.SampleValue(value)}
	}
	return res
}

func Test_processHistogramIncrease(t *testing.T) {
	type testCase struct {
		ts             time.Time
		data           []*model.SampleStream // == type Matrix
		expectedEvents []*event.Raw
	}
	ts := time.Now()
	tsStr := strconv.Itoa(int(ts.Unix()))

	testCases := []testCase{
		// monotonic value of x to 10 (in two samples)
		{
			ts: ts,
			data: []*model.SampleStream{
				{
					Metric: newMetric("histogram_bucket", stringmap.StringMap{"foo": "bar", "le": "1"}),
					Values: []model.SamplePair{{Timestamp: 10, Value: model.SampleValue(2)}},
				},
				{
					Metric: newMetric("histogram_bucket", stringmap.StringMap{"foo": "bar", "le": "3"}),
					Values: []model.SamplePair{{Timestamp: 10, Value: model.SampleValue(8)}},
				},
				{
					Metric: newMetric("histogram_bucket", stringmap.StringMap{"foo": "bar", "le": "6"}),
					Values: []model.SamplePair{{Timestamp: 10, Value: model.SampleValue(8)}},
				},
				{
					Metric: newMetric("histogram_bucket", stringmap.StringMap{"foo": "bar", "le": "+Inf"}),
					Values: []model.SamplePair{{Timestamp: 10, Value: model.SampleValue(10)}},
				},
			},
			expectedEvents: []*event.Raw{
				{Metadata: stringmap.StringMap{"__name__": "histogram_bucket", "foo": "bar", "le": "1", metadataTimestampKey: tsStr, metadataHistogramMinValue: "-Inf", metadataHistogramMaxValue: "1", metadataValueKey: "2"}, Quantity: 2},
				{Metadata: stringmap.StringMap{"__name__": "histogram_bucket", "foo": "bar", "le": "3", metadataTimestampKey: tsStr, metadataHistogramMinValue: "1", metadataHistogramMaxValue: "3", metadataValueKey: "6"}, Quantity: 6},
				{Metadata: stringmap.StringMap{"__name__": "histogram_bucket", "foo": "bar", "le": "+Inf", metadataTimestampKey: tsStr, metadataHistogramMinValue: "6", metadataHistogramMaxValue: "+Inf", metadataValueKey: "2"}, Quantity: 2},
			},
		},
	}

	for _, testCase := range testCases {
		q := &queryExecutor{
			eventsChan: make(chan *event.Raw),
			Query: queryOptions{
				Type:             histogramQueryType,
				ResultAsQuantity: newTrue(),
			},
			previousResult: resultFromSampleStreams(testCase.ts, testCase.data, 0),
		}

		var generatedEvents []*event.Raw
		done := make(chan struct{})
		go func() {
			for e := range q.eventsChan {
				generatedEvents = append(generatedEvents, e)
			}
			done <- struct{}{}
		}()
		err := q.processHistogramIncrease(testCase.data, ts)
		assert.NoError(t, err)
		close(q.eventsChan)
		<-done

		assert.ElementsMatchf(t, testCase.expectedEvents, generatedEvents, "expected events:\n%s\n\nresult:\n%s", testCase.expectedEvents, generatedEvents)
	}
}
