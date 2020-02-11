package main

import (
	"context"
	"fmt"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/config"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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
func multiplexToChannels(srcChannel chan *slo_event_producer.SloEvent, dstChannels []chan *slo_event_producer.SloEvent) {
	for e := range srcChannel {
		for _, ch := range dstChannels {
			// Needs copy because we are not concurrent safe (probably accessing metadata).
			newE := slo_event_producer.SloEvent{
				TimeOccurred: e.TimeOccurred,
				SloMetadata:  map[string]string{},
				Result:       e.Result,
			}
			for k, v := range e.SloMetadata {
				newE.SloMetadata[k] = v
			}
			ch <- &newE
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

	config := config.New()
	if err := config.LoadFromFile(*configFilePath); err != nil {
		log.Fatalf("failed to load configuration file: %w", err)
	}

	if err := setupLogging(config.LogLevel); err != nil {
		log.Fatalf("invalid specified log level %v, error: %v", config.LogLevel, err)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	liveness := prober.NewLiveness()
	readiness := prober.NewReadiness()

	// shared error channel
	errChan := make(chan error, 10)
	gracefulShutdownChan := make(chan struct{}, 10)

	// Classify event by dynamic classifier
	dynamicClassifier, err := dynamic_classifier.NewFromViper(config.MustModuleConfig("dynamicClassifier"))
	if err != nil {
		log.Fatalf("failed to initialize dynamic classifier: %v", err)
	}
	dynamicClassifierHandler := handler.NewDynamicClassifierHandler(dynamicClassifier)

	// Start default server
	defaultServer := setupDefaultServer(config.WebServerListenAddress, liveness, readiness, dynamicClassifierHandler)
	go func() {
		log.Infof("HTTP server listening on %v", defaultServer.Addr)
		if err := defaultServer.ListenAndServe(); err != nil {
			errChan <- err
		}
		gracefulShutdownChan <- struct{}{}
	}()

	// Tail nginx logs and parse them to RequestEvent
	nginxTailer, err := tailer.NewFromViper(config.MustModuleConfig("tailer"))
	if err != nil {
		log.Fatal(err)
	}

	// Add the EntityKey to all RequestEvents
	requestNormalizer := normalizer.NewForRequestEvent()

	eventFilter, err := event_filter.NewFromViper(config.MustModuleConfig("eventFilter"))
	if err != nil {
		log.Fatal(err)
	}

	sloEventProducer, err := slo_event_producer.NewFromViper(config.MustModuleConfig("sloEventProducer"))
	if err != nil {
		log.Fatalf("failed to load SLO rules config: %v", err)
	}

	//-- start enabled exporters
	var exporterChannels []chan *slo_event_producer.SloEvent

	if !*disablePrometheus {
		sloEventExporter := prometheus_exporter.New(sloEventProducer.PossibleMetadataKeys(), slo_event_producer.EventResults)
		prometheusSloEventsChan := make(chan *slo_event_producer.SloEvent)
		exporterChannels = append(exporterChannels, prometheusSloEventsChan)
		sloEventExporter.Run(prometheusSloEventsChan)
	}

	if !*disableTimescale {
		timescaleExporter, err := timescale_exporter.NewFromViper(config.MustModuleConfig("timescaleExporter"))
		if err != nil {
			log.Fatalf("failed to initialize timescale exporter: %v", err)
		}
		timescaleSloEventsChan := make(chan *slo_event_producer.SloEvent)
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

	sloEventsChan := make(chan *slo_event_producer.SloEvent)
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
			shutdownCtx, _ := context.WithTimeout(ctx, config.GracefulShutdownTimeout)
			cancelFunc()
			if err := defaultServer.Shutdown(shutdownCtx); err != nil {
				log.Errorf("failed to gracefully shutdown HTTP server %v. ", err)
			}
			log.Infof("waiting configured graceful shutdown timeout %s", config.GracefulShutdownTimeout)
			shutdownCtx.Done()
			return
		case sig := <-sigChan:
			log.Infof("received signal %v", sig)
			gracefulShutdownChan <- struct{}{}
		case err := <-errChan:
			log.Errorf("encountered error: %v", err)
			gracefulShutdownChan <- struct{}{}
		}
	}

}
