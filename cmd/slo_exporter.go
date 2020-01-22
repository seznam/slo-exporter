package main

import (
	"context"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/normalizer"
	"os"

	log "github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/tailer"
)

func setupLogging(debug bool) {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	if debug {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	verbose := kingpin.Flag("verbose", "Enable verbose logging.").Short('v').Bool()
	follow := kingpin.Flag("follow", "Follow the given log file.").Short('f').Bool()
	logFile := kingpin.Arg("logFile", "Path to log file to process").Required().String()
	kingpin.Parse()

	setupLogging(*verbose)

	eventCount := 0

	// shared error channel
	errChan := make(chan error)
	go func() {
		for err := range errChan {
			log.Error(err)
		}
	}()

	// done chan is used to signal individual stages of a pipeline to quit
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	reopen := follow
	nginxTailer, err := tailer.New(*logFile, *follow, *reopen)
	if err != nil {
		log.Fatal(err)
	}

	requestNormalizer := normalizer.NewForRequestEvent()

	nginxEventsChan := make(chan *producer.RequestEvent)
	nginxTailer.Run(ctx, nginxEventsChan, errChan)

	normalizedEventsChan := make(chan *producer.RequestEvent)
	requestNormalizer.Run(ctx, nginxEventsChan, normalizedEventsChan)

	for event := range normalizedEventsChan {
		eventCount += 1
		log.Debug(event)
	}

	log.Infof("Exiting. Total number of events processed: %d", eventCount)
}
