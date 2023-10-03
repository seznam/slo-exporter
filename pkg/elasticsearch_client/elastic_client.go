package elasticsearch_client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"time"
)

var (
	ElasticApiCall = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "elasticsearch_request_seconds",
		Help:    "Duration histogram of elasticsearch api calls",
		Buckets: prometheus.ExponentialBuckets(0.1, 2, 5),
	}, []string{"api_version", "endpoint", "error"})
)

type Config struct {
	Addresses          []string
	Username           string
	Password           string
	Timeout            time.Duration
	Healtchecks        bool
	Sniffing           bool
	InsecureSkipVerify bool
	ClientCertFile     string
	ClientKeyFile      string
	CaCertFile         string
	Debug              bool
}

type Client interface {
	RangeSearch(ctx context.Context, index, timestampField string, since time.Time, size int, query string, timeout time.Duration) ([]json.RawMessage, int, error)
}

var clientFactory = map[string]func(config Config, logger logrus.FieldLogger) (Client, error){
	"v7": NewV7Client,
}

func NewClient(version string, config Config, logger logrus.FieldLogger) (Client, error) {
	factoryFn, ok := clientFactory[version]
	if !ok {
		var supportedValues []string
		for k, _ := range clientFactory {
			supportedValues = append(supportedValues, k)
		}
		return nil, fmt.Errorf("unsupported Elasticsearch API version %s, only supported values are: %s", version, supportedValues)
	}
	return factoryFn(config, logger)
}
