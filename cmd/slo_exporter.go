package main

import (
	"context"
	"fmt"
	"runtime"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"

	"github.com/seznam/slo-exporter/pkg/config"
	"github.com/seznam/slo-exporter/pkg/dynamic_classifier"
	"github.com/seznam/slo-exporter/pkg/envoy_access_log_server"
	"github.com/seznam/slo-exporter/pkg/event_key_generator"
	"github.com/seznam/slo-exporter/pkg/event_metadata_renamer"
	"github.com/seznam/slo-exporter/pkg/kafka_ingester"
	"github.com/seznam/slo-exporter/pkg/metadata_classifier"
	"github.com/seznam/slo-exporter/pkg/pipeline"
	"github.com/seznam/slo-exporter/pkg/prometheus_exporter"
	"github.com/seznam/slo-exporter/pkg/prometheus_ingester"
	"github.com/seznam/slo-exporter/pkg/relabel"
	"github.com/seznam/slo-exporter/pkg/slo_event_producer"
	"github.com/seznam/slo-exporter/pkg/statistical_classifier"
	"github.com/seznam/slo-exporter/pkg/tailer"

	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/seznam/slo-exporter/pkg/prober"

	_ "net/http/pprof"
)

var (
	// Set using goreleaser ldflags during build, see https://goreleaser.com/environment/#using-the-mainversion
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"

	appName                   = "slo_exporter"
	prometheusRegistry        = prometheus.DefaultRegisterer
	wrappedPrometheusRegistry = prometheus.WrapRegistererWithPrefix(appName+"_", prometheusRegistry)
	appBuildInfo              = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "app_build_info",
		Help:        "Metadata metric with information about application build and version",
		ConstLabels: prometheus.Labels{"app": "slo-exporter", "version": version, "revision": commit, "build_date": date, "built_by": builtBy},
	})
)

func init() {
	appBuildInfo.Set(1)
	prometheusRegistry.MustRegister(appBuildInfo)
}

// Factory to instantiate pipeline modules
func moduleFactory(moduleName string, logger logrus.FieldLogger, conf *viper.Viper) (pipeline.Module, error) {
	switch moduleName {
	case "tailer":
		return tailer.NewFromViper(conf, logger)
	case "prometheusIngester":
		return prometheus_ingester.NewFromViper(conf, logger, version)
	case "kafkaIngester":
		return kafka_ingester.NewFromViper(conf, logger)
	case "envoyAccessLogServer":
		return envoy_access_log_server.NewFromViper(conf, logger)
	case "eventMetadataRenamer":
		return event_metadata_renamer.NewFromViper(conf, logger)
	case "relabel":
		return relabel.NewFromViper(conf, logger)
	case "eventKeyGenerator":
		return event_key_generator.NewFromViper(conf, logger)
	case "metadataClassifier":
		return metadata_classifier.NewFromViper(conf, logger)
	case "dynamicClassifier":
		return dynamic_classifier.NewFromViper(conf, logger)
	case "statisticalClassifier":
		return statistical_classifier.NewFromViper(conf, logger)
	case "sloEventProducer":
		return slo_event_producer.NewFromViper(conf, logger)
	case "prometheusExporter":
		return prometheus_exporter.NewFromViper(conf, logger)
	default:
		return nil, fmt.Errorf("unknown module %s", moduleName)
	}
}

func setupLogger(logLevel string, logFormat string) (*logrus.Logger, error) {
	lvl, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid log level '%s', must be one of 'error', 'warn', 'info', 'debug', 'trace'", logLevel)
	}

	newLogger := logrus.New()
	newLogger.SetOutput(os.Stdout)
	const timestampFormat = "2006-01-02T15:04:05.99999Z07:00"
	switch logFormat {
	case "json":
		newLogger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: timestampFormat,
		})
	case "text":
		newLogger.SetFormatter(&logrus.TextFormatter{
			DisableColors:   true,
			FullTimestamp:   true,
			TimestampFormat: timestampFormat,
		})
	default:
		return nil, fmt.Errorf("invalid log format '%s', must be 'json' or 'text'", logFormat)
	}
	newLogger.SetLevel(lvl)
	return newLogger, nil
}

func setupDefaultServer(listenAddr string, liveness *prober.Prober, readiness *prober.Prober, logger *logrus.Logger) (*http.Server, *mux.Router) {
	dynamicLoggingHandler := func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			lvl, err := logrus.ParseLevel(req.URL.Query().Get("level"))
			if err != nil {
				http.Error(w, "invalid specified logging level: "+err.Error(), http.StatusBadRequest)
				return
			}
			logger.SetLevel(lvl)
			_, _ = w.Write([]byte("logging level set to: " + lvl.String()))
			return
		}
		_, _ = w.Write([]byte("current logging level is: " + logger.Level.String()))
	}

	promHandler := promhttp.InstrumentMetricHandler(
		prometheus.DefaultRegisterer,
		promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}),
	)

	router := mux.NewRouter()
	router.Handle("/metrics", promHandler)
	router.HandleFunc("/liveness", liveness.HandleFunc)
	router.HandleFunc("/readiness", readiness.HandleFunc)
	router.HandleFunc("/logging", dynamicLoggingHandler)
	router.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
	return &http.Server{Addr: listenAddr, Handler: router}, router
}

