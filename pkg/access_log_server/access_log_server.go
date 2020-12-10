package access_log_server

import (
	"fmt"
	"net"
	"time"

	"github.com/spf13/viper"

	"google.golang.org/grpc"

	"github.com/seznam/slo-exporter/pkg/event"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	logEntriesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "processed_logentries_total",
		Help: "Total number of processed log entries.",
	}, []string{"protocol", "api_version"})

	errorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "errors_total",
		Help: "Errors while processing the received logs.",
	}, []string{"type"})
)

type accessLogServerConfig struct {
	Address                 string
	GracefulShutdownTimeout time.Duration
}

type AccessLogServer struct {
	outputChannel           chan *event.Raw
	shutdownChannel         chan struct{}
	logger                  logrus.FieldLogger
	done                    bool
	server                  *grpc.Server
	service_v2              *AccessLogServiceV2
	service_v3              *AccessLogServiceV3
	address                 string
	gracefulShutdownTimeout time.Duration
}

func (als *AccessLogServer) String() string {
	return "accessLogServer"
}

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*AccessLogServer, error) {
	viperConfig.SetDefault("Address", ":18090")
	viperConfig.SetDefault("GracefulShutdownTimeout", 15*time.Second)
	var config accessLogServerConfig
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return New(config, logger)
}

// New returns an instance of AccessLogServer
func New(config accessLogServerConfig, logger logrus.FieldLogger) (*AccessLogServer, error) {
	als := AccessLogServer{
		address:                 config.Address,
		logger:                  logger,
		gracefulShutdownTimeout: config.GracefulShutdownTimeout,
	}
	return &als, nil
}

func (als *AccessLogServer) Run() {
	listen, err := net.Listen("tcp", als.address)
	if err != nil {
		als.logger.Fatalf("Error while starting the %s: %v", als, err)
	}
	// TODO: possibly add an UnknownServiceHandler
	als.server = grpc.NewServer()

	als.service_v2 = &AccessLogServiceV2{
		outChan: als.outputChannel,
		logger:  als.logger,
	}
	als.service_v2.Register(als.server)
	als.service_v3 = &AccessLogServiceV3{
		outChan: als.outputChannel,
		logger:  als.logger,
	}
	als.service_v3.Register(als.server)

	err = als.server.Serve(listen)
	if err != nil {
		als.logger.Fatalf("Error while starting the %s: %v", als, err)
	}
	go func() {
		<-als.shutdownChannel
		// Shutdown signal received, initiate the server shutdown
		als.server.GracefulStop()
		time.Sleep(als.gracefulShutdownTimeout)
		close(als.outputChannel)
		als.server.Stop()
		als.done = true
	}()
}

func (als *AccessLogServer) Stop() {
	if !als.done {
		als.shutdownChannel <- struct{}{}
	}
}

func (als *AccessLogServer) Done() bool {
	return als.done
}

func (als *AccessLogServer) RegisterMetrics(_ prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	toRegister := []prometheus.Collector{logEntriesTotal, errorsTotal}
	for _, collector := range toRegister {
		if err := wrappedRegistry.Register(collector); err != nil {
			return fmt.Errorf("error registering metric %s: %w", collector, err)
		}
	}
	return nil
}

func (als *AccessLogServer) OutputChannel() chan *event.Raw {
	return als.outputChannel
}
