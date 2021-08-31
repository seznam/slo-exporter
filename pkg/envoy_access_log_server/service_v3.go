package envoy_access_log_server

import (
	"fmt"
	"io"
	"strconv"
	"time"

	envoy_data_accesslog_v3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	envoy_service_accesslog_v3 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"github.com/golang/protobuf/ptypes"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/seznam/slo-exporter/pkg/stringmap"
)

type AccessLogServiceV3 struct {
	outChan chan event.Raw
	logger  logrus.FieldLogger
	envoy_service_accesslog_v3.UnimplementedAccessLogServiceServer
	eventIdMetadataKey string
}

type envoyV3AccessLogEntryCommonProperties envoy_data_accesslog_v3.AccessLogCommon

func (p envoyV3AccessLogEntryCommonProperties) StringMap(logger logrus.FieldLogger) stringmap.StringMap {
	m := stringmap.StringMap{}

	if p.DownstreamDirectRemoteAddress != nil {
		if sa := p.DownstreamDirectRemoteAddress.GetSocketAddress(); sa != nil {
			m["downstreamDirectRemoteAddress"] = sa.GetAddress()
			m["downstreamDirectRemotePort"] = fmt.Sprint(sa.GetPortValue())
		} else {
			logger.Warnf("%s address is in unsupported format", "DownstreamDirectRemoteAddress")
			errorsTotal.WithLabelValues("AddressFormatUnsupported").Inc()
		}
	}
	if p.DownstreamRemoteAddress != nil {
		if sa := p.DownstreamRemoteAddress.GetSocketAddress(); sa != nil {
			m["downstreamRemoteAddress"] = sa.GetAddress()
			m["downstreamRemotePort"] = fmt.Sprint(sa.GetPortValue())
		} else {
			logger.Warnf("%s address is in unsupported format", "DownstreamRemoteAddress")
			errorsTotal.WithLabelValues("AddressFormatUnsupported").Inc()
		}
	}
	if p.DownstreamLocalAddress != nil {
		if sa := p.DownstreamLocalAddress.GetSocketAddress(); sa != nil {
			m["downstreamLocalAddress"] = sa.GetAddress()
			m["downstreamLocalPort"] = fmt.Sprint(sa.GetPortValue())
		} else {
			logger.Warnf("%s address is in unsupported format", "DownstreamLocalAddress")
			errorsTotal.WithLabelValues("AddressFormatUnsupported").Inc()
		}
	}
	m["routeName"] = p.RouteName
	if ts := p.StartTime; ts != nil {
		if t, err := ptypes.Timestamp(ts); err == nil {
			m["startTime"] = t.Format(time.RFC3339)
		} else {
			logger.Warnf("Unable to parse %s timestamp", "StartTime")
			errorsTotal.WithLabelValues("InvalidTimestamp").Inc()
		}
	}
	if p.TimeToFirstDownstreamTxByte != nil {
		if duration, err := pbDurationDeterministicString(p.TimeToFirstDownstreamTxByte); err == nil {
			m["timeToFirstDownstreamTxByte"] = duration
		} else {
			logger.Warnf("Unable to parse %s duration", "TimeToFirstDownstreamTxByte")
			errorsTotal.WithLabelValues("InvalidDuration").Inc()
		}
	}
	if p.TimeToFirstUpstreamRxByte != nil {
		if duration, err := pbDurationDeterministicString(p.TimeToFirstUpstreamRxByte); err == nil {
			m["timeToFirstUpstreamRxByte"] = duration
		} else {
			logger.Warnf("Unable to parse %s duration", "TimeToFirstUpstreamRxByte")
			errorsTotal.WithLabelValues("InvalidDuration").Inc()
		}
	}
	if p.TimeToFirstUpstreamTxByte != nil {
		if duration, err := pbDurationDeterministicString(p.TimeToFirstUpstreamTxByte); err == nil {
			m["timeToFirstUpstreamTxByte"] = duration
		} else {
			logger.Warnf("Unable to parse %s duration", "TimeToFirstUpstreamTxByte")
			errorsTotal.WithLabelValues("InvalidDuration").Inc()
		}
	}
	if p.TimeToLastDownstreamTxByte != nil {
		if duration, err := pbDurationDeterministicString(p.TimeToLastDownstreamTxByte); err == nil {
			m["timeToLastDownstreamTxByte"] = duration
		} else {
			logger.Warnf("Unable to parse %s duration", "TimeToLastDownstreamTxByte")
			errorsTotal.WithLabelValues("InvalidDuration").Inc()
		}
	}
	if p.TimeToLastRxByte != nil {
		if duration, err := pbDurationDeterministicString(p.TimeToLastRxByte); err == nil {
			m["timeToLastRxByte"] = duration
		} else {
			logger.Warnf("Unable to parse %s duration", "TimeToLastRxByte")
			errorsTotal.WithLabelValues("InvalidDuration").Inc()
		}
	}
	if p.TimeToLastUpstreamRxByte != nil {
		if duration, err := pbDurationDeterministicString(p.TimeToLastUpstreamRxByte); err == nil {
			m["timeToLastUpstreamRxByte"] = duration
		} else {
			logger.Warnf("Unable to parse %s duration", "TimeToLastUpstreamRxByte")
			errorsTotal.WithLabelValues("InvalidDuration").Inc()
		}
	}
	if p.TimeToLastUpstreamTxByte != nil {
		if duration, err := pbDurationDeterministicString(p.TimeToLastUpstreamTxByte); err == nil {
			m["timeToLastUpstreamTxByte"] = duration
		} else {
			logger.Warnf("Unable to parse %s duration", "TimeToLastUpstreamTxByte")
			errorsTotal.WithLabelValues("InvalidDuration").Inc()
		}
	}
	m["upstreamCluster"] = p.UpstreamCluster
	if p.UpstreamLocalAddress != nil {
		if sa := p.UpstreamLocalAddress.GetSocketAddress(); sa != nil {
			m["upstreamLocalAddress"] = sa.GetAddress()
			m["upstreamLocalPort"] = fmt.Sprint(sa.GetPortValue())
		} else {
			logger.Warnf("%s address is in unsupported format", "UpstreamLocalAddress")
			errorsTotal.WithLabelValues("AddressFormatUnsupported").Inc()
		}
	}
	if p.UpstreamRemoteAddress != nil {
		if sa := p.UpstreamRemoteAddress.GetSocketAddress(); sa != nil {
			m["upstreamRemoteAddress"] = sa.GetAddress()
			m["upstreamRemotePort"] = fmt.Sprint(sa.GetPortValue())
		} else {
			logger.Warnf("%s address is in unsupported format", "UpstreamRemoteAddress")
			errorsTotal.WithLabelValues("AddressFormatUnsupported").Inc()
		}
	}
	m["upstreamTransportFailureReason"] = p.UpstreamTransportFailureReason
	m["sampleRate"] = strconv.FormatFloat(p.SampleRate, 'f', -1, 64)

	return m
}

