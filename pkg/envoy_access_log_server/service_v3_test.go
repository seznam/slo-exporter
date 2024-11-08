package envoy_access_log_server

import (
	"testing"

	v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_data_accesslog_v3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/seznam/slo-exporter/pkg/stringmap"
)

var logger = &logrus.Logger{}

func Test_exportCommonPropertiesV3(t *testing.T) {
	tests := []struct {
		description    string
		input          *envoy_data_accesslog_v3.AccessLogCommon
		expectedResult stringmap.StringMap
	}{
		{
			description: "Empty AccessLogCommon properties",
			input:       &envoy_data_accesslog_v3.AccessLogCommon{},
			expectedResult: stringmap.StringMap{
				"sampleRate": "0",
			},
		},
		{
			description: "Non-empty AccessLogCommon properties",
			input: &envoy_data_accesslog_v3.AccessLogCommon{
				SampleRate: 1,
				DownstreamDirectRemoteAddress: &v3.Address{Address: &v3.Address_SocketAddress{SocketAddress: &v3.SocketAddress{
					Address:       "127.0.0.1",
					PortSpecifier: &v3.SocketAddress_PortValue{PortValue: 44848},
				}}},
				DownstreamRemoteAddress: &v3.Address{Address: &v3.Address_SocketAddress{SocketAddress: &v3.SocketAddress{
					Address:       "127.0.0.1",
					PortSpecifier: &v3.SocketAddress_PortValue{PortValue: 46058},
				}}},
				DownstreamLocalAddress: &v3.Address{Address: &v3.Address_SocketAddress{SocketAddress: &v3.SocketAddress{
					Address:       "127.0.0.1",
					PortSpecifier: &v3.SocketAddress_PortValue{PortValue: 8080},
				}}},
				TlsProperties: &envoy_data_accesslog_v3.TLSProperties{
					TlsVersion:     4, // TLSv1_3
					TlsCipherSuite: &wrappers.UInt32Value{Value: 4865},
				},
				StartTime: &timestamp.Timestamp{
					Seconds: 1608647248,
					Nanos:   741408000,
				},
				TimeToLastRxByte: &duration.Duration{
					Nanos: 101859,
				},
				TimeToFirstUpstreamTxByte: &duration.Duration{
					Nanos: 490187312,
				},
				TimeToLastUpstreamTxByte: &duration.Duration{
					Nanos: 490187312,
				},
				TimeToFirstUpstreamRxByte: &duration.Duration{
					Nanos: 463920708,
				},
				TimeToLastUpstreamRxByte: &duration.Duration{
					Nanos: 490187312,
				},
				TimeToFirstDownstreamTxByte: &duration.Duration{
					Nanos: 490791797,
				},
				TimeToLastDownstreamTxByte: &duration.Duration{
					Nanos: 490791800,
				},
				UpstreamRemoteAddress: &v3.Address{Address: &v3.Address_SocketAddress{SocketAddress: &v3.SocketAddress{
					Address:       "77.75.75.172",
					PortSpecifier: &v3.SocketAddress_PortValue{PortValue: 443},
				}}},
				UpstreamLocalAddress: &v3.Address{Address: &v3.Address_SocketAddress{SocketAddress: &v3.SocketAddress{
					Address:       "10.0.116.130",
					PortSpecifier: &v3.SocketAddress_PortValue{PortValue: 48734},
				}}},
				UpstreamCluster: "service_seznam_cz",
				ResponseFlags: &envoy_data_accesslog_v3.ResponseFlags{
					ResponseFromCacheFilter: true,
				},
				Metadata:                       nil,
				UpstreamTransportFailureReason: "foo",
				RouteName:                      "foo",

				FilterStateObjects: nil,
			},
			expectedResult: stringmap.StringMap{
				"downstreamDirectRemoteAddress":  "127.0.0.1",
				"downstreamDirectRemotePort":     "44848",
				"downstreamLocalAddress":         "127.0.0.1",
				"downstreamLocalPort":            "8080",
				"downstreamRemoteAddress":        "127.0.0.1",
				"downstreamRemotePort":           "46058",
				"routeName":                      "foo",
				"sampleRate":                     "1",
				"startTime":                      "2020-12-22T14:27:28Z",
				"timeToFirstDownstreamTxByte":    "490791797ns",
				"timeToFirstUpstreamRxByte":      "463920708ns",
				"timeToFirstUpstreamTxByte":      "490187312ns",
				"timeToLastDownstreamTxByte":     "490791800ns",
				"timeToLastRxByte":               "101859ns",
				"timeToLastUpstreamRxByte":       "490187312ns",
				"timeToLastUpstreamTxByte":       "490187312ns",
				"upstreamCluster":                "service_seznam_cz",
				"upstreamLocalAddress":           "10.0.116.130",
				"upstreamLocalPort":              "48734",
				"upstreamRemoteAddress":          "77.75.75.172",
				"upstreamRemotePort":             "443",
				"upstreamTransportFailureReason": "foo",
			},
		},
	}
	for _, tt := range tests {
		f := func(*testing.T) {
			output := envoyV3AccessLogEntryCommonPropertiesToStringMap(logger, tt.input)
			for k, v := range tt.expectedResult {
				assert.Equal(t, v, output[k], "expected %s=%s", k, v)
			}
		}
		t.Run(tt.description, f)
	}
}

