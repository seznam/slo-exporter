package elasticsearch_ingester

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/seznam/slo-exporter/pkg/elasticsearch_client"
	"github.com/sirupsen/logrus"
	"regexp"
	"sync"
	"time"

	tailer_module "github.com/seznam/slo-exporter/pkg/tailer"
)

var (
	searchedDocuments = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "searched_documents_total",
		Help: "How many documents were retrieved from the elastic search",
	})
	documentsLeftMetric = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "documents_left",
		Help: "Number of documents still to be processed",
	})
	lastSearchTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "last_document_timestamp_seconds",
		Help: "Timestamp of the last processed document, next fetch will read since this timestamp",
	})
	missingRawLogField = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "missing_raw_log_filed_total",
		Help: "How many times defined raw log wasn't found in the document",
	})
	invalidRawLogFormat = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "raw_log_invalid_format_total",
		Help: "How many times the raw log had invalid format",
	})
	missingTimestampField = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "missing_timestamp_field_total",
		Help: "How many times the timestamp field was missing",
	})
	invalidTimestampFormat = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "invalid_timestamp_format_total",
		Help: "How many times the timestamp field had invalid format",
	})
)

func newTailer(logger logrus.FieldLogger, client elasticsearch_client.Client, index, timestampField, timestampFormat, rawLogField string, rawLogFormatRegexp, rawLogEmptyGroupRegexp *regexp.Regexp, query string, timeout time.Duration, maxBatchSize int) tailer {
	return tailer{
		client:                 client,
		index:                  index,
		timestampField:         timestampField,
		timestampFormat:        timestampFormat,
		rawLogField:            rawLogField,
		rawLogFormatRegexp:     rawLogFormatRegexp,
		rawLogEmptyGroupRegexp: rawLogEmptyGroupRegexp,
		lastTimestamp:          time.Now(),
		lastTimestampMtx:       sync.RWMutex{},
		maxBatchSize:           maxBatchSize,
		timeout:                timeout,
		query:                  query,
		logger:                 logger,
	}
}

type document struct {
	timestamp time.Time
	fields    map[string]string
}

type tailer struct {
	client                 elasticsearch_client.Client
	index                  string
	timestampField         string
	timestampFormat        string
	rawLogField            string
	rawLogFormatRegexp     *regexp.Regexp
	rawLogEmptyGroupRegexp *regexp.Regexp
	query                  string
	lastTimestamp          time.Time
	lastTimestampMtx       sync.RWMutex
	maxBatchSize           int
	timeout                time.Duration
	logger                 logrus.FieldLogger
}

func (t *tailer) setLastTimestamp(ts time.Time) {
	t.lastTimestampMtx.Lock()
	defer t.lastTimestampMtx.Unlock()
	t.lastTimestamp = ts
	lastSearchTimestamp.Set(float64(t.lastTimestamp.Unix()))
}

func (t *tailer) newDocumentFromJson(data json.RawMessage) (document, error) {
	newDoc := document{
		timestamp: time.Time{},
		fields:    map[string]string{},
	}

	var fields map[string]interface{}
	err := json.Unmarshal(data, &fields)
	if err != nil {
		return newDoc, fmt.Errorf("unable to unmarshall document body: %w", err)
	}
	for k, v := range fields {
		newDoc.fields[k] = fmt.Sprint(v)
	}

	if t.rawLogField != "" {
		rawLog, ok := newDoc.fields[t.rawLogField]
		if !ok {
			missingRawLogField.Inc()
			return newDoc, fmt.Errorf("document missing the raw log field %s", t.rawLogField)
		} else {
			rawLogFields, err := tailer_module.ParseLine(t.rawLogFormatRegexp, t.rawLogEmptyGroupRegexp, rawLog)
			if err != nil {
				invalidRawLogFormat.Inc()
				return newDoc, fmt.Errorf("document has invalid format of the raw log field %s", t.rawLogField)
			}
			for k, v := range rawLogFields {
				newDoc.fields[k] = v
			}
		}
	}

	timeFiled, ok := newDoc.fields[t.timestampField]
	if !ok {
		missingTimestampField.Inc()
		return newDoc, fmt.Errorf("document missing the timestamp field %s, using now instead", t.timestampField)
	} else {
		ts, err := time.Parse(t.timestampFormat, timeFiled)
		if err != nil {
			invalidTimestampFormat.Inc()
			return newDoc, fmt.Errorf("document has invalid timestamp field %s, using now instead", t.timestampField)
		}
		newDoc.timestamp = ts
	}
	return newDoc, nil
}

func (t *tailer) run(ctx context.Context, interval time.Duration) chan document {
	ticker := time.NewTicker(interval)
	outChan := make(chan document, t.maxBatchSize)
	go func() {
		defer ticker.Stop()
		defer close(outChan)
		shouldQuery := true
		documentsLeft := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				shouldQuery = true
			default:
			}
			if !shouldQuery {
				continue
			}
			var jsonDocs []json.RawMessage
			var err error
			jsonDocs, documentsLeft, err = t.client.RangeSearch(ctx, t.index, t.timestampField, t.lastTimestamp, t.maxBatchSize, t.query, t.timeout)
			if err != nil {
				t.logger.WithFields(logrus.Fields{"error": err, "since": t.lastTimestamp}).Error("failed to search data from elastic search")
				continue
			}
			documentsLeftMetric.Set(float64(documentsLeft))
			for _, jd := range jsonDocs {
				select {
				case <-ctx.Done():
					break
				default:
				}
				newDoc, err := t.newDocumentFromJson(jd)
				if err != nil {
					t.logger.WithFields(logrus.Fields{"error": err, "document": jd}).Errorf("failed to read document")
					continue
				}
				t.setLastTimestamp(newDoc.timestamp)
				searchedDocuments.Inc()
				outChan <- newDoc
			}
			if documentsLeft > 0 {
				t.logger.WithField("documents_behind", documentsLeft).Info("scheduling additional query to catch up with processing left documents")
				shouldQuery = true
			} else {
				shouldQuery = false
			}
		}
	}()
	return outChan
}
