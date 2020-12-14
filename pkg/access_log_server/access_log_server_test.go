package access_log_server

import (
	"testing"

	v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_data_accesslog_v3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"

	"github.com/stretchr/testify/assert"

	"github.com/seznam/slo-exporter/pkg/stringmap"
)

func Test_exportCommonPropertiesV3(t *testing.T) {
	tests := []struct {
		input envoy_data_accesslog_v3.AccessLogCommon
		res   stringmap.StringMap
	}{
		{
			input: envoy_data_accesslog_v3.AccessLogCommon{},
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
			input: envoy_data_accesslog_v3.AccessLogCommon{
				SampleRate: 0,
				DownstreamRemoteAddress: &v3.Address{Address: &v3.Address_SocketAddress{SocketAddress: &v3.SocketAddress{
					Protocol:      0,
					Address:       "127.0.0.1",
					PortSpecifier: &v3.SocketAddress_PortValue{46058},
					ResolverName:  "",
					Ipv4Compat:    false,
				}}},
				DownstreamLocalAddress:      nil,
				TlsProperties:               nil,
				StartTime:                   nil,
				TimeToLastRxByte:            nil,
				TimeToFirstUpstreamTxByte:   nil,
				TimeToLastUpstreamTxByte:    nil,
				TimeToFirstUpstreamRxByte:   nil,
				TimeToLastUpstreamRxByte:    nil,
				TimeToFirstDownstreamTxByte: nil,
				TimeToLastDownstreamTxByte:  nil,
				UpstreamRemoteAddress: &v3.Address{Address: &v3.Address_SocketAddress{SocketAddress: &v3.SocketAddress{
					Protocol:      0,
					Address:       "77.75.75.172",
					PortSpecifier: &v3.SocketAddress_PortValue{443},
					ResolverName:  "",
					Ipv4Compat:    false,
				}}},
				UpstreamLocalAddress:           nil,
				UpstreamCluster:                "",
				ResponseFlags:                  nil,
				Metadata:                       nil,
				UpstreamTransportFailureReason: "",
				RouteName:                      "",
				DownstreamDirectRemoteAddress:  nil,
				FilterStateObjects:             nil,
			},
			res: stringmap.StringMap{
				"DownstreamDirectRemoteAddress":  "<nil>",
				"DownstreamLocalAddress":         "<nil>",
				"DownstreamRemoteAddress":        "socket_address:{address:\"127.0.0.1\" port_value:46058}",
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
				"UpstreamRemoteAddress":          "socket_address:{address:\"77.75.75.172\" port_value:443}",
				"UpstreamTransportFailureReason": "",
			},
		},
	}
	for _, tt := range tests {
		assert.Equal(t, exportCommonPropertiesV3(&tt.input), tt.res)
	}
}
