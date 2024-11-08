package envoy_access_log_server

import (
	"fmt"
	"net"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
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
	serverMetrics *grpc_prometheus.ServerMetrics
)

type accessLogServerConfig struct {
	Address                 string
	GracefulShutdownTimeout time.Duration
}

type AccessLogServer struct {
	outputChannel           chan *event.Raw
	logger                  logrus.FieldLogger
	done                    bool
	server                  *grpc.Server
	serviceV3               *AccessLogServiceV3
	address                 string
	gracefulShutdownTimeout time.Duration
}

func init() {
	serverMetrics = grpc_prometheus.NewServerMetrics()
}

func (als *AccessLogServer) String() string {
	return "envoyAccessLogServer"
}

func NewFromViper(viperConfig *viper.Viper, logger logrus.FieldLogger) (*AccessLogServer, error) {
	viperConfig.SetDefault("address", ":18090")
	viperConfig.SetDefault("gracefulShutdownTimeout", 5*time.Second)
	var config accessLogServerConfig
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return New(config, logger)
}

// New returns an instance of AccessLogServer.
func New(config accessLogServerConfig, logger logrus.FieldLogger) (*AccessLogServer, error) {
	als := AccessLogServer{
		outputChannel:           make(chan *event.Raw),
		logger:                  logger,
		address:                 config.Address,
		gracefulShutdownTimeout: config.GracefulShutdownTimeout,
	}
	return &als, nil
}

func (als *AccessLogServer) Run() {
	listen, err := net.Listen("tcp", als.address)
	if err != nil {
		als.logger.Fatalf("Error while starting the %s: %v", als, err)
	}
	als.server = grpc.NewServer(
		grpc.StreamInterceptor(serverMetrics.StreamServerInterceptor()),
	)

	als.serviceV3 = &AccessLogServiceV3{
		outChan: als.outputChannel,
		logger:  als.logger.WithField("EnvoyApiVersion", "3"),
	}
	als.serviceV3.Register(als.server)

	serverMetrics.InitializeMetrics(als.server)

	// Start the server
	go func() {
		err = als.server.Serve(listen)
		if err != nil {
			als.logger.Errorf("%s GRPC server fatal error, initiating graceful shutdown: %v", als, err)
			als.Stop()
		}
	}()
}

func (als *AccessLogServer) Stop() {
	go func() {
		// Initiate the server shutdown
		stopped := make(chan struct{})
		go func() {
			als.server.GracefulStop()
			close(stopped)
		}()
		t := time.NewTimer(als.gracefulShutdownTimeout)
		select {
		case <-t.C:
		case <-stopped:
		}
		als.server.Stop()
		close(als.outputChannel)
		als.done = true
	}()
}

func (als *AccessLogServer) Done() bool {
	return als.done
}

func (als *AccessLogServer) RegisterMetrics(_, wrappedRegistry prometheus.Registerer) error {
	toRegister := []prometheus.Collector{logEntriesTotal, errorsTotal}
	for _, collector := range toRegister {
		if err := wrappedRegistry.Register(collector); err != nil {
			return fmt.Errorf("error registering metric %s: %w", collector, err)
		}
	}
	if err := wrappedRegistry.Register(serverMetrics); err != nil {
		return fmt.Errorf("error registering metric %+v: %w", serverMetrics, err)
	}
	return nil
}

func (als *AccessLogServer) OutputChannel() chan *event.Raw {
	return als.outputChannel
}