type envoyV3AccessLogEntryHttpRequestProperties envoy_data_accesslog_v3.HTTPRequestProperties

func (request envoyV3AccessLogEntryHttpRequestProperties) StringMap(logger logrus.FieldLogger) stringmap.StringMap {
	result := stringmap.StringMap{}

	result["authority"] = request.Authority
	result["forwardedFor"] = request.ForwardedFor
	result["originalPath"] = request.OriginalPath
	result["path"] = request.Path
	result["referer"] = request.Referer

	// request headers are encoded with `http_` prefix (to keep the scheme used by Nginx)
	if request.RequestHeaders != nil {
		for header_name, header_value := range request.RequestHeaders {
			result["http_"+header_name] = header_value
		}
	}

	result["requestBodyBytes"] = strconv.FormatUint(request.RequestBodyBytes, 10)
	result["requestHeadersBytes"] = strconv.FormatUint(request.RequestHeadersBytes, 10)
	result["requestId"] = request.RequestId
	result["requestMethod"] = request.RequestMethod.String()
	result["scheme"] = request.Scheme
	result["userAgent"] = request.UserAgent

	return result
}

type envoyV3AccessLogEntryHttpResponseProperties envoy_data_accesslog_v3.HTTPResponseProperties

func (response envoyV3AccessLogEntryHttpResponseProperties) StringMap(logger logrus.FieldLogger) stringmap.StringMap {
	result := stringmap.StringMap{}

	result["responseBodyBytes"] = strconv.FormatUint(response.ResponseBodyBytes, 10)
	result["responseCodeDetails"] = response.ResponseCodeDetails
	result["responseCode"] = strconv.FormatUint(uint64(response.ResponseCode.GetValue()), 10)
	result["responseHeadersBytes"] = strconv.FormatUint(response.ResponseHeadersBytes, 10)
	// response headers are encoded with `sent_http_` prefix (to keep the scheme used by Nginx)
	if response.ResponseHeaders != nil {
		for header_name, header_value := range response.ResponseHeaders {
			result["sent_http_"+header_name] = header_value
		}
	}
	if response.ResponseTrailers != nil {
		for trailer_name, trailer_value := range response.ResponseTrailers {
			result["sent_trailer_"+trailer_name] = trailer_value
		}
	}
	return result
}

