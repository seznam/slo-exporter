package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/config"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/dynamic_classifier"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event_filter"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/normalizer"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/pipeline"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/prometheus_exporter"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/prometheus_ingester"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/slo_event_producer"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/statistical_classifier"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/tailer"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/prober"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	_ "net/http/pprof"
)

var (
	// Set using ldflags during build.
	buildVersion  = ""
	buildRevision = ""
	buildBranch   = ""
	buildTag      = ""

	appName                   = "slo_exporter"
	prometheusRegistry        = prometheus.DefaultRegisterer
	wrappedPrometheusRegistry = prometheus.WrapRegistererWithPrefix(appName+"_", prometheusRegistry)
	appBuildInfo              = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_build_info",
		Help: "Metadata metric with information about application build and version",
		ConstLabels: prometheus.Labels{"app": "slo-exporter", "version": buildVersion, "revision": buildRevision,
			"branch": buildBranch, "tag": buildTag, "standardized_metrics_version": "1.5.0"},
	})
)

func init() {
	appBuildInfo.Set(1)
	prometheusRegistry.MustRegister(appBuildInfo)
}

// Factory to instantiate pipeline modules
func moduleFactory(moduleName string, logger *logrus.Entry, conf *viper.Viper) (pipeline.Module, error) {
	switch moduleName {
	case "tailer":
		return tailer.NewFromViper(conf, logger)
	case "prometheusIngester":
		return prometheus_ingester.NewFromViper(conf, logger)
	case "normalizer":
		return normalizer.NewFromViper(conf, logger)
	case "eventFilter":
		return event_filter.NewFromViper(conf, logger)
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

func setupLogger(logLevel string) (*logrus.Logger, error) {
	lvl, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return nil, err
	}

	newLogger := logrus.New()
	newLogger.SetOutput(os.Stdout)
	newLogger.SetFormatter(&logrus.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	newLogger.SetLevel(lvl)
	return newLogger, nil
}

func setupDefaultServer(listenAddr string, liveness *prober.Prober, readiness *prober.Prober) (*http.Server, *mux.Router) {
	router := mux.NewRouter()
	router.Handle("/metrics", promhttp.Handler())
	router.HandleFunc("/liveness", liveness.HandleFunc)
	router.HandleFunc("/readiness", readiness.HandleFunc)
	router.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
	return &http.Server{Addr: listenAddr, Handler: router}, router
}

func main() {
	// Enable mutex and block profiling
	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)

	configFilePath := kingpin.Flag("config-file", "SLO exporter configuration file.").Required().ExistingFile()
	logLevel := kingpin.Flag("log-level", "Log level (error, warn, info, debug,trace).").Default("info").String()
	kingpin.Parse()
	envLogLevel, ok := syscall.Getenv("SLO_EXPORTER_LOGLEVEL")
	if ok {
		logLevel = &envLogLevel
	}

	logger, err := setupLogger(*logLevel)
	if err != nil {
		log.Fatalf("invalid specified log level %+v, error: %+v", logLevel, err)
	}

	conf := config.New(logger.WithField("component", "config"))
	if err := conf.LoadFromFile(*configFilePath); err != nil {
		logger.Fatalf("failed to load configuration file: %v", err)
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
	defaultServer, router := setupDefaultServer(conf.WebServerListenAddress, liveness, readiness)
	go func() {
		logger.Infof("HTTP server listening on http://%+v", defaultServer.Addr)
		if err := defaultServer.ListenAndServe(); err != nil {
			errChan <- err
		}
		gracefulShutdownRequestChan <- struct{}{}
	}()

	// Initialize the pipeline
	pipelineManager, err := pipeline.NewManager(moduleFactory, conf, logger.WithField("component", "pipeline_manager"))
	if err != nil {
		logger.Fatalf("failed to initialize the pipeline: %v", err)
	}
	if err := pipelineManager.RegisterPrometheusMetrics(prometheusRegistry, wrappedPrometheusRegistry); err != nil {
		logger.Fatalf("failed to register pipeline metrics: %v", err)
	}
	pipelineManager.RegisterWebInterface(router)

	// Start the pipeline items `processing
	pipelineManager.StartPipeline()

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
