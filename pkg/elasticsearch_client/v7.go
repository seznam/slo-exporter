package elasticsearch_client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	elasticV7 "github.com/olivere/elastic/v7"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

func NewV7Client(config Config, logger logrus.FieldLogger) (Client, error) {
	var clientCertFn func(*tls.CertificateRequestInfo) (*tls.Certificate, error)
	if config.ClientKeyFile != "" && config.ClientCertFile != "" {
		clientCertFn = func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			cert, err := tls.LoadX509KeyPair(config.ClientCertFile, config.ClientKeyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read client certs %s, %s: %w", config.ClientCertFile, config.ClientKeyFile, err)
			}
			return &cert, nil
		}
	}

	var clientCaCertPool *x509.CertPool
	if config.CaCertFile != "" {
		cert, err := ioutil.ReadFile(config.CaCertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read clientCaCertFile %s: %w", config.CaCertFile, err)
		}
		clientCaCertPool = x509.NewCertPool()
		clientCaCertPool.AppendCertsFromPEM(cert)
	}
	httpClient := http.Client{
		Transport: &http.Transport{
			ResponseHeaderTimeout: config.Timeout,
			DialContext:           (&net.Dialer{Timeout: config.Timeout}).DialContext,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify:   config.InsecureSkipVerify,
				GetClientCertificate: clientCertFn,
				ClientCAs:            clientCaCertPool,
			},
		},
		Timeout: config.Timeout,
	}
	opts := []elasticV7.ClientOptionFunc{
		elasticV7.SetHttpClient(&httpClient),
		elasticV7.SetErrorLog(logger),
		elasticV7.SetURL(config.Addresses...),
		elasticV7.SetScheme("https"),
		elasticV7.SetSniff(config.Sniffing),
		elasticV7.SetHealthcheck(config.Healtchecks),
	}
	if config.Debug {
		opts = append(opts, elasticV7.SetTraceLog(logger), elasticV7.SetInfoLog(logger))
	}
	if config.Username != "" || config.Password != "" {
		opts = append(opts, elasticV7.SetBasicAuth(config.Username, config.Password))
	}
	cli, err := elasticV7.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &v7Client{client: cli, logger: logger}, nil
}

type v7Client struct {
	logger logrus.FieldLogger
	client *elasticV7.Client
}

func (v *v7Client) RangeSearch(ctx context.Context, index, timestampField string, since time.Time, size int, query string, timeout time.Duration) ([]json.RawMessage, int, error) {
	filters := []elasticV7.Query{
		elasticV7.NewRangeQuery(timestampField).From(since),
	}
	if query != "" {
		filters = append(filters, elasticV7.NewQueryStringQuery(query))
	}
	q := elasticV7.NewBoolQuery().Filter(filters...)
	start := time.Now()
	result, err := v.client.Search().Index(index).TimeoutInMillis(int(timeout.Milliseconds())).Size(size).Sort(timestampField, true).Query(q).Do(ctx)
	if err != nil {
		ElasticApiCall.WithLabelValues("v7", "rangeSearch", err.Error()).Observe(time.Since(start).Seconds())
		return nil, 0, err
	}
	ElasticApiCall.WithLabelValues("v7", "rangeSearch", "").Observe(time.Since(start).Seconds())
	v.logger.WithFields(logrus.Fields{"index": index, "hits": len(result.Hits.Hits), "duration_ms": result.TookInMillis, "query": query, "since": since}).Debug("elastic search range search call")
	msgs := make([]json.RawMessage, len(result.Hits.Hits))
	for i, h := range result.Hits.Hits {
		msgs[i] = h.Source
	}
	return msgs, int(result.TotalHits()) - len(result.Hits.Hits), err
}
