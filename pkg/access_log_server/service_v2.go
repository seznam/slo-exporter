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

func exportCommonPropertiesv2(p *envoy_data_accesslog_v2.AccessLogCommon) stringmap.StringMap {
	res := stringmap.StringMap{}

	res["DownstreamDirectRemoteAddress"] = p.DownstreamDirectRemoteAddress.String()
	res["DownstreamLocalAddress"] = p.DownstreamLocalAddress.String()
	res["DownstreamRemoteAddress"] = p.DownstreamRemoteAddress.String()
	res["Metadata"] = p.Metadata.String()
	res["ResponseFlags"] = p.ResponseFlags.String()
	res["RouteName"] = p.RouteName
	res["StartTime"] = p.StartTime.String()
	res["TimeToFirstDownstreamTxByte"] = pbDurationDeterministicString(p.TimeToFirstDownstreamTxByte)
	res["TimeToFirstUpstreamRxByte"] = pbDurationDeterministicString(p.TimeToFirstUpstreamRxByte)
	res["TimeToFirstUpstreamTxByte"] = pbDurationDeterministicString(p.TimeToFirstUpstreamTxByte)
	res["TimeToLastDownstreamTxByte"] = pbDurationDeterministicString(p.TimeToLastDownstreamTxByte)
	res["TimeToLastRxByte"] = pbDurationDeterministicString(p.TimeToLastRxByte)
	res["TimeToLastUpstreamRxByte"] = pbDurationDeterministicString(p.TimeToLastUpstreamRxByte)
	res["TimeToLastUpstreamTxByte"] = pbDurationDeterministicString(p.TimeToLastUpstreamTxByte)
	res["TlsProperties"] = p.TlsProperties.String()
	res["UpstreamCluster"] = p.UpstreamCluster
	res["UpstreamLocalAddress"] = p.UpstreamLocalAddress.String()
	res["UpstreamRemoteAddress"] = p.UpstreamRemoteAddress.String()
	res["UpstreamTransportFailureReason"] = p.UpstreamTransportFailureReason
	res["SampleRate"] = strconv.FormatFloat(p.SampleRate, 'e', -1, 64)
	// FilterStateObjects are omitted

	return res
}

func exportRequestPropertiesv2(request *envoy_data_accesslog_v2.HTTPRequestProperties) stringmap.StringMap {
	result := stringmap.StringMap{}

	result["Authority"] = request.Authority
	result["ForwardedFor"] = request.ForwardedFor
	result["OriginalPath"] = request.OriginalPath
	result["Path"] = request.Path
	result["Port"] = request.Port.String()
	result["Referer"] = request.Referer
	result["RequestHeaders"] = stringmap.StringMap(request.RequestHeaders).String()
	result["RequestBodyBytes"] = strconv.FormatUint(request.RequestBodyBytes, 10)
	result["RequestHeadersBytes"] = strconv.FormatUint(request.RequestHeadersBytes, 10)
	result["RequestId"] = request.RequestId
	result["RequestMethod"] = request.RequestMethod.String()
	result["Scheme"] = request.Scheme
	result["UserAgent"] = request.UserAgent

	return result
}

func exportResponsePropertiesv2(response *envoy_data_accesslog_v2.HTTPResponseProperties) stringmap.StringMap {
	result := stringmap.StringMap{}

	result["ResponseBodyBytes"] = strconv.FormatUint(response.ResponseBodyBytes, 10)
	result["ResponseCodeDetails"] = response.ResponseCodeDetails
	result["ResponseCode"] = response.ResponseCode.String()
	result["ResponseHeadersBytes"] = strconv.FormatUint(response.ResponseHeadersBytes, 10)
	result["ResponseHeaders"] = stringmap.StringMap(response.ResponseHeaders).String()
	result["ResponseTrailers"] = stringmap.StringMap(response.ResponseTrailers).String()

	return result
}

func exportHttpLogEntryv2(logEntry *envoy_data_accesslog_v2.HTTPAccessLogEntry) stringmap.StringMap {

	result := stringmap.StringMap{
		"__log_entry_json": logEntry.String(),
	}
	// AccessLogCommon
	result = result.Merge(exportCommonPropertiesv2(logEntry.CommonProperties))

	// ProtocolVersion
	result["ProtocolVersion"] = logEntry.ProtocolVersion.String()

	// Request properties
	result = result.Merge(exportRequestPropertiesv2(logEntry.Request))

	// Response properties
	result = result.Merge(exportResponsePropertiesv2(logEntry.Response))

	return result
}

func exportTcpLogEntryv2(logEntry *envoy_data_accesslog_v2.TCPAccessLogEntry) stringmap.StringMap {

	result := stringmap.StringMap{
		"__log_entry_json": logEntry.String(),
	}
	// AccessLogCommon
	result = result.Merge(exportCommonPropertiesv2(logEntry.CommonProperties))

	// ConnectionProperties
	result["ReceivedBytes"] = strconv.FormatUint(logEntry.ConnectionProperties.ReceivedBytes, 10)
	result["SentBytes"] = strconv.FormatUint(logEntry.ConnectionProperties.SentBytes, 10)

	return result
}

func exportLogEntriesv2(msg *envoy_service_accesslog_v2.StreamAccessLogsMessage) []stringmap.StringMap {
	res := []stringmap.StringMap{}

	if logs := msg.GetHttpLogs(); logs != nil {
		for _, l := range logs.LogEntry {

			res = append(res, exportHttpLogEntryv2(l))

			logEntriesTotal.WithLabelValues("HTTP", "v2").Inc()
		}
	} else if logs := msg.GetTcpLogs(); logs != nil {
		for _, l := range logs.LogEntry {

			res = append(res, exportTcpLogEntryv2(l))

			logEntriesTotal.WithLabelValues("TCP", "v2").Inc()
		}
	} else {
		// Unknown access log type
		errorsTotal.WithLabelValues("UnknownLogType").Inc()
		// TODO log
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

		for _, singleLogEntryMetadata := range exportLogEntriesv2(msg) {
			e := &event.Raw{
				Metadata: singleLogEntryMetadata,
				Quantity: 1,
			}
			service_v2.logger.Info(e)
			service_v2.outChan <- e
		}

	}
	return nil
}

func (service_v2 *AccessLogServiceV2) Register(server *grpc.Server) {
	envoy_service_accesslog_v2.RegisterAccessLogServiceServer(server, service_v2)
}