func Test_exportHTTPRequestPropertiesV3(t *testing.T) {
	tests := []struct {
		description    string
		input          *envoy_data_accesslog_v3.HTTPRequestProperties
		expectedResult stringmap.StringMap
	}{
		{
			description: "Empty request properties",
			input:       &envoy_data_accesslog_v3.HTTPRequestProperties{},
			expectedResult: stringmap.StringMap{
				"requestMethod":       "METHOD_UNSPECIFIED",
				"requestHeadersBytes": "0",
				"requestBodyBytes":    "0",
			},
		},
		{
			description: "Non-empty request properties",
			input: &envoy_data_accesslog_v3.HTTPRequestProperties{
				RequestMethod:       1, // GET
				Scheme:              "https",
				Authority:           "www.seznam.cz",
				Path:                "/",
				UserAgent:           "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:83.0) Gecko/20100101 Firefox/83.0",
				Referer:             "xxx",
				ForwardedFor:        "xxx",
				RequestId:           "f1d027b9-8b7f-448a-bbe2-1a3151801af1",
				OriginalPath:        "",
				RequestHeadersBytes: 932,
				RequestBodyBytes:    0,
				RequestHeaders:      map[string]string{"foo": "bar"},
			},
			expectedResult: stringmap.StringMap{
				"requestMethod":       "GET",
				"scheme":              "https",
				"authority":           "www.seznam.cz",
				"path":                "/",
				"userAgent":           "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:83.0) Gecko/20100101 Firefox/83.0",
				"referer":             "xxx",
				"forwardedFor":        "xxx",
				"requestId":           "f1d027b9-8b7f-448a-bbe2-1a3151801af1",
				"requestHeadersBytes": "932",
				"requestBodyBytes":    "0",
				"http_foo":            "bar",
			},
		},
	}

	for _, test := range tests {
		f := func(*testing.T) {
			for k, v := range test.expectedResult {
				assert.Equal(t, v, envoyV3AccessLogEntryHTTPRequestPropertiesToStringMap(logger, test.input)[k], "expected %s=%s", k, v)
			}
		}
		t.Run(test.description, f)
	}
}

func Test_exportHTTPResponsePropertiesV3(t *testing.T) {
	tests := []struct {
		description string
		input       *envoy_data_accesslog_v3.HTTPResponseProperties
		result      stringmap.StringMap
	}{
		{
			description: "Empty response properties",
			input:       &envoy_data_accesslog_v3.HTTPResponseProperties{},
			result: stringmap.StringMap{
				"responseCode":         "0",
				"responseHeadersBytes": "0",
				"responseBodyBytes":    "0",
				"responseCodeDetails":  "",
			},
		},
		{
			description: "Non-empty response properties",
			input: &envoy_data_accesslog_v3.HTTPResponseProperties{
				ResponseCode:         &wrappers.UInt32Value{Value: 200},
				ResponseHeadersBytes: 166,
				ResponseBodyBytes:    74400,
				ResponseHeaders:      map[string]string{"slo-domain": "userportal", "slo-class": "critical"},
				ResponseTrailers:     map[string]string{"slo-availability-expectedResult": "success"},
				ResponseCodeDetails:  "via_upstream",
			},
			result: stringmap.StringMap{
				"responseCode":                                 "200",
				"responseHeadersBytes":                         "166",
				"responseBodyBytes":                            "74400",
				"sent_http_slo-class":                          "critical",
				"sent_http_slo-domain":                         "userportal",
				"sent_trailer_slo-availability-expectedResult": "success",
				"responseCodeDetails":                          "via_upstream",
			},
		},
	}

	for _, test := range tests {
		f := func(*testing.T) {
			assert.Equal(t, test.result, envoyV3AccessLogEntryHTTPResponsePropertiesToStringMap(logger, test.input))
		}
		t.Run(test.description, f)
	}
}

