package envoy_access_log_server

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

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

func envoyV3AccessLogEntryCommonPropertiesToStringMap(logger logrus.FieldLogger, p *envoy_data_accesslog_v3.AccessLogCommon) stringmap.StringMap {
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
		if !ts.IsValid() {
			logger.Warnf("Unable to parse %s timestamp", "StartTime")
			errorsTotal.WithLabelValues("InvalidTimestamp").Inc()
		} else {
			m["startTime"] = ts.AsTime().Format(time.RFC3339)
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

func envoyV3AccessLogEntryHTTPRequestPropertiesToStringMap(_ logrus.FieldLogger, request *envoy_data_accesslog_v3.HTTPRequestProperties) stringmap.StringMap {
	result := stringmap.StringMap{}

	result["authority"] = request.Authority
	result["forwardedFor"] = request.ForwardedFor
	result["originalPath"] = request.OriginalPath
	result["path"] = request.Path
	result["referer"] = request.Referer

	// request headers are encoded with `http_` prefix (to keep the scheme used by Nginx)
	if request.RequestHeaders != nil {
		for headerName, headerValue := range request.RequestHeaders {
			result["http_"+headerName] = headerValue
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

func envoyV3AccessLogEntryHTTPResponsePropertiesToStringMap(_ logrus.FieldLogger, response *envoy_data_accesslog_v3.HTTPResponseProperties) stringmap.StringMap {
	result := stringmap.StringMap{}

	result["responseBodyBytes"] = strconv.FormatUint(response.ResponseBodyBytes, 10)
	result["responseCodeDetails"] = response.ResponseCodeDetails
	result["responseCode"] = strconv.FormatUint(uint64(response.ResponseCode.GetValue()), 10)
	result["responseHeadersBytes"] = strconv.FormatUint(response.ResponseHeadersBytes, 10)
	// response headers are encoded with `sent_http_` prefix (to keep the scheme used by Nginx)
	if response.ResponseHeaders != nil {
		for headerName, headerValue := range response.ResponseHeaders {
			result["sent_http_"+headerName] = headerValue
		}
	}
	if response.ResponseTrailers != nil {
		for trailerName, trailerValue := range response.ResponseTrailers {
			result["sent_trailer_"+trailerName] = trailerValue
		}
	}
	return result
}

func envoyV3HttpAccessLogEntryToStringMap(logger logrus.FieldLogger, l *envoy_data_accesslog_v3.HTTPAccessLogEntry) stringmap.StringMap {
	m := envoyV3AccessLogEntryCommonPropertiesToStringMap(logger, l.CommonProperties)
	m["protocolVersion"] = l.ProtocolVersion.String()
	m = m.Merge(envoyV3AccessLogEntryHTTPRequestPropertiesToStringMap(logger, l.Request))
	m = m.Merge(envoyV3AccessLogEntryHTTPResponsePropertiesToStringMap(logger, l.Response))
	return m
}

func envoyV3TcpAccessLogEntryToStringMap(logger logrus.FieldLogger, l *envoy_data_accesslog_v3.TCPAccessLogEntry) stringmap.StringMap {
	m := envoyV3AccessLogEntryCommonPropertiesToStringMap(logger, l.CommonProperties)
	m["receivedBytes"] = strconv.FormatUint(l.ConnectionProperties.ReceivedBytes, 10)
	m["sentBytes"] = strconv.FormatUint(l.ConnectionProperties.SentBytes, 10)
	return m
}

func (service_v3 *AccessLogServiceV3) emitEvents(msg *envoy_service_accesslog_v3.StreamAccessLogsMessage) {
	if logs := msg.GetHttpLogs(); logs != nil {
		for _, l := range logs.LogEntry {
			logEntriesTotal.WithLabelValues("HTTP", "v3").Inc()
			e := &event.Raw{
				Metadata: envoyV3HttpAccessLogEntryToStringMap(service_v3.logger, l),
				Quantity: 1,
			}
			service_v3.logger.Debug(e)
			service_v3.outChan <- e
		}
	} else if logs := msg.GetTcpLogs(); logs != nil {
		for _, l := range logs.LogEntry {
			logEntriesTotal.WithLabelValues("TCP", "v3").Inc()
			e := &event.Raw{
				Metadata: envoyV3TcpAccessLogEntryToStringMap(service_v3.logger, l),
				Quantity: 1,
			}
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
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			errorsTotal.WithLabelValues("ProcessingStream").Inc()
			return err
		}
		service_v3.emitEvents(msg)
	}
}

func (service_v3 *AccessLogServiceV3) Register(server *grpc.Server) {
	envoy_service_accesslog_v3.RegisterAccessLogServiceServer(server, service_v3)
}
