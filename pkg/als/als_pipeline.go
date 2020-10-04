package als

import (
	"fmt"
	"net"

	alsv2 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v2"
	// "github.com/seznam/slo-exporter/pkg/als/als"
	"github.com/seznam/slo-exporter/pkg/event"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

type alsPipelineConfig struct {
	Port uint
}

type AccessLogServicePipeline struct {
	port          uint
	grpcServer    *grpc.Server
	outputChannel chan *event.Raw
	done          bool
	logger        logrus.FieldLogger
}

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*AccessLogServicePipeline, error) {
	viperConfig.SetDefault("Port", 18090)
	var config alsPipelineConfig
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return New(config, logger)
}

// New returns an instance of AccessLogServicePipeline
func New(config alsPipelineConfig, logger logrus.FieldLogger) (*AccessLogServicePipeline, error) {
	return &AccessLogServicePipeline{
		grpcServer:    grpc.NewServer(),
		outputChannel: make(chan *event.Raw),
		port:          config.Port,
		done:          false,
		logger:        logger,
	}, nil
}

func (a *AccessLogServicePipeline) Done() bool {
	return a.done
}

func (a *AccessLogServicePipeline) OutputChannel() chan *event.Raw {
	return a.outputChannel
}

func (a *AccessLogServicePipeline) Run() {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
	if err != nil {
		a.logger.WithError(err).Fatal("failed to listen")
	}

	als := &AccessLogService{
		logger:        a.logger,
		outputChannel: &(a.outputChannel),
	}
	alsv2.RegisterAccessLogServiceServer(a.grpcServer, als)
	a.logger.WithFields(logrus.Fields{"port": a.port}).Info("access log server listening")

	go func() {
		if err := a.grpcServer.Serve(listener); err != nil {
			a.logger.Error(err)
		}
	}()
}

func (a *AccessLogServicePipeline) Stop() {
	if !a.done {
		a.grpcServer.GracefulStop()
		close(a.outputChannel)
		a.done = true
	}
}
