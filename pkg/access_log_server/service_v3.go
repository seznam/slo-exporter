package access_log_server

import (
	"io"
	"strconv"

	envoy_data_accesslog_v3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	envoy_service_accesslog_v3 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/stringmap"
)

type AccessLogServiceV3 struct {
	outChan chan *event.Raw
	logger  logrus.FieldLogger
	envoy_service_accesslog_v3.UnimplementedAccessLogServiceServer
}

func exportCommonPropertiesV3(p *envoy_data_accesslog_v3.AccessLogCommon) stringmap.StringMap {
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

func exportRequestPropertiesV3(request *envoy_data_accesslog_v3.HTTPRequestProperties) stringmap.StringMap {
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

func exportResponsePropertiesV3(response *envoy_data_accesslog_v3.HTTPResponseProperties) stringmap.StringMap {
	result := stringmap.StringMap{}

	result["ResponseBodyBytes"] = strconv.FormatUint(response.ResponseBodyBytes, 10)
	result["ResponseCodeDetails"] = response.ResponseCodeDetails
	result["ResponseCode"] = response.ResponseCode.String()
	result["ResponseHeadersBytes"] = strconv.FormatUint(response.ResponseHeadersBytes, 10)
	result["ResponseHeaders"] = stringmap.StringMap(response.ResponseHeaders).String()
	result["ResponseTrailers"] = stringmap.StringMap(response.ResponseTrailers).String()

	return result
}

func exportHttpLogEntryV3(logEntry *envoy_data_accesslog_v3.HTTPAccessLogEntry) stringmap.StringMap {

	result := stringmap.StringMap{
		"__log_entry_json": logEntry.String(),
	}
	// AccessLogCommon
	result = result.Merge(exportCommonPropertiesV3(logEntry.CommonProperties))

	// ProtocolVersion
	result["ProtocolVersion"] = logEntry.ProtocolVersion.String()

	// Request properties
	result = result.Merge(exportRequestPropertiesV3(logEntry.Request))

	// Response properties
	result = result.Merge(exportResponsePropertiesV3(logEntry.Response))

	return result
}

func exportTcpLogEntryV3(logEntry *envoy_data_accesslog_v3.TCPAccessLogEntry) stringmap.StringMap {

	result := stringmap.StringMap{
		"__log_entry_json": logEntry.String(),
	}
	// AccessLogCommon
	result = result.Merge(exportCommonPropertiesV3(logEntry.CommonProperties))

	// ConnectionProperties
	result["ReceivedBytes"] = strconv.FormatUint(logEntry.ConnectionProperties.ReceivedBytes, 10)
	result["SentBytes"] = strconv.FormatUint(logEntry.ConnectionProperties.SentBytes, 10)

	return result
}

func exportLogEntriesV3(msg *envoy_service_accesslog_v3.StreamAccessLogsMessage) []stringmap.StringMap {
	res := []stringmap.StringMap{}

	if logs := msg.GetHttpLogs(); logs != nil {
		for _, l := range logs.LogEntry {

			res = append(res, exportHttpLogEntryV3(l))

			logEntriesTotal.WithLabelValues("HTTP", "v3").Inc()
		}
	} else if logs := msg.GetTcpLogs(); logs != nil {
		for _, l := range logs.LogEntry {

			res = append(res, exportTcpLogEntryV3(l))

			logEntriesTotal.WithLabelValues("TCP", "v3").Inc()
		}
	} else {
		// Unknown access log type
		errorsTotal.WithLabelValues("UnknownLogType").Inc()
		// TODO log
	}
	return res
}

func (service_v3 *AccessLogServiceV3) StreamAccessLogs(stream envoy_service_accesslog_v3.AccessLogService_StreamAccessLogsServer) error {
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

		for _, singleLogEntryMetadata := range exportLogEntriesV3(msg) {
			e := &event.Raw{
				Metadata: singleLogEntryMetadata,
				Quantity: 1,
			}
			service_v3.logger.Info(e)
			service_v3.outChan <- e
		}

	}
	return nil
}

func (service_v3 *AccessLogServiceV3) Register(server *grpc.Server) {
	envoy_service_accesslog_v3.RegisterAccessLogServiceServer(server, service_v3)
}