type envoyV3HttpAccessLogEntry envoy_data_accesslog_v3.HTTPAccessLogEntry

func (l envoyV3HttpAccessLogEntry) StringMap(logger logrus.FieldLogger) stringmap.StringMap {
	m := envoyV3AccessLogEntryCommonProperties(*l.CommonProperties).StringMap(logger)
	m["protocolVersion"] = l.ProtocolVersion.String()
	m = m.Merge(envoyV3AccessLogEntryHttpRequestProperties(*l.Request).StringMap(logger))
	m = m.Merge(envoyV3AccessLogEntryHttpResponseProperties(*l.Response).StringMap(logger))
	return m
}

type envoyV3TcpAccessLogEntry envoy_data_accesslog_v3.TCPAccessLogEntry

func (l envoyV3TcpAccessLogEntry) StringMap(logger logrus.FieldLogger) stringmap.StringMap {
	m := envoyV3AccessLogEntryCommonProperties(*l.CommonProperties).StringMap(logger)
	m["receivedBytes"] = strconv.FormatUint(l.ConnectionProperties.ReceivedBytes, 10)
	m["sentBytes"] = strconv.FormatUint(l.ConnectionProperties.SentBytes, 10)
	return m
}

func (service_v3 *AccessLogServiceV3) emitEvents(msg *envoy_service_accesslog_v3.StreamAccessLogsMessage) {
	if logs := msg.GetHttpLogs(); logs != nil {
		for _, l := range logs.LogEntry {
			logEntriesTotal.WithLabelValues("HTTP", "v3").Inc()
			metadata := envoyV3HttpAccessLogEntry(*l).StringMap(service_v3.logger)
			e := event.NewRaw(metadata.Get(service_v3.eventIdMetadataKey, ""), 1, metadata, nil)
			service_v3.logger.Debug(e)
			service_v3.outChan <- e
		}
	} else if logs := msg.GetTcpLogs(); logs != nil {
		for _, l := range logs.LogEntry {
			logEntriesTotal.WithLabelValues("TCP", "v3").Inc()
			metadata := envoyV3TcpAccessLogEntry(*l).StringMap(service_v3.logger)
			e := event.NewRaw(metadata.Get(service_v3.eventIdMetadataKey, ""), 1, metadata, nil)
			service_v3.logger.Debug(e)
			service_v3.outChan <- e
		}
	} else {
		// Unknown access log type
		errorsTotal.WithLabelValues("UnknownLogType").Inc()
		service_v3.logger.Warnf("Unknown access log message type: %v", msg)
	}
}

func (service_v3 *AccessLogServiceV3) StreamAccessLogs(stream envoy_service_accesslog_v3.AccessLogService_StreamAccessLogsServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			errorsTotal.WithLabelValues("ProcessingStream").Inc()
			return err
		}
		service_v3.emitEvents(msg)
	}
	return nil
}

func (service_v3 *AccessLogServiceV3) Register(server *grpc.Server) {
	envoy_service_accesslog_v3.RegisterAccessLogServiceServer(server, service_v3)
}
