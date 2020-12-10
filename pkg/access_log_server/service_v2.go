package access_log_server

import (
	"io"
	"strconv"

	envoy_data_accesslog_v2 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v2"
	envoy_service_accesslog_v2 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v2"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/stringmap"
)

type AccessLogServiceV2 struct {
	outChan chan *event.Raw
	logger  logrus.FieldLogger
	envoy_service_accesslog_v2.UnimplementedAccessLogServiceServer
}

func exportCommonPropertiesV2(p *envoy_data_accesslog_v2.AccessLogCommon) stringmap.StringMap {
	res := stringmap.StringMap{}
	res["DownstreamDirectRemoteAddress"] = p.DownstreamDirectRemoteAddress.String()
	res["DownstreamLocalAddress"] = p.DownstreamLocalAddress.String()
	res["DownstreamRemoteAddress"] = p.DownstreamRemoteAddress.String()
	res["Metadata"] = p.Metadata.String()
	res["ResponseFlags"] = p.ResponseFlags.String()
	res["RouteName"] = p.RouteName
	res["StartTime"] = p.StartTime.String()
	res["TimeToFirstDownstreamTxByte"] = p.TimeToFirstDownstreamTxByte.String()
	res["TimeToFirstUpstreamRxByte"] = p.TimeToFirstUpstreamRxByte.String()
	res["TimeToFirstUpstreamTxByte"] = p.TimeToFirstUpstreamTxByte.String()
	res["TimeToLastDownstreamTxByte"] = p.TimeToLastDownstreamTxByte.String()
	res["TimeToLastRxByte"] = p.TimeToLastRxByte.String()
	res["TimeToLastUpstreamRxByte"] = p.TimeToLastUpstreamRxByte.String()
	res["TimeToLastUpstreamTxByte"] = p.TimeToLastUpstreamTxByte.String()
	res["TlsProperties"] = p.TlsProperties.String()
	res["UpstreamCluster"] = p.UpstreamCluster
	res["UpstreamLocalAddress"] = p.UpstreamLocalAddress.String()
	res["UpstreamRemoteAddress"] = p.UpstreamRemoteAddress.String()
	res["UpstreamTransportFailureReason"] = p.UpstreamTransportFailureReason
	res["SampleRate"] = strconv.FormatFloat(p.SampleRate, 'e', -1, 64)
	// FilterStateObjects are omitted

	return res
}

func exportLogEntriesV2(msg *envoy_service_accesslog_v2.StreamAccessLogsMessage) []stringmap.StringMap {
	res := []stringmap.StringMap{}

	if logs := msg.GetHttpLogs(); logs != nil {
		for _, l := range logs.LogEntry {
			exportedLogEntry := stringmap.StringMap{}
			// AccessLogCommon
			exportedLogEntry.Merge(exportCommonPropertiesV2(l.CommonProperties))

			// ProtocolVersion
			exportedLogEntry["ProtocolVersion"] = l.ProtocolVersion.String()

			// Request
			exportedLogEntry["Authority"] = l.Request.Authority
			exportedLogEntry["ForwardedFor"] = l.Request.ForwardedFor
			exportedLogEntry["OriginalPath"] = l.Request.OriginalPath
			exportedLogEntry["Path"] = l.Request.Path
			exportedLogEntry["Port"] = l.Request.Port.String()
			exportedLogEntry["Referer"] = l.Request.Referer
			exportedLogEntry["RequestHeaders"] = stringmap.StringMap(l.Request.RequestHeaders).String()
			exportedLogEntry["RequestBodyBytes"] = strconv.FormatUint(l.Request.RequestBodyBytes, 10)
			exportedLogEntry["RequestHeadersBytes"] = strconv.FormatUint(l.Request.RequestHeadersBytes, 10)
			exportedLogEntry["RequestId"] = l.Request.RequestId
			exportedLogEntry["RequestMethod"] = l.Request.RequestMethod.String()
			exportedLogEntry["Scheme"] = l.Request.Scheme
			exportedLogEntry["UserAgent"] = l.Request.UserAgent

			// Response
			exportedLogEntry["ResponseBodyBytes"] = strconv.FormatUint(l.Response.ResponseBodyBytes, 10)
			exportedLogEntry["ResponseCodeDetails"] = l.Response.ResponseCodeDetails
			exportedLogEntry["ResponseCode"] = l.Response.ResponseCode.String()
			exportedLogEntry["ResponseHeadersBytes"] = strconv.FormatUint(l.Response.ResponseHeadersBytes, 10)
			exportedLogEntry["ResponseHeaders"] = stringmap.StringMap(l.Response.ResponseHeaders).String()
			exportedLogEntry["ResponseTrailers"] = stringmap.StringMap(l.Response.ResponseTrailers).String()

			res = append(res, exportedLogEntry)

			logEntriesTotal.WithLabelValues("HTTP", "v2").Inc()

		}
	} else if logs := msg.GetTcpLogs(); logs != nil {
		for _, l := range logs.LogEntry {
			exportedLogEntry := stringmap.StringMap{}
			// AccessLogCommon
			exportedLogEntry.Merge(exportCommonPropertiesV2(l.CommonProperties))

			// ConnectionProperties
			exportedLogEntry["ReceivedBytes"] = strconv.FormatUint(l.ConnectionProperties.ReceivedBytes, 64)
			exportedLogEntry["SentBytes"] = strconv.FormatUint(l.ConnectionProperties.SentBytes, 64)

			res = append(res, exportedLogEntry)

			logEntriesTotal.WithLabelValues("TCP", "v2").Inc()

		}

	} else {
		// Unknown access log type
		errorsTotal.WithLabelValues("UnknownLogType").Inc()
	}
	return res

}

func (service_v2 *AccessLogServiceV2) StreamAccessLogs(stream envoy_service_accesslog_v2.AccessLogService_StreamAccessLogsServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// TODO verify whether correct
			return nil
		}
		if err != nil {
			errorsTotal.WithLabelValues("ProcessingStream").Inc()
			return err
		}

		for _, singleLogEntryMetadata := range exportLogEntriesV2(msg) {
			e := &event.Raw{
				Metadata: singleLogEntryMetadata,
				Quantity: 1,
			}
			service_v2.outChan <- e
		}

	}
	return nil
}

func (service_v2 *AccessLogServiceV2) Register(server *grpc.Server) {
	envoy_service_accesslog_v2.RegisterAccessLogServiceServer(server, service_v2)
}
