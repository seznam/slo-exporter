package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/normalizer"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/prober"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/gorilla/mux"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/dynamic_classifier"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/handler"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/slo_event_producer"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/tailer"
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

func main() {
	logLevel := kingpin.Flag("log-level", "Set log level").Default("info").String()
	webServerListenAddr := kingpin.Flag("listen-address", "Listen address to listen on for web server.").Short('l').Default("0.0.0.0:8080").String()
	follow := kingpin.Flag("follow", "Follow the given log file.").Short('f').Bool()
	gracefulShutdownTimeout := kingpin.Flag("graceful-shutdown-timeout", "How long to wait for graceful shutdown.").Default("20s").Short('g').Duration()
	logFile := kingpin.Arg("logFile", "Path to log file to process").Required().String()
	sloDomain := kingpin.Flag("slo-domain", "slo domain name").Required().String()
	regexpClassificationFiles := kingpin.Flag("regexp-classification-file", "Path to regexp classification file.").ExistingFiles()
	exactClassificationFiles := kingpin.Flag("exact-classification-file", "Path to exact classification file.").ExistingFiles()
	sloRulesFile := kingpin.Flag("slo-rules-config", "Path to config with SLO rules for evaluation.").Required().ExistingFile()
	persistPositionFile := kingpin.Arg("persist-position-file", "File to be used to persist tailer position").Default("./.slo_exporter.pos").String()
	persistPositionInterval := kingpin.Arg("persist-position-interval", "Interval for persisting the file offset persistence").Default("2s").Duration()

	kingpin.Parse()

	if err := setupLogging(*logLevel); err != nil {
		log.Fatalf("invalid specified log level %v, error: %v", logLevel, err)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	liveness := prober.NewLiveness()
	readiness := prober.NewReadiness()

	// shared error channel
	errChan := make(chan error, 10)
	gracefulShutdownChan := make(chan struct{}, 10)

	// listen for OS signals
	sigChan := make(chan os.Signal, 3)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Classify event by dynamic classifier
	dynamicClassifier := dynamic_classifier.NewDynamicClassifier(*sloDomain)
	dynamicClassifierHandler := handler.NewDynamicClassifierHandler(dynamicClassifier)

	// Start default server
	defaultServer := setupDefaultServer(*webServerListenAddr, liveness, readiness, dynamicClassifierHandler)
	go func() {
		log.Infof("HTTP server listening on %v", defaultServer.Addr)
		if err := defaultServer.ListenAndServe(); err != nil {
			errChan <- err
		}
		gracefulShutdownChan <- struct{}{}
	}()

	// Tail nginx logs and parse them to RequestEvent
	nginxTailer, err := tailer.New(*logFile, *follow, *follow, *persistPositionFile, *persistPositionInterval)
	if err != nil {
		log.Fatal(err)
	}

	// Add the EntityKey to all RequestEvents
	requestNormalizer := normalizer.NewForRequestEvent()

	// load regexp matches
	if err := dynamicClassifier.LoadExactMatchesFromMultipleCSV(*exactClassificationFiles); err != nil {
		log.Fatalf("Failed to load classification: %v", err)
	}
	// load regex matches
	if err := dynamicClassifier.LoadRegexpMatchesFromMultipleCSV(*regexpClassificationFiles); err != nil {
		log.Fatalf("Failed to load classification: %v", err)
	}

	sloEventProducer, err := slo_event_producer.NewSloEventProducer(*sloRulesFile)
	if err != nil {
		log.Fatalf("failed to load SLO rules config: %v", err)
	}

	// Pipeline definition
	nginxEventsChan := make(chan *producer.RequestEvent)
	nginxTailer.Run(ctx, nginxEventsChan, errChan)

	normalizedEventsChan := make(chan *producer.RequestEvent)
	requestNormalizer.Run(ctx, nginxEventsChan, normalizedEventsChan)

	classifiedEventsChan := make(chan *producer.RequestEvent)
	dynamicClassifier.Run(ctx, normalizedEventsChan, classifiedEventsChan)

	sloEventsChan := make(chan *slo_event_producer.SloEvent)
	sloEventProducer.Run(ctx, classifiedEventsChan, sloEventsChan)

	readiness.Ok()
	defer log.Info("see ya!")
	for {
		select {
		// TODO validate correctness of the graceful shutdown. Might be necessary to use wait group for verifying all modules are terminated.
		case <-gracefulShutdownChan:
			log.Info("gracefully shutting down")
			readiness.NotOk(fmt.Errorf("shutting down"))
			shutdownCtx, _ := context.WithTimeout(ctx, *gracefulShutdownTimeout)
			cancelFunc()
			if err := defaultServer.Shutdown(shutdownCtx); err != nil {
				log.Errorf("failed to gracefully shutdown HTTP server %v. ", err)
			}
			log.Infof("waiting configured graceful shutdown timeout %v", gracefulShutdownTimeout)
			shutdownCtx.Done()
			return
		case sig := <-sigChan:
			log.Infof("received signal %v", sig)
			gracefulShutdownChan <- struct{}{}
		case err := <-errChan:
			log.Errorf("encountered error: %v", err)
			gracefulShutdownChan <- struct{}{}
		// TODO remove this, just for debugging now, reads the last channel and prints it out
		case event, ok := <-sloEventsChan:
			log.Infof("processed event: %v", event)
			if !ok {
				log.Info("finished classifying all events")
				gracefulShutdownChan <- struct{}{}
			}
		}
	}

}
