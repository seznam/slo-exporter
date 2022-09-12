package prometheus_ingester

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/prometheus/client_golang/api"
	"github.com/stretchr/testify/assert"
)

type testHttpHeaderRoundTripper struct {
	expectedHeaders http.Header
	t               *testing.T
}

func (rt *testHttpHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	assert.Equal(rt.t, rt.expectedHeaders, req.Header)

	return &http.Response{
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewBufferString("ahoj")),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}, nil
}

func testHttpHeaderRoundTripperMapToHeaders(data map[string]string) http.Header {
	h := http.Header{}
	for k, v := range data {
		h.Set(k, v)
	}
	return h
}

func Test_httpHeadersRoundTripper_RoundTrip(t *testing.T) {

	type fields struct {
		headers map[string]string
		rt      http.RoundTripper
	}
	type args struct {
		r *http.Request
	}
	tests := []struct {
		name            string
		initialHeaders  map[string]string
		appendedHeaders map[string]string
		expectedHeaders map[string]string
	}{
		{
			name:            "have header and append header",
			initialHeaders:  map[string]string{"header1": "value1"},
			appendedHeaders: map[string]string{"appendedHeader": "appendedHeaderValue"},
			expectedHeaders: map[string]string{"appendedHeader": "appendedHeaderValue", "header1": "value1"},
		},
		{
			name:            "only append header",
			initialHeaders:  map[string]string{},
			appendedHeaders: map[string]string{"appendedHeader": "appendedHeaderValue"},
			expectedHeaders: map[string]string{"appendedHeader": "appendedHeaderValue"},
		},
		{
			name:            "have header and not append header",
			initialHeaders:  map[string]string{"header1": "value1"},
			appendedHeaders: map[string]string{},
			expectedHeaders: map[string]string{"header1": "value1"},
		},
		{
			name:            "empty headers",
			initialHeaders:  map[string]string{},
			appendedHeaders: map[string]string{},
			expectedHeaders: map[string]string{},
		},
		{
			name:            "append multiple headers",
			initialHeaders:  map[string]string{},
			appendedHeaders: map[string]string{"appendedHeader1": "appendedHeaderValue1", "appendedHeader2": "appendedHeaderValue2"},
			expectedHeaders: map[string]string{"appendedHeader1": "appendedHeaderValue1", "appendedHeader2": "appendedHeaderValue2"},
		},
		{
			name:            "overwrite header",
			initialHeaders:  map[string]string{"header": "value"},
			appendedHeaders: map[string]string{"header": "newValue"},
			expectedHeaders: map[string]string{"header": "newValue"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := httpHeadersRoundTripper{
				headers: tt.appendedHeaders,
				rt: &testHttpHeaderRoundTripper{
					expectedHeaders: testHttpHeaderRoundTripperMapToHeaders(tt.expectedHeaders),
					t:               t,
				},
			}

			c, err := api.NewClient(api.Config{
				Address:      "http://fake-address",
				RoundTripper: rt,
			})
			if err != nil {
				t.Fatal(err)
			}
			r := &http.Request{
				URL:    &url.URL{Scheme: "http", Host: "fake-host", Path: "/"},
				Header: testHttpHeaderRoundTripperMapToHeaders(tt.initialHeaders),
			}
			if _, _, err = c.Do(context.Background(), r); err != nil {
				t.Fatal(err)
			}

		})
	}
}
