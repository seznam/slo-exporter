package normalizer

import (
	"context"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"path"
	"regexp"
	"strings"
)

const eventKeyFieldSeparator = ":"
const numberPlaceholder = "0"
const pathItemsSeparator = "/"

var log *logrus.Entry

// Should replace IDs in the URL path such as `/user/1/info` but not to replace for example `/api/v1/`
var followingDigitsRegex = regexp.MustCompile("[0-9]+")
var onlyDigitsRegex = regexp.MustCompile("^[0-9]+$")

func init() {
	log = logrus.WithField("component", "normalizer")
}

// NewForRequestEvent returns requestNormalizer which allows to add EventKey to RequestEvent
func NewForRequestEvent() *requestNormalizer {
	return &requestNormalizer{}
}

type requestNormalizer struct{}


// Normalizes the URL path, applies those rules:
//  1. If path is empty returns `/`.
//  2. If path is non empty, the trailing `/` is removed.
//  3. Only digit sequences between slashes are replaced with placeholder such as `/foo/123/bar` -> `/foo/<placeholder>/bar`.
//  4. Last part of the path has all digit sequences replaced with the placeholder such as `/foo/banner-50x60.png` -> `/foo/banner-<placeholder>x<placeholder>.png`
func (rn *requestNormalizer) normalizePath(rawPath string) string {
	if rawPath == "" {
		return "/"
	}
	pathItems := strings.Split(path.Clean(rawPath), pathItemsSeparator)
	itemsCount := len(pathItems)
	for i, item := range pathItems {
		// In last part of the rawPath replace all numbers for zero
		if i+1 == itemsCount {
			pathItems[i] = followingDigitsRegex.ReplaceAllString(item, numberPlaceholder)
			continue
		}
		// Replace all only number items in the rawPath with placeholder
		pathItems[i] = onlyDigitsRegex.ReplaceAllString(item, numberPlaceholder)
	}
	return strings.Join(pathItems, pathItemsSeparator)
}

func (rn *requestNormalizer) getNormalizedEventKey(event *producer.RequestEvent) string {
	var eventIdentifiers = []string{event.Method}
	eventIdentifiers = append(eventIdentifiers, rn.normalizePath(event.URL.Path))
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
		defer log.Info("stopping normalizer")

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
				log.Debugf("processed event with EventKey: %s", event.EventKey)
				outputEventsChan <- event
			}
		}
	}()
}
