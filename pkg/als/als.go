package als

import (
	"fmt"
	"github.com/seznam/slo-exporter/pkg/event"
	"time"

	//"encoding/json"
	ec "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"

	alf "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v2"
	als "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v2"
	"github.com/golang/protobuf/ptypes"
	"github.com/sirupsen/logrus"
)

type AccessLogService struct {
	logger        logrus.FieldLogger
	outputChannel *(chan *event.Raw)
}

func (svc *AccessLogService) log(logName string, logEntry *alf.HTTPAccessLogEntry) {
	// json, _ := json.MarshalIndent(logEntry, "", "  ")
	// svc.logger.Info("AccessLog: ", string(json[:]))
	// fmt.Println("\n%s\n", string(json))

	common := logEntry.CommonProperties
	req := logEntry.Request
	resp := logEntry.Response
	if common == nil {
		common = &alf.AccessLogCommon{}
	}
	if req == nil {
		req = &alf.HTTPRequestProperties{}
	}
	if resp == nil {
		resp = &alf.HTTPResponseProperties{}
	}

	// fmt.Printf("\n%+v\n\n", logEntry)
	// fmt.Printf("\n%#v\n", logEntry)
	// fmt.Printf("\n%#v\n", logEntry.CommonProperties)
	// fmt.Printf("\n%#v\n", logEntry.Request)
	// fmt.Printf("\n%#v\n\n", logEntry.Response)

	metadata := make(map[string]string)
	metadata["logName"] = logName

	if common.DownstreamLocalAddress != nil {
		if sa := common.DownstreamLocalAddress.GetSocketAddress(); sa != nil {
			metadata["downstreamRemoteAddress"] = sa.GetAddress()
			metadata["downstreamRemotePort"] = fmt.Sprintf("%d", sa.GetPortValue())
		}
	}

	if st := common.GetStartTime(); st != nil {
		if t, err := ptypes.Timestamp(st); err == nil {
			metadata["time"] = t.Format(time.RFC3339)
		}
	}
	if common.GetUpstreamRemoteAddress() != nil {
	}

	pbduration := common.GetTimeToLastDownstreamTxByte()
	if duration, err := ptypes.Duration(pbduration); err == nil {
		metadata["duration"] = fmt.Sprintf("%#v", duration.Seconds())
	}

	if method, ok := ec.RequestMethod_name[int32(req.RequestMethod)]; ok {
		metadata["method"] = method
	} else {
		metadata["method"] = "UNDEFINED"
	}
	metadata["scheme"] = req.Scheme
	metadata["authority"] = req.Authority
	metadata["path"] = req.Path
	metadata["userAgent"] = req.UserAgent
	metadata["referer"] = req.Referer
	metadata["forwardedFor"] = req.ForwardedFor
	metadata["requestId"] = req.RequestId
	metadata["originalPath"] = req.OriginalPath
	// request headers are encoded with `http_` prefix
	// same encoding is using nginx (and others)
	for header := range req.RequestHeaders {
		metadata["http_"+header] = req.RequestHeaders[header]
	}

	if rc := resp.GetResponseCode(); rc != nil {
		metadata["statusCode"] = fmt.Sprintf("%d", rc.GetValue())
	}

	metadata["ResponseCodeDetails"] = resp.ResponseCodeDetails
	// response headers are encoded with `sent_http_` prefix
	for header := range resp.ResponseHeaders {
		metadata["sent_http_"+header] = resp.ResponseHeaders[header]
	}
	for trailer := range resp.ResponseTrailers {
		metadata["sent_trailer_"+trailer] = resp.ResponseTrailers[trailer]
	}

	(*svc.outputChannel) <- &event.Raw{Quantity: 1, Metadata: metadata}
}

// StreamAccessLogs implements the access log service.
func (svc *AccessLogService) StreamAccessLogs(stream als.AccessLogService_StreamAccessLogsServer) error {
	var logName string
	for {
		msg, err := stream.Recv()
		var t int
		t = msg
		if err != nil {
			svc.logger.Errorf("error receive stream: %+v", err)
			continue
		}
		if msg.Identifier != nil {
			logName = msg.Identifier.LogName
		}
		// svc.logger.Info("msg: ", msg)
		switch entries := msg.LogEntries.(type) {
		case *als.StreamAccessLogsMessage_HttpLogs:
			for _, entry := range entries.HttpLogs.LogEntry {
				if entry != nil {
					svc.log(logName, entry)
				}
			}
		case *als.StreamAccessLogsMessage_TcpLogs:
			svc.logger.Warn("envoy LogEntry StreamAccessLogsMessage_TcpLogs not supported")
		}
	}
}
