package normalizer

import (
	"path"
	"regexp"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

const (
	eventKeyFieldSeparator = ":"
	numberPlaceholder      = "0"
	ipPlaceholder          = ":ip"
	hashPlaceholder        = ":hash"
	uuidPlaceholder        = ":uuid"
	imagePlaceholder       = ":image"
	fontPlaceholder        = ":font"
	pathItemsSeparator     = "/"
)

type normalizator struct {
	pattern     *regexp.Regexp
	replacement string
}

func (n *normalizator) normalize(path string) string {
	if n.pattern.MatchString(path) {
		return n.replacement
	}
	return path
}

var (
	log                  *logrus.Entry
	followingDigitsRegex = regexp.MustCompile("[0-9]+")
	imageExtensionRegex  = regexp.MustCompile(`(?i)\.(?:png|jpg|jpeg|svg|tif|tiff|gif)$`)
	fontExtensionRegex   = regexp.MustCompile(`(?i)\.(?:ttf|woff)$`)

	customPatterns = []normalizator{
		{pattern: regexp.MustCompile(`/api/v1/ppchit/rule/[0-9a-fA-F]{5,16}`), replacement: "/api/v1/ppchit/rule/0"},
		{pattern: regexp.MustCompile(`/campaigns/\d+/groups/\d+/placements/automatic/(\w[\w-]+\.)+\w{2,}/urls`), replacement: "/campaigns/0/groups/0/placements/automatic/:domain/urls"},
	}
)

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
//  4. Only digit sequences (including hexadecimal), hashes, uuids and IPs are replaced with placeholder such as `/foo/123/bar` -> `/foo/<placeholder>/bar`.
//  5. All image names on the last position are replaced with placeholder such as `/foo/bar.png` -> `/foo/<placeholder>`
//  6. Last part of the path has all digit sequences replaced with the placeholder such as `/foo/banner-50x60.info` -> `/foo/banner-<placeholder>x<placeholder>.info`
func (rn *requestNormalizer) normalizePath(rawPath string) string {
	if rawPath == "" {
		return "/"
	}
	for _, norm := range customPatterns {
		rawPath = norm.normalize(rawPath)
	}
	pathItems := strings.Split(path.Clean(rawPath), pathItemsSeparator)
	itemsCount := len(pathItems)
	for i, item := range pathItems {
		if item == "" {
			continue
		}

		if govalidator.IsMD5(item) || govalidator.IsSHA1(item) || govalidator.IsSHA256(item) {
			pathItems[i] = hashPlaceholder
			continue
		}
		if govalidator.IsNumeric(item) || govalidator.IsHexadecimal(item) {
			pathItems[i] = numberPlaceholder
			continue
		}

		if govalidator.IsUUID(item) || govalidator.IsUUIDv4(item) {
			pathItems[i] = uuidPlaceholder
			continue
		}

		if govalidator.IsIP(item) {
			pathItems[i] = ipPlaceholder
			continue
		}

		// replace all numbers with zero in the last part of the rawPath
		if i+1 == itemsCount {
			if imageExtensionRegex.MatchString(item) {
				pathItems[i] = imagePlaceholder
				continue
			}
			if fontExtensionRegex.MatchString(item) {
				pathItems[i] = fontPlaceholder
				continue
			}
			pathItems[i] = followingDigitsRegex.ReplaceAllString(item, numberPlaceholder)
			continue
		}
	}
	return strings.Join(pathItems, pathItemsSeparator)
}

func (rn *requestNormalizer) getNormalizedEventKey(event *producer.RequestEvent) string {
	var eventIdentifiers = []string{event.Method}
	eventIdentifiers = append(eventIdentifiers, rn.normalizePath(event.URL.Path))
	// Append all values of 'operationName' parameter.
	// TODO possibly have this in a configuration
	operationNames, ok := event.URL.Query()["operationName"]
	if ok {
		for _, operation := range operationNames {
			eventIdentifiers = append(eventIdentifiers, operation)
		}
	}
	return strings.Join(eventIdentifiers, eventKeyFieldSeparator)
}

// Run event normalizer receiving events and filling their EventKey if not already filled.
func (rn *requestNormalizer) Run(inputEventsChan <-chan *producer.RequestEvent, outputEventsChan chan<- *producer.RequestEvent) {
	go func() {
		defer close(outputEventsChan)
		defer log.Info("stopping...")

		for {
			select {
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
