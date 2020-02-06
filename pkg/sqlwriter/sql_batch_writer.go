package sqlwriter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

var (
	batchWriteDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Subsystem: "sqlwriter",
		Name:      "batch_write_duration_seconds",
		Help:      "Histogram od duration inserting batch of data to SQL database.",
		Buckets:   prometheus.ExponentialBuckets(0.01, 3, 8),
	})
	writesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "sqlwriter",
		Name:      "batch_writes_total",
		Help:      "Total number of executed queries.",
	})
	batchSizeMetric = prometheus.NewGauge(prometheus.GaugeOpts{
		Subsystem: "sqlwriter",
		Name:      "batch_size",
		Help:      "Size of the last written batch.",
	})
	retriesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "sqlwriter",
		Name:      "retries_total",
		Help:      "Total number of retried queries.",
	})
	errorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: "sqlwriter",
		Name:      "errors_total",
		Help:      "Total number of errors encountered during querying.",
	}, []string{"type"})
)

type SqlWriter interface {
	Write(sql string, args ...interface{})
	WriteQueueSize() int
	RetryQueueSize() int
	Close(ctx context.Context) error
}

type parametrizedQuery struct {
	sql     string
	args    []interface{}
	retries int
	error   error
}

func (q *parametrizedQuery) String() string {
	return fmt.Sprintf("sql: `%s` args: %v retries: %d error: %v", q.sql, q.args, q.retries, q.error)
}

type queryBatch []*parametrizedQuery

func registerPrometheusMetrics(reg prometheus.Registerer) {
	if reg != nil {
		reg.MustRegister(batchWriteDurationSeconds, writesTotal, retriesTotal, errorsTotal, batchSizeMetric)
	}
}

func NewSqlBatchWriter(db *sql.DB, logger *logrus.Entry, promRegistry prometheus.Registerer, batchSize int, writeInterval, retryInterval time.Duration, retryLimit int) SqlWriter {
	registerPrometheusMetrics(promRegistry)
	writer := batchedSqlWriter{
		log:         logger.WithField("component", "sql_batch_writer"),
		dbMtx:       sync.Mutex{},
		db:          db,
		batchSize:   batchSize,
		retryLimit:  retryLimit,
		retryTicker: time.NewTicker(retryInterval),
		writeTicker: time.NewTicker(writeInterval),
		writeQueue:  make(chan *parametrizedQuery, 50000),
		retryQueue:  make(chan *parametrizedQuery, 10000),
	}
	go writer.startBatchedWrite(writer.writeTicker, writer.writeQueue, "write")
	go writer.startBatchedWrite(writer.retryTicker, writer.retryQueue, "retry")
	return &writer
}

type batchedSqlWriter struct {
	log         *logrus.Entry
	dbMtx       sync.Mutex
	db          *sql.DB
	batchSize   int
	retryLimit  int
	retryTicker *time.Ticker
	writeTicker *time.Ticker
	writeQueue  chan *parametrizedQuery
	retryQueue  chan *parametrizedQuery
}

func (w *batchedSqlWriter) readQueryBatch(bufCh chan *parametrizedQuery, batchSize int) (queryBatch, int, bool) {
	var batch queryBatch
Batching:
	for i := 0; i < batchSize; i++ {
		select {
		case item, ok := <-bufCh:
			if !ok {
				return batch, len(batch), true
			}
			batch = append(batch, item)
		default:
			break Batching
		}
	}
	return batch, len(batch), false
}

func (w *batchedSqlWriter) startBatchedWrite(ticker *time.Ticker, ch chan *parametrizedQuery, jobName string) {
	for _ = range ticker.C {
		batch, batchSize, closed := w.readQueryBatch(ch, w.batchSize)
		if batchSize == 0 {
			w.log.WithField("job", jobName).Debug("skipping retry, nothing to retry")
			continue
		}
		batchSizeMetric.Set(float64(batchSize))
		success, err := w.writeBatch(batch)
		if err != nil {
			w.log.WithField("job", jobName).Warnf("failed to retry batch of queries: %v", err)
		}
		if !success {
			w.log.WithField("job", jobName).Warnf("some of retried queries failed: %v", err)
		}
		if closed {
			ticker.Stop()
		}
	}
}

func (w *batchedSqlWriter) Write(sql string, args ...interface{}) {
	w.writeQueue <- &parametrizedQuery{sql: sql, args: args, retries: 0}
}

func (w *batchedSqlWriter) executeQuery(query *parametrizedQuery) error {
	if _, err := w.db.Exec(query.sql, query.args...); err != nil {
		query.error = err
		errorsTotal.WithLabelValues("executeQuery").Inc()
		return fmt.Errorf("failed to execute query: %w", err)
	}
	return nil
}

func (w *batchedSqlWriter) retryQuery(query *parametrizedQuery, err error) bool {
	// Add retryQuery reason to error
	query.error = multierror.Append(query.error, err)
	// Drop the query if the retry limit is exceeded
	if query.retries >= w.retryLimit {
		w.log.WithField("query", query).WithField("error", query.error).Errorf("query exceeded retry limit, dropping it")
		return false
	}
	// Add it to the retry queue
	query.retries++
	retriesTotal.Inc()
	w.retryQueue <- query
	return true
}

func (w *batchedSqlWriter) writeBatch(batch queryBatch) (bool, error) {
	w.dbMtx.Lock()
	defer w.dbMtx.Unlock()
	if len(batch) == 0 {
		return true, nil
	}
	timer := prometheus.NewTimer(batchWriteDurationSeconds)
	defer timer.ObserveDuration()
	writesTotal.Inc()
	transaction, err := w.db.Begin()
	if err != nil {
		errorsTotal.WithLabelValues("startTransaction").Inc()
		return false, fmt.Errorf("failed to begin transaction: %w", err)
	}

	var errs error = nil
	for _, query := range batch {
		// Execute query and schedule it for retry if failed
		if err := w.executeQuery(query); err != nil {
			errs = multierror.Append(errs, err)
			w.retryQuery(query, err)
		}
	}

	if err := transaction.Commit(); err != nil {
		// Schedule all writes for retryQuery if whole transaction failed
		for _, q := range batch {
			w.retryQuery(q, err)
		}
		errorsTotal.WithLabelValues("transactionCommit").Inc()
		return false, multierror.Append(errs, fmt.Errorf("failed to commit transaction: %w", err))
	}
	if errs != nil {
		errs = fmt.Errorf("some queries failed: %w", errs)
	}
	return true, errs
}

func (w *batchedSqlWriter) Close(ctx context.Context) error {
	w.log.Info("gracefully stopping")
	for {
		select {
		case <-ctx.Done():
			w.log.WithField("retry_queue_size", w.RetryQueueSize()).WithField("write_queue_size", w.WriteQueueSize()).Info("context exceeded for graceful stop")
			return errors.New("context deadline exceeded")
		default:
			if w.WriteQueueSize() == 0 && w.RetryQueueSize() == 0 {
				return nil
			}
			w.log.WithField("retry_queue_size", w.RetryQueueSize()).WithField("write_queue_size", w.WriteQueueSize()).Info("waiting for all queries to be processed")
			time.Sleep(time.Second)
		}

	}
}

func (w *batchedSqlWriter) WriteQueueSize() int {
	return len(w.writeQueue)
}
func (w *batchedSqlWriter) RetryQueueSize() int {
	return len(w.retryQueue)
}
