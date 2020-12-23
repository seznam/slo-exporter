package access_log_server

import (
	"testing"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoy_data_accesslog_v2 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v2"
	envoy_service_accesslog_v2 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v2"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"

	"github.com/stretchr/testify/assert"

	"github.com/seznam/slo-exporter/pkg/stringmap"
)

func Test_exportCommonPropertiesv2(t *testing.T) {
	tests := []struct {
		input envoy_data_accesslog_v2.AccessLogCommon
		res   stringmap.StringMap
	}{
		{
			input: envoy_data_accesslog_v2.AccessLogCommon{},
			res: stringmap.StringMap{
				"DownstreamDirectRemoteAddress":  "<nil>",
				"DownstreamLocalAddress":         "<nil>",
				"DownstreamRemoteAddress":        "<nil>",
				"Metadata":                       "<nil>",
				"ResponseFlags":                  "<nil>",
				"RouteName":                      "",
				"SampleRate":                     "0e+00",
				"StartTime":                      "<nil>",
				"TimeToFirstDownstreamTxByte":    "<nil>",
				"TimeToFirstUpstreamRxByte":      "<nil>",
				"TimeToFirstUpstreamTxByte":      "<nil>",
				"TimeToLastDownstreamTxByte":     "<nil>",
				"TimeToLastRxByte":               "<nil>",
				"TimeToLastUpstreamRxByte":       "<nil>",
				"TimeToLastUpstreamTxByte":       "<nil>",
				"TlsProperties":                  "<nil>",
				"UpstreamCluster":                "",
				"UpstreamLocalAddress":           "<nil>",
				"UpstreamRemoteAddress":          "<nil>",
				"UpstreamTransportFailureReason": "",
			},
		},
		{
			input: envoy_data_accesslog_v2.AccessLogCommon{
				SampleRate: 1,
				DownstreamRemoteAddress: &v2.Address{Address: &v2.Address_SocketAddress{SocketAddress: &v2.SocketAddress{
					Address:       "127.0.0.1",
					PortSpecifier: &v2.SocketAddress_PortValue{46058},
				}}},
				DownstreamLocalAddress: &v2.Address{Address: &v2.Address_SocketAddress{SocketAddress: &v2.SocketAddress{
					Address:       "127.0.0.1",
					PortSpecifier: &v2.SocketAddress_PortValue{8080},
				}}},
				TlsProperties: &envoy_data_accesslog_v2.TLSProperties{
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
				UpstreamRemoteAddress: &v2.Address{Address: &v2.Address_SocketAddress{SocketAddress: &v2.SocketAddress{
					Address:       "77.75.75.172",
					PortSpecifier: &v2.SocketAddress_PortValue{443},
				}}},
				UpstreamLocalAddress: &v2.Address{Address: &v2.Address_SocketAddress{SocketAddress: &v2.SocketAddress{
					Address:       "10.0.116.130",
					PortSpecifier: &v2.SocketAddress_PortValue{48734},
				}}},
				UpstreamCluster: "service_seznam_cz",
				ResponseFlags: &envoy_data_accesslog_v2.ResponseFlags{
					RateLimited: true,
				},
				Metadata:                       nil,
				UpstreamTransportFailureReason: "foo",
				RouteName:                      "foo",
				DownstreamDirectRemoteAddress: &v2.Address{Address: &v2.Address_SocketAddress{SocketAddress: &v2.SocketAddress{
					Address:       "127.0.0.1",
					PortSpecifier: &v2.SocketAddress_PortValue{44848},
				}}},
				FilterStateObjects: nil,
			},
			res: stringmap.StringMap{
				"DownstreamDirectRemoteAddress":  "socket_address:{address:\"127.0.0.1\" port_value:44848}",
				"DownstreamLocalAddress":         "socket_address:{address:\"127.0.0.1\" port_value:8080}",
				"DownstreamRemoteAddress":        "socket_address:{address:\"127.0.0.1\" port_value:46058}",
				"Metadata":                       "<nil>",
				"ResponseFlags":                  "RateLimited:true",
				"RouteName":                      "foo",
				"SampleRate":                     "1e+00",
				"StartTime":                      "seconds:1608647248 nanos:741408000",
				"TimeToFirstDownstreamTxByte":    "490791797",
				"TimeToFirstUpstreamRxByte":      "463920708",
				"TimeToFirstUpstreamTxByte":      "490187312",
				"TimeToLastDownstreamTxByte":     "490791800",
				"TimeToLastRxByte":               "101859",
				"TimeToLastUpstreamRxByte":       "490187312",
				"TimeToLastUpstreamTxByte":       "490187312",
				"TlsProperties":                  "tls_version:TLSv1_3 tls_cipher_suite:{value:4865}",
				"UpstreamCluster":                "service_seznam_cz",
				"UpstreamLocalAddress":           "socket_address:{address:\"10.0.116.130\" port_value:48734}",
				"UpstreamRemoteAddress":          "socket_address:{address:\"77.75.75.172\" port_value:443}",
				"UpstreamTransportFailureReason": "foo",
			},
		},
	}
	for _, tt := range tests {
		output := exportCommonPropertiesv2(&tt.input)
		for k, v := range tt.res {
			assert.Equal(t, v, output[k])
		}
	}
}

func Test_exportRequestPropertiesv2(t *testing.T) {
	tests := []struct {
		input  envoy_data_accesslog_v2.HTTPRequestProperties
		result stringmap.StringMap
	}{
		{
			input: envoy_data_accesslog_v2.HTTPRequestProperties{},
			result: stringmap.StringMap{
				"RequestMethod":       "METHOD_UNSPECIFIED",
				"Scheme":              "",
				"Authority":           "",
				"Port":                "<nil>",
				"Path":                "",
				"UserAgent":           "",
				"Referer":             "",
				"ForwardedFor":        "",
				"RequestId":           "",
				"OriginalPath":        "",
				"RequestHeadersBytes": "0",
				"RequestBodyBytes":    "0",
				"RequestHeaders":      "",
			},
		},
		{
			input: envoy_data_accesslog_v2.HTTPRequestProperties{
				RequestMethod:       1, // GET
				Scheme:              "https",
				Authority:           "www.seznam.cz",
				Port:                &wrappers.UInt32Value{Value: 443},
				Path:                "/",
				UserAgent:           "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:83.0) Gecko/20100101 Firefox/83.0",
				Referer:             "xxx",
				ForwardedFor:        "xxx",
				RequestId:           "f1d027b9-8b7f-448a-bbe2-1a3151801af1",
				OriginalPath:        "",
				RequestHeadersBytes: 932,
				RequestBodyBytes:    0,
				RequestHeaders:      nil,
			},
			result: stringmap.StringMap{
				"RequestMethod":       "GET",
				"Scheme":              "https",
				"Authority":           "www.seznam.cz",
				"Port":                "value:443",
				"Path":                "/",
				"UserAgent":           "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:83.0) Gecko/20100101 Firefox/83.0",
				"Referer":             "xxx",
				"ForwardedFor":        "xxx",
				"RequestId":           "f1d027b9-8b7f-448a-bbe2-1a3151801af1",
				"OriginalPath":        "",
				"RequestHeadersBytes": "932",
				"RequestBodyBytes":    "0",
				"RequestHeaders":      "",
			},
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.result, exportRequestPropertiesv2(&test.input))
	}
}

func Test_exportResponsePropertiesv2(t *testing.T) {
	tests := []struct {
		input  envoy_data_accesslog_v2.HTTPResponseProperties
		result stringmap.StringMap
	}{
		{
			input: envoy_data_accesslog_v2.HTTPResponseProperties{},
			result: stringmap.StringMap{
				"ResponseCode":         "<nil>",
				"ResponseHeadersBytes": "0",
				"ResponseBodyBytes":    "0",
				"ResponseHeaders":      "",
				"ResponseTrailers":     "",
				"ResponseCodeDetails":  "",
			}},
		{
			input: envoy_data_accesslog_v2.HTTPResponseProperties{
				ResponseCode:         &wrappers.UInt32Value{Value: 200},
				ResponseHeadersBytes: 166,
				ResponseBodyBytes:    74400,
				ResponseHeaders:      map[string]string{"slo-domain": "userportal", "slo-class": "critical"},
				ResponseTrailers:     map[string]string{"slo-availability-result": "success"},
				ResponseCodeDetails:  "via_upstream",
			},
			result: stringmap.StringMap{
				"ResponseCode":         "value:200",
				"ResponseHeadersBytes": "166",
				"ResponseBodyBytes":    "74400",
				"ResponseHeaders":      "slo-class=\"critical\",slo-domain=\"userportal\"",
				"ResponseTrailers":     "slo-availability-result=\"success\"",
				"ResponseCodeDetails":  "via_upstream",
			},
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.result, exportResponsePropertiesv2(&test.input))
	}
}

func Test_exportHttpLogEntryv2(t *testing.T) {
	tests := []struct {
		expected stringmap.StringMap
		logEntry *envoy_data_accesslog_v2.HTTPAccessLogEntry
	}{
		{
			logEntry: &envoy_data_accesslog_v2.HTTPAccessLogEntry{
				CommonProperties: &envoy_data_accesslog_v2.AccessLogCommon{
					SampleRate: 1,
					UpstreamRemoteAddress: &v2.Address{Address: &v2.Address_SocketAddress{SocketAddress: &v2.SocketAddress{
						Address:       "77.75.75.172",
						PortSpecifier: &v2.SocketAddress_PortValue{443},
					}}},
				},
				ProtocolVersion: 2,
				Request: &envoy_data_accesslog_v2.HTTPRequestProperties{
					Scheme:    "http",
					Authority: "www.seznam.cz",
					Path:      "/",
				},
				Response: &envoy_data_accesslog_v2.HTTPResponseProperties{
					ResponseCode: &wrappers.UInt32Value{Value: 200},
				},
			},
			expected: stringmap.StringMap{
				"Authority":                      "www.seznam.cz",
				"DownstreamDirectRemoteAddress":  "<nil>",
				"DownstreamLocalAddress":         "<nil>",
				"DownstreamRemoteAddress":        "<nil>",
				"ForwardedFor":                   "",
				"Metadata":                       "<nil>",
				"OriginalPath":                   "",
				"Path":                           "/",
				"Port":                           "<nil>",
				"ProtocolVersion":                "HTTP11",
				"Referer":                        "",
				"RequestBodyBytes":               "0",
				"RequestHeaders":                 "",
				"RequestHeadersBytes":            "0",
				"RequestId":                      "",
				"RequestMethod":                  "METHOD_UNSPECIFIED",
				"ResponseBodyBytes":              "0",
				"ResponseCode":                   "value:200",
				"ResponseCodeDetails":            "",
				"ResponseFlags":                  "<nil>",
				"ResponseHeaders":                "",
				"ResponseHeadersBytes":           "0",
				"ResponseTrailers":               "",
				"RouteName":                      "",
				"SampleRate":                     "1e+00",
				"Scheme":                         "http",
				"StartTime":                      "<nil>",
				"TimeToFirstDownstreamTxByte":    "<nil>",
				"TimeToFirstUpstreamRxByte":      "<nil>",
				"TimeToFirstUpstreamTxByte":      "<nil>",
				"TimeToLastDownstreamTxByte":     "<nil>",
				"TimeToLastRxByte":               "<nil>",
				"TimeToLastUpstreamRxByte":       "<nil>",
				"TimeToLastUpstreamTxByte":       "<nil>",
				"TlsProperties":                  "<nil>",
				"UpstreamCluster":                "",
				"UpstreamLocalAddress":           "<nil>",
				"UpstreamRemoteAddress":          "socket_address:{address:\"77.75.75.172\" port_value:443}",
				"UpstreamTransportFailureReason": "",
				"UserAgent":                      "",
				"__log_entry_json":               "common_properties:{sample_rate:1 upstream_remote_address:{socket_address:{address:\"77.75.75.172\" port_value:443}}} protocol_version:HTTP11 request:{scheme:\"http\" authority:\"www.seznam.cz\" path:\"/\"} response:{response_code:{value:200}}"},
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, exportHttpLogEntryv2(test.logEntry))
	}
}

func Test_exportTcpLogEntryv2(t *testing.T) {
	tests := []struct {
		expected stringmap.StringMap
		logEntry *envoy_data_accesslog_v2.TCPAccessLogEntry
	}{
		{
			logEntry: &envoy_data_accesslog_v2.TCPAccessLogEntry{
				CommonProperties: &envoy_data_accesslog_v2.AccessLogCommon{
					SampleRate: 1,
					UpstreamRemoteAddress: &v2.Address{Address: &v2.Address_SocketAddress{SocketAddress: &v2.SocketAddress{
						Address:       "77.75.75.172",
						PortSpecifier: &v2.SocketAddress_PortValue{443},
					}}},
				},
				ConnectionProperties: &envoy_data_accesslog_v2.ConnectionProperties{
					ReceivedBytes: uint64(100),
					SentBytes:     uint64(100),
				},
			},
			expected: stringmap.StringMap{
				"DownstreamDirectRemoteAddress":  "<nil>",
				"DownstreamLocalAddress":         "<nil>",
				"DownstreamRemoteAddress":        "<nil>",
				"Metadata":                       "<nil>",
				"ReceivedBytes":                  "100",
				"ResponseFlags":                  "<nil>",
				"RouteName":                      "",
				"SampleRate":                     "1e+00",
				"SentBytes":                      "100",
				"StartTime":                      "<nil>",
				"TimeToFirstDownstreamTxByte":    "<nil>",
				"TimeToFirstUpstreamRxByte":      "<nil>",
				"TimeToFirstUpstreamTxByte":      "<nil>",
				"TimeToLastDownstreamTxByte":     "<nil>",
				"TimeToLastRxByte":               "<nil>",
				"TimeToLastUpstreamRxByte":       "<nil>",
				"TimeToLastUpstreamTxByte":       "<nil>",
				"TlsProperties":                  "<nil>",
				"UpstreamCluster":                "",
				"UpstreamLocalAddress":           "<nil>",
				"UpstreamRemoteAddress":          "socket_address:{address:\"77.75.75.172\" port_value:443}",
				"UpstreamTransportFailureReason": "",
				"__log_entry_json":               "common_properties:{sample_rate:1 upstream_remote_address:{socket_address:{address:\"77.75.75.172\" port_value:443}}} connection_properties:{received_bytes:100 sent_bytes:100}",
			}},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, exportTcpLogEntryv2(test.logEntry))
	}
}

