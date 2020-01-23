package main

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/normalizer"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/prober"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

func setupDefaultServer(listenAddr string, liveness *prober.Prober, readiness *prober.Prober) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/liveness", liveness.HandleFunc)
	mux.HandleFunc("/readiness", readiness.HandleFunc)
	return &http.Server{Addr: listenAddr, Handler: mux,}
}

func main() {
	logLevel := kingpin.Flag("log-level", "Set log level").Default("info").String()
	webServerListenAddr := kingpin.Flag("listen-address", "Listen address to listen on for web server.").Short('l').Default("0.0.0.0:8080").String()
	follow := kingpin.Flag("follow", "Follow the given log file.").Short('f').Bool()
	gracefulShutdownTimeout := kingpin.Flag("graceful-shutdown-timeout", "How long to wait for graceful shutdown.").Default("20s").Short('g').Duration()
	logFile := kingpin.Arg("logFile", "Path to log file to process").Required().String()
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

	// Start default server
	defaultServer := setupDefaultServer(*webServerListenAddr, liveness, readiness)
	go func() {
		log.Infof("HTTP server listening on %v", defaultServer.Addr)
		if err := defaultServer.ListenAndServe(); err != nil {
			errChan <- err
		}
		gracefulShutdownChan <- struct{}{}
	}()

	// Tail nginx logs and parse them to RequestEvent
	nginxTailer, err := tailer.New(*logFile, *follow, *follow)
	if err != nil {
		log.Fatal(err)
	}
	nginxEventsChan := make(chan *producer.RequestEvent)
	nginxTailer.Run(ctx, nginxEventsChan, errChan)

	// Add the EntityKey to all RequestEvents
	requestNormalizer := normalizer.NewForRequestEvent()
	normalizedEventsChan := make(chan *producer.RequestEvent)
	requestNormalizer.Run(ctx, nginxEventsChan, normalizedEventsChan)

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
		case _, ok := <-normalizedEventsChan:
			if !ok {
				log.Info("finished processing all logs")
				gracefulShutdownChan <- struct{}{}
			}
		case sig := <-sigChan:
			log.Infof("received signal %v", sig)
			gracefulShutdownChan <- struct{}{}
		case err := <-errChan:
			log.Errorf("encountered error: %v", err)
			gracefulShutdownChan <- struct{}{}
		}
	}

}
