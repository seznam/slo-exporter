package main

import (
	"context"
	"fmt"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/config"

	"github.com/gorilla/mux"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event_filter"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/dynamic_classifier"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/handler"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/normalizer"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/prober"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/prometheus_exporter"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/slo_event_producer"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/tailer"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/timescale_exporter"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

const (
	// global limit for unique eventKeys
	// TODO add to config once https://gitlab.seznam.net/Sklik-DevOps/slo-exporter/merge_requests/50 is merged
	prometheusExporterLimit int = 1000
	// same as above, but also duplicit with slo_event_producer/rule:eventKeyMetadataKey
	eventKeyLabel string = "event_key"
)

var (
	prometheusRegistry             = prometheus.DefaultRegisterer
	eventProcessingDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "slo_exporter",
			Name:      "event_processing_duration_seconds",
			Help:      "Duration histogram of event processing per module.",
			Buckets:   prometheus.ExponentialBuckets(0.0005, 5, 6),
		},
		[]string{"module"},
	)
)

func init() {
	prometheusRegistry.MustRegister(eventProcessingDurationSeconds)
}

func setupLogging(logLevel string) error {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	lvl, err := log.ParseLevel(logLevel)
	if err != nil {
		return err
	}
	log.SetLevel(lvl)
	return nil
}

func setupDefaultServer(listenAddr string, liveness *prober.Prober, readiness *prober.Prober, dch *handler.DynamicClassifierHandler) *http.Server {
	router := mux.NewRouter()
	router.Handle("/metrics", promhttp.Handler())
	router.HandleFunc("/liveness", liveness.HandleFunc)
	router.HandleFunc("/readiness", readiness.HandleFunc)
	// TODO: mby dump format by content-type?
	router.HandleFunc("/dynamic_classifier/matchers/{matcher}", dch.DumpCSV)
	return &http.Server{Addr: listenAddr, Handler: router}
}

// TODO FUSAKLA temporary workaround to multiplex one event to multiple channels, we should think if we can do better
func multiplexToChannels(srcChannel chan *event.Slo, dstChannels []chan *event.Slo) {
	for e := range srcChannel {
		for _, ch := range dstChannels {
			newEvent := e.Copy()
			ch <- &newEvent
		}
	}
	for _, ch := range dstChannels {
		close(ch)
	}
}