func main() {
	// Enable mutex and block profiling
	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)

	configFilePath := kingpin.Flag("config-file", "SLO exporter configuration file.").ExistingFile()
	logLevel := kingpin.Flag("log-level", "Log level.").Envar("SLO_EXPORTER_LOGLEVEL").Default("info").Enum("error", "warn", "info", "debug", "trace")
	logFormat := kingpin.Flag("log-format", "Log format (text, json).").Envar("SLO_EXPORTER_LOGFORMAT").Default("text").Enum("json", "text")
	checkConfig := kingpin.Flag("check-config", "Only check config file and exit with 0 if ok and other status code if not.").Default("false").Bool()
	versionFlag := kingpin.Flag("version", "Display version.").Default("false").Bool()
	kingpin.Parse()

	// If version is requested, end here.
	if *versionFlag {
		fmt.Printf("slo_exporter version %s (from commit %s at %s by %s)\n", version, commit, date, builtBy)
		return
	}

	// Not using .Required() to allow for version flag to be used
	if *configFilePath == "" {
		fmt.Fprintln(os.Stderr, "error: required flag --config-file not provided, try --help")
		os.Exit(1)
	}

	logger, err := setupLogger(*logLevel, *logFormat)
	if err != nil {
		log.Fatalf("failed to setup logging, error: %s", err)
	}

	conf := config.New(logger.WithField("component", "config"))
	if err := conf.LoadFromFile(*configFilePath); err != nil {
		logger.Fatalf("failed to load configuration file: %v", err)
	}

	// Initialize the pipeline
	pipelineManager, err := pipeline.NewManager(moduleFactory, conf, logger.WithField("component", "pipeline_manager"))
	if err != nil {
		logger.Fatalf("failed to initialize the pipeline: %v", err)
	}

	// If configuration check is required, end here.
	if *checkConfig {
		logger.Info("Configuration is valid!")
		return
	}

	liveness, err := prober.NewLiveness(prometheusRegistry, logger.WithField("component", "prober"))
	if err != nil {
		logger.Fatalf("failed to initialize liveness prober: %v", err)
	}
	readiness, err := prober.NewReadiness(prometheusRegistry, logger.WithField("component", "prober"))
	if err != nil {
		logger.Fatalf("failed to initialize readiness prober: %v", err)
	}

	// shared error channel
	errChan := make(chan error, 10)
	gracefulShutdownRequestChan := make(chan struct{}, 10)

	// Start default server
	defaultServer, router := setupDefaultServer(conf.WebServerListenAddress, liveness, readiness, logger)
	go func() {
		logger.Infof("HTTP server listening on http://%+v", defaultServer.Addr)
		if err := defaultServer.ListenAndServe(); err != nil {
			errChan <- err
		}
	}()

	if err := pipelineManager.RegisterPrometheusMetrics(prometheusRegistry, wrappedPrometheusRegistry); err != nil {
		logger.Fatalf("failed to register pipeline metrics: %v", err)
	}
	pipelineManager.RegisterWebInterface(router)

	// Start the pipeline processing
	if err := pipelineManager.StartPipeline(); err != nil {
		logger.Fatalf("failed to start the pipeline: %v", err)
	}

	// listen for OS signals
	sigChan := make(chan os.Signal, 3)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	readiness.Ok()
	pipelineDoneCheck := time.NewTicker(time.Second)

	defer func() {
		pipelineDoneCheck.Stop()
		logger.Info("see you next time!")
	}()

	for {
		select {
		case <-gracefulShutdownRequestChan:
			logger.Info("gracefully shutting down")
			readiness.NotOk(fmt.Errorf("shutting down"))
			shutdownCtx, _ := context.WithTimeout(context.Background(), conf.MaximumGracefulShutdownDuration)

			<-pipelineManager.StopPipeline(shutdownCtx)
			// Add the delay after pipeline shutdown.
			delayedShutdownContext, _ := context.WithTimeout(shutdownCtx, conf.AfterPipelineShutdownDelay)
			// Wait until any of the context expires
			logger.Infof("waiting the configured delay %s after pipeline has finished", conf.AfterPipelineShutdownDelay)
			<-delayedShutdownContext.Done()

			if err := defaultServer.Shutdown(shutdownCtx); err != nil {
				logger.Errorf("failed to gracefully shutdown HTTP server %+v. ", err)
			}
			return
		case sig := <-sigChan:
			logger.Infof("received signal %+v", sig)
			gracefulShutdownRequestChan <- struct{}{}
		case err := <-errChan:
			logger.Errorf("encountered error: %+v", err)
			gracefulShutdownRequestChan <- struct{}{}
		case <-pipelineDoneCheck.C:
			if pipelineManager.Done() {
				logger.Info("finished processing all logs")
				gracefulShutdownRequestChan <- struct{}{}
			}
		}
	}

}