// Test whether raw json of the received msg is includede in the generated event's metadata
func Test_HttpLogRawJsonIncludedV2(t *testing.T) {
	tests := []envoy_service_accesslog_v2.StreamAccessLogsMessage{

		{
			Identifier: &envoy_service_accesslog_v2.StreamAccessLogsMessage_Identifier{
				Node:    nil,
				LogName: "access_log",
			},
			LogEntries: &envoy_service_accesslog_v2.StreamAccessLogsMessage_HttpLogs{
				HttpLogs: &envoy_service_accesslog_v2.StreamAccessLogsMessage_HTTPAccessLogEntries{},
			},
		},
		{
			Identifier: &envoy_service_accesslog_v2.StreamAccessLogsMessage_Identifier{
				Node:    nil,
				LogName: "access_log",
			},
			LogEntries: &envoy_service_accesslog_v2.StreamAccessLogsMessage_HttpLogs{
				HttpLogs: &envoy_service_accesslog_v2.StreamAccessLogsMessage_HTTPAccessLogEntries{
					LogEntry: []*envoy_data_accesslog_v2.HTTPAccessLogEntry{
						{
							CommonProperties: &envoy_data_accesslog_v2.AccessLogCommon{
								DownstreamRemoteAddress: &v2.Address{Address: &v2.Address_SocketAddress{SocketAddress: &v2.SocketAddress{
									Protocol:      0,
									Address:       "127.0.0.1",
									PortSpecifier: &v2.SocketAddress_PortValue{46058},
									ResolverName:  "",
									Ipv4Compat:    false,
								}}},
								UpstreamRemoteAddress: &v2.Address{Address: &v2.Address_SocketAddress{SocketAddress: &v2.SocketAddress{
									Protocol:      0,
									Address:       "77.75.75.172",
									PortSpecifier: &v2.SocketAddress_PortValue{443},
									ResolverName:  "",
									Ipv4Compat:    false,
								}}},
							},
							Request: &envoy_data_accesslog_v2.HTTPRequestProperties{
								Scheme:              "http",
								Authority:           "www.seznam.cz",
								Path:                "/",
								UserAgent:           "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36",
								RequestHeadersBytes: 932,
								RequestHeaders:      nil,
							},
							Response: &envoy_data_accesslog_v2.HTTPResponseProperties{
								ResponseCode:         &wrappers.UInt32Value{Value: 200},
								ResponseHeadersBytes: 166,
								ResponseBodyBytes:    74400,
							},
						},
					},
				},
			},
		},
	}

	for _, testMsg := range tests {
		res := exportLogEntriesv2(&testMsg)
		for i, _ := range testMsg.GetHttpLogs().LogEntry {
			assert.Equal(t, testMsg.GetHttpLogs().LogEntry[i].String(), res[i]["__log_entry_json"])
		}
	}
}