func main() {
	configFilePath := kingpin.Flag("config-file", "SLO exporter configuration file.").Required().ExistingFile()
	disableTimescale := kingpin.Flag("disable-timescale-exporter", "Do not start timescale exporter").Bool()
	disablePrometheus := kingpin.Flag("disable-prometheus-exporter", "Do not start prometheus exporter. (App runtime metrics are still exposed)").Bool()

	kingpin.Parse()

	conf := config.New()
	if err := conf.LoadFromFile(*configFilePath); err != nil {
		log.Fatalf("failed to load configuration file: %w", err)
	}

	if err := setupLogging(conf.LogLevel); err != nil {
		log.Fatalf("invalid specified log level %+v, error: %+v", conf.LogLevel, err)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	liveness := prober.NewLiveness()
	readiness := prober.NewReadiness()

	// shared error channel
	errChan := make(chan error, 10)
	gracefulShutdownChan := make(chan struct{}, 10)

	// Classify event by dynamic classifier
	dynamicClassifier, err := dynamic_classifier.NewFromViper(conf.MustModuleConfig("dynamicClassifier"))
	if err != nil {
		log.Fatalf("failed to initialize dynamic classifier: %+v", err)
	}
	dynamicClassifier.SetPrometheusObserver(eventProcessingDurationSeconds.WithLabelValues("dynamic_classifier"))
	dynamicClassifierHandler := handler.NewDynamicClassifierHandler(dynamicClassifier)

	// Start default server
	defaultServer := setupDefaultServer(conf.WebServerListenAddress, liveness, readiness, dynamicClassifierHandler)
	go func() {
		log.Infof("HTTP server listening on %+v", defaultServer.Addr)
		if err := defaultServer.ListenAndServe(); err != nil {
			errChan <- err
		}
		gracefulShutdownChan <- struct{}{}
	}()

	// Tail nginx logs and parse them to RequestEvent
	nginxTailer, err := tailer.NewFromViper(conf.MustModuleConfig("tailer"))
	if err != nil {
		log.Fatal(err)
	}
	nginxTailer.SetPrometheusObserver(eventProcessingDurationSeconds.WithLabelValues("tailer"))

	// Add the EntityKey to all RequestEvents
	requestNormalizer, err := normalizer.NewFromViper(conf.MustModuleConfig("normalizer"))
	if err != nil {
		log.Fatal(err)
	}
	requestNormalizer.SetPrometheusObserver(eventProcessingDurationSeconds.WithLabelValues("normalizer"))

	eventFilter, err := event_filter.NewFromViper(conf.MustModuleConfig("eventFilter"))
	if err != nil {
		log.Fatal(err)
	}
	eventFilter.SetPrometheusObserver(eventProcessingDurationSeconds.WithLabelValues("event_filter"))

	sloEventProducer, err := slo_event_producer.NewFromViper(conf.MustModuleConfig("sloEventProducer"))
	if err != nil {
		log.Fatalf("failed to load SLO rules conf: %+v", err)
	}
	sloEventProducer.SetPrometheusObserver(eventProcessingDurationSeconds.WithLabelValues("slo_event_producer"))

	//-- start enabled exporters
	var exporterChannels []chan *event.Slo

	if !*disablePrometheus {
		sloEventExporter, err := prometheus_exporter.NewFromViper(prometheusRegistry, sloEventProducer.PossibleMetadataKeys(), event.PossibleResults, conf.MustModuleConfig("prometheusExporter"))
		if err != nil {
			log.Fatalf("failed to load SLO rules conf: %+v", err)
		}
		sloEventExporter.SetPrometheusObserver(eventProcessingDurationSeconds.WithLabelValues("prometheus_exporter"))
		prometheusSloEventsChan := make(chan *event.Slo)
		exporterChannels = append(exporterChannels, prometheusSloEventsChan)
		sloEventExporter.Run(prometheusSloEventsChan)
	}

	if !*disableTimescale {
		timescaleExporter, err := timescale_exporter.NewFromViper(conf.MustModuleConfig("timescaleExporter"))
		if err != nil {
			log.Fatalf("failed to initialize timescale exporter: %+v", err)
		}
		timescaleSloEventsChan := make(chan *event.Slo)
		exporterChannels = append(exporterChannels, timescaleSloEventsChan)
		timescaleExporter.Run(timescaleSloEventsChan)
	}
	//--

	//-- start the rest of the pipeline

	// listen for OS signals
	sigChan := make(chan os.Signal, 3)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Pipeline definition
	nginxEventsChan := make(chan *producer.RequestEvent)
	nginxTailer.Run(ctx, nginxEventsChan, errChan)

	normalizedEventsChan := make(chan *producer.RequestEvent)
	requestNormalizer.Run(nginxEventsChan, normalizedEventsChan)

	filteredEventsChan := make(chan *producer.RequestEvent)
	eventFilter.Run(normalizedEventsChan, filteredEventsChan)

	classifiedEventsChan := make(chan *producer.RequestEvent)
	dynamicClassifier.Run(filteredEventsChan, classifiedEventsChan)

	sloEventsChan := make(chan *event.Slo)
	sloEventProducer.Run(classifiedEventsChan, sloEventsChan)

	// Replicate events to multiple channels
	go multiplexToChannels(sloEventsChan, exporterChannels)
	//--

	readiness.Ok()
	defer log.Info("see ya!")
	for {
		select {
		// TODO validate correctness of the graceful shutdown. Might be necessary to use wait group for verifying all modules are terminated.
		case <-gracefulShutdownChan:
			log.Info("gracefully shutting down")
			readiness.NotOk(fmt.Errorf("shutting down"))
			shutdownCtx, _ := context.WithTimeout(ctx, conf.GracefulShutdownTimeout)
			cancelFunc()
			if err := defaultServer.Shutdown(shutdownCtx); err != nil {
				log.Errorf("failed to gracefully shutdown HTTP server %+v. ", err)
			}
			log.Infof("waiting configured graceful shutdown timeout %s", conf.GracefulShutdownTimeout)
			shutdownCtx.Done()
			return
		case sig := <-sigChan:
			log.Infof("received signal %+v", sig)
			gracefulShutdownChan <- struct{}{}
		case err := <-errChan:
			log.Errorf("encountered error: %+v", err)
			gracefulShutdownChan <- struct{}{}
		}
	}

}