// Make sure that all HTTP log entry properties are extracted. Just a minimal test case is present,
// full extent extraction of request, response and common properties are to be tested separately within:
// Test_exportHTTPResponsePropertiesV3
// Test_exportHTTPRequestPropertiesV3
// Test_exportCommonPropertiesV3
func Test_exportHttpLogEntryV3(t *testing.T) {
	tests := []struct {
		description    string
		expectedOutput stringmap.StringMap
		logEntry       *envoy_data_accesslog_v3.HTTPAccessLogEntry
	}{
		{
			description: "Minimal HTTP log entry",
			logEntry: &envoy_data_accesslog_v3.HTTPAccessLogEntry{
				CommonProperties: &envoy_data_accesslog_v3.AccessLogCommon{
					SampleRate: 1,
					UpstreamRemoteAddress: &v3.Address{Address: &v3.Address_SocketAddress{SocketAddress: &v3.SocketAddress{
						Address:       "77.75.75.172",
						PortSpecifier: &v3.SocketAddress_PortValue{PortValue: 443},
					}}},
				},
				ProtocolVersion: 2,
				Request: &envoy_data_accesslog_v3.HTTPRequestProperties{
					Scheme:    "http",
					Authority: "www.seznam.cz",
					Path:      "/",
				},
				Response: &envoy_data_accesslog_v3.HTTPResponseProperties{
					ResponseCode: &wrappers.UInt32Value{Value: 200},
				},
			},
			expectedOutput: stringmap.StringMap{
				"authority":             "www.seznam.cz",
				"path":                  "/",
				"protocolVersion":       "HTTP11",
				"responseCode":          "200",
				"sampleRate":            "1",
				"scheme":                "http",
				"upstreamRemoteAddress": "77.75.75.172",
				"upstreamRemotePort":    "443",
			},
		},
	}

	for _, test := range tests {
		f := func(*testing.T) {
			output := envoyV3HttpAccessLogEntryToStringMap(logger, test.logEntry)
			for k, v := range test.expectedOutput {
				assert.Equal(t, v, output[k], "expected %s=%s", k, v)
			}
		}
		t.Run(test.description, f)
	}
}

func Test_exportTcpLogEntryV3(t *testing.T) {
	tests := []struct {
		description    string
		expectedOutput stringmap.StringMap
		logEntry       *envoy_data_accesslog_v3.TCPAccessLogEntry
	}{
		{
			description: "Full TCP log entry",
			logEntry: &envoy_data_accesslog_v3.TCPAccessLogEntry{
				CommonProperties: &envoy_data_accesslog_v3.AccessLogCommon{
					SampleRate: 1,
					UpstreamRemoteAddress: &v3.Address{Address: &v3.Address_SocketAddress{SocketAddress: &v3.SocketAddress{
						Address:       "77.75.75.172",
						PortSpecifier: &v3.SocketAddress_PortValue{PortValue: 443},
					}}},
				},
				ConnectionProperties: &envoy_data_accesslog_v3.ConnectionProperties{
					ReceivedBytes: uint64(100),
					SentBytes:     uint64(100),
				},
			},
			expectedOutput: stringmap.StringMap{
				"receivedBytes":         "100",
				"sampleRate":            "1",
				"sentBytes":             "100",
				"upstreamCluster":       "",
				"upstreamRemoteAddress": "77.75.75.172",
				"upstreamRemotePort":    "443",
			},
		},
	}

	for _, test := range tests {
		f := func(*testing.T) {
			for k, v := range test.expectedOutput {
				assert.Equal(t, v, envoyV3TcpAccessLogEntryToStringMap(logger, test.logEntry)[k], "expected %s=%s", k, v)
			}
		}
		t.Run(test.description, f)
	}
}
