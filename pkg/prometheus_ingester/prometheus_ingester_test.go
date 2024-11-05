package prometheus_ingester

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/stringmap"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

type MockedRoundTripper struct {
	t                 *testing.T
	result            model.Value
	expectedTimestamp string
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
	vals, err := url.ParseQuery(buf.String())
	assert.NoError(m.t, err)

	if m.expectedTimestamp != "" {
		assert.Equal(m.t, m.expectedTimestamp, vals.Get("time"))
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
		assert.ElementsMatchf(t, tc.eventsProduced, actualEventResult, "Produced events doesn't match expected events", "actual", HttpRequestsToString(actualEventResult))
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

func promTime(ts time.Time, add time.Duration) model.Time {
	return model.Time(ts.Add(add).Unix())
}

func Test_processMetricsIncrease(t *testing.T) {
	type testCase struct {
		name           string
		staleness      time.Duration
		ts             time.Time
		previousResult queryResult
		newResult      []*model.SampleStream
		expectedEvents []*event.Raw
	}

	x := newMetric("x", nil)
	y := newMetric("y", nil)

	ts := model.Time(0)
	testCases := []testCase{
		{
			name:      "continuous increase of 10 in metric x, no increase in metric y",
			ts:        ts.Time().Add(time.Minute * 2),
			staleness: defaultStaleness,
			previousResult: queryResult{
				ts.Time(),
				map[model.Fingerprint]model.SamplePair{
					x.Fingerprint(): {ts, 0},
					y.Fingerprint(): {ts, 10},
				},
			},
			newResult: []*model.SampleStream{
				{
					Metric: x,
					Values: []model.SamplePair{
						{
							Timestamp: ts.Add(time.Minute * 1),
							Value:     model.SampleValue(5),
						},
						{
							Timestamp: ts.Add(time.Minute * 2),
							Value:     model.SampleValue(10),
						},
					},
				},
				{
					Metric: y,
					Values: []model.SamplePair{
						{
							Timestamp: ts.Add(time.Minute * 1),
							Value:     model.SampleValue(10),
						},
						{
							Timestamp: ts.Add(time.Minute * 2),
							Value:     model.SampleValue(10),
						},
					},
				},
			},
			expectedEvents: []*event.Raw{
				{
					Metadata: stringmap.NewFromMetric(x).Merge(stringmap.StringMap{metadataValueKey: "10", metadataTimestampKey: fmt.Sprintf("%d", ts.Add(time.Minute*2).Unix())}),
					Quantity: 10,
				},
			},
		},
		{
			name:      "test staleness with gap between samples, should return no increase",
			staleness: defaultStaleness,
			ts:        ts.Time().Add(time.Minute * 10),
			previousResult: queryResult{
				ts.Time(),
				map[model.Fingerprint]model.SamplePair{
					x.Fingerprint(): {ts, 0},
				},
			},
			newResult: []*model.SampleStream{
				{
					Metric: x,
					Values: []model.SamplePair{
						{
							Timestamp: ts.Add(time.Minute),
							Value:     model.SampleValue(1),
						},
					},
				},
			},
			expectedEvents: []*event.Raw{},
		},
		{
			name:      "test counter reset on x series",
			staleness: defaultStaleness,
			ts:        ts.Time().Add(time.Minute * 4),
			previousResult: queryResult{
				ts.Time(),
				map[model.Fingerprint]model.SamplePair{
					x.Fingerprint(): {ts, 0},
				},
			},
			newResult: []*model.SampleStream{
				{
					Metric: x,
					Values: []model.SamplePair{
						{
							Timestamp: ts.Add(time.Minute * 1),
							Value:     model.SampleValue(5),
						},
						{
							Timestamp: ts.Add(time.Minute * 2),
							Value:     model.SampleValue(1),
						},
						{
							Timestamp: ts.Add(time.Minute * 3),
							Value:     model.SampleValue(5),
						},
					},
				},
			},
			expectedEvents: []*event.Raw{
				{
					Metadata: stringmap.NewFromMetric(x).Merge(stringmap.StringMap{metadataValueKey: "10", metadataTimestampKey: fmt.Sprintf("%d", ts.Add(time.Minute*4).Unix())}),
					Quantity: 10,
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			q := &queryExecutor{
				Query: queryOptions{
					Interval:         time.Second * 20,
					Type:             counterQueryType,
					ResultAsQuantity: newTrue(),
				},
				staleness:      testCase.staleness,
				previousResult: testCase.previousResult,
				eventsChan:     make(chan *event.Raw),
			}

			var generatedEvents []*event.Raw
			done := make(chan struct{})
			go func() {
				for e := range q.eventsChan {
					generatedEvents = append(generatedEvents, e)
				}
				done <- struct{}{}
			}()
			q.processCountersIncrease(testCase.newResult, testCase.ts)
			close(q.eventsChan)
			<-done

			assert.ElementsMatchf(t, testCase.expectedEvents, generatedEvents, "expected events:\n%s\n\nresult:\n%s", testCase.expectedEvents, generatedEvents)
		})
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
	ts := model.Time(0)
	newTs := ts.Add(time.Minute)

	testCases := []testCase{
		// monotonic value of x to 10 (in two samples)
		{
			ts: ts.Time(),
			data: []*model.SampleStream{
				{
					Metric: newMetric("histogram_bucket", stringmap.StringMap{"foo": "bar", "le": "1"}),
					Values: []model.SamplePair{{Timestamp: newTs, Value: model.SampleValue(2)}},
				},
				{
					Metric: newMetric("histogram_bucket", stringmap.StringMap{"foo": "bar", "le": "3"}),
					Values: []model.SamplePair{{Timestamp: newTs, Value: model.SampleValue(8)}},
				},
				{
					Metric: newMetric("histogram_bucket", stringmap.StringMap{"foo": "bar", "le": "6"}),
					Values: []model.SamplePair{{Timestamp: newTs, Value: model.SampleValue(8)}},
				},
				{
					Metric: newMetric("histogram_bucket", stringmap.StringMap{"foo": "bar", "le": "+Inf"}),
					Values: []model.SamplePair{{Timestamp: newTs, Value: model.SampleValue(10)}},
				},
				{
					Metric: newMetric("histogram_bucket", stringmap.StringMap{"foo": "xxx", "le": "0.5"}),
					Values: []model.SamplePair{{Timestamp: newTs, Value: model.SampleValue(2)}},
				},
				{
					Metric: newMetric("histogram_bucket", stringmap.StringMap{"foo": "xxx", "le": "+Inf"}),
					Values: []model.SamplePair{{Timestamp: newTs, Value: model.SampleValue(10)}},
				},
			},
			expectedEvents: []*event.Raw{
				{Metadata: stringmap.StringMap{"__name__": "histogram_bucket", "foo": "bar", "le": "1", metadataTimestampKey: ts.String(), metadataHistogramMinValue: "-Inf", metadataHistogramMaxValue: "1", metadataValueKey: "2"}, Quantity: 2},
				{Metadata: stringmap.StringMap{"__name__": "histogram_bucket", "foo": "bar", "le": "3", metadataTimestampKey: ts.String(), metadataHistogramMinValue: "1", metadataHistogramMaxValue: "3", metadataValueKey: "6"}, Quantity: 6},
				{Metadata: stringmap.StringMap{"__name__": "histogram_bucket", "foo": "bar", "le": "+Inf", metadataTimestampKey: ts.String(), metadataHistogramMinValue: "6", metadataHistogramMaxValue: "+Inf", metadataValueKey: "2"}, Quantity: 2},
				{Metadata: stringmap.StringMap{"__name__": "histogram_bucket", "foo": "xxx", "le": "0.5", metadataTimestampKey: ts.String(), metadataHistogramMinValue: "-Inf", metadataHistogramMaxValue: "0.5", metadataValueKey: "2"}, Quantity: 2},
				{Metadata: stringmap.StringMap{"__name__": "histogram_bucket", "foo": "xxx", "le": "+Inf", metadataTimestampKey: ts.String(), metadataHistogramMinValue: "0.5", metadataHistogramMaxValue: "+Inf", metadataValueKey: "8"}, Quantity: 8},
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
		err := q.processHistogramIncrease(testCase.data, ts.Time())
		assert.NoError(t, err)
		close(q.eventsChan)
		<-done

		assert.ElementsMatchf(t, testCase.expectedEvents, generatedEvents, "expected events:\n%s\n\nresult:\n%s", testCase.expectedEvents, generatedEvents)
	}
}

func Test_httpHeaders_toMap(t *testing.T) {
	var headerValue = "value"
	var headerValue2 = "value2"

	tests := []struct {
		name    string
		data    string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "empty headers",
			data: `httpHeaders: []`,
			want: map[string]string{},
		},
		{
			name: "multiple headers",
			data: `
httpHeaders:
- name: header1
  value: value
- name: header2
  value: value2
`,
			want: map[string]string{"header1": headerValue, "header2": headerValue2},
		},
		{
			name: "headers overwrite",
			data: `
httpHeaders:
- name: header1
  value: value
- name: header2
  value: value2
- name: header1
  value: value2
`,
			want: map[string]string{"header1": headerValue2, "header2": headerValue2},
		},
		{
			name: "validate fail - no header name",
			data: `
httpHeaders:
- value: value
`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var headers struct {
				HttpHeaders httpHeaders
			}
			v := viper.New()
			v.SetConfigType("yaml")
			if err := v.ReadConfig(strings.NewReader(tt.data)); err != nil {
				t.Fatal(err)
			}
			if err := v.UnmarshalExact(&headers); err != nil {
				t.Fatal(err)
			}

			got, err := headers.HttpHeaders.toMap()
			if (err != nil) != tt.wantErr {
				t.Errorf("httpHeader.getValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_httpHeader_getValue(t *testing.T) {
	var headerName = "headerName"
	var headerValue = "headerValue"
	var headerValueFromEnvValue = "headerValueFromEnv"
	var headerValueFromEnvPrefix = "Prefix"
	var envName = "envName"
	var nonExistingEnv = "NON_EXISTING_ENV_NAME"

	if err := os.Unsetenv(nonExistingEnv); err != nil {
		t.Fatal(err)
	}

	if err := os.Setenv(envName, headerValueFromEnvValue); err != nil {
		t.Fatal(err)
	}

	type fields struct {
		Name         string
		ValueFromEnv *httpHeaderValueFromEnv
		Value        *string
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{name: "value", fields: fields{Name: headerName, Value: &headerValue}, want: headerValue},
		{name: "valueFromEnv", fields: fields{Name: headerName, ValueFromEnv: &httpHeaderValueFromEnv{Name: envName}}, want: headerValueFromEnvValue},
		{
			name: "valueFromEnv with prefix",
			fields: fields{
				Name:         headerName,
				ValueFromEnv: &httpHeaderValueFromEnv{Name: envName, ValuePrefix: headerValueFromEnvPrefix}},
			want: headerValueFromEnvPrefix + headerValueFromEnvValue},
		{name: "valueFromEnv non existing env", fields: fields{Name: headerName, ValueFromEnv: &httpHeaderValueFromEnv{Name: nonExistingEnv}}, wantErr: true},
		{name: "valueFromEnv no env name set", fields: fields{Name: headerName, ValueFromEnv: &httpHeaderValueFromEnv{}}, wantErr: true},
		{name: "header name not set", fields: fields{Name: "", Value: &headerValue}, wantErr: true},
		{name: "no value neither valueFromEnv set", fields: fields{Name: headerName}, wantErr: true},
		{name: "value and valueFromEnv", fields: fields{Name: headerName, ValueFromEnv: &httpHeaderValueFromEnv{}, Value: &headerValue}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &httpHeader{
				Name:         tt.fields.Name,
				ValueFromEnv: tt.fields.ValueFromEnv,
				Value:        tt.fields.Value,
			}
			got, err := h.getValue()
			if (err != nil) != tt.wantErr {
				t.Errorf("httpHeader.getValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_queryOffset(t *testing.T) {
	type testCase struct {
		name        string
		queryOpts   queryOptions
		expectError bool
	}

	cases := []testCase{
		{name: "no offset expected", queryOpts: queryOptions{Query: "up", Interval: time.Second, Type: "simple"}},
		{name: "offset expected", queryOpts: queryOptions{Query: "up", Interval: time.Second, Type: "simple", Offset: time.Minute}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := model.Now()
			roundTripper := &MockedRoundTripper{
				t:                 t,
				expectedTimestamp: ts.Add(-tc.queryOpts.Offset).String(),
				result:            &model.Scalar{Value: 1, Timestamp: 0},
			}

			ingester, err := New(PrometheusIngesterConfig{
				RoundTripper: roundTripper,
				QueryTimeout: 400 * time.Millisecond,
				Queries: []queryOptions{
					tc.queryOpts,
				},
			}, logrus.New())
			if err != nil {
				t.Error(err)
				return
			}
			_, _, err = ingester.queryExecutors[0].execute(ts.Time())
			assert.NoError(t, err)
		})
	}
}
