package normalizer

import (
	"context"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"regexp"
	"strings"
)

const eventKeyFieldSeparator = ":"

var log *logrus.Entry
// Should replace IDs in the URL path such as `/user/1/info` but not to replace for example `/api/v1/`
var digitRegex = regexp.MustCompile("/[0-9]+")

func init() {
	log = logrus.WithField("component", "normalizer")
}

// NewForRequestEvent returns requestNormalizer which allows to add EventKey to RequestEvent
func NewForRequestEvent() *requestNormalizer {
	return &requestNormalizer{}
}

type requestNormalizer struct{}

func (rn *requestNormalizer) getNormalizedEventKey(event *producer.RequestEvent) string {
	var eventIdentifiers []string = []string{event.Method}
	eventIdentifiers = append(eventIdentifiers, digitRegex.ReplaceAllLiteralString(event.URL.Path, "/0"))
	// Append all values of 'operationName' parameter.
	operationNames, ok := event.URL.Query()["operationName"]
	if ok {
		for _, operation := range operationNames {
			eventIdentifiers = append(eventIdentifiers, operation)
		}
	}
	return strings.Join(eventIdentifiers, eventKeyFieldSeparator)
}

// Run event normalizer receiving events and filling their EventKey if not already filled.
func (rn *requestNormalizer) Run(ctx context.Context, inputEventsChan <-chan *producer.RequestEvent, outputEventsChan chan<- *producer.RequestEvent) {
	go func() {
		defer close(outputEventsChan)

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-inputEventsChan:
				if !ok {
					log.Info("input channel closed, finishing")
					return
				}
				if event.EventKey != "" {
					log.Debugf("skipping event normalization, already has EventKey: %s", event.EventKey)
					continue
				}
				event.EventKey = rn.getNormalizedEventKey(event)
				log.Infof("processed event with EventKey: %s", event.EventKey)
				outputEventsChan <- event
			}
		}
	}()
}
