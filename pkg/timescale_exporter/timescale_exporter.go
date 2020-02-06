package timescale_exporter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/slo_event_producer"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/sqlwriter"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	component             = "timescale_exporter"
	sloEventsMetricName   = "timescale_slo_events_total"
	timescaleMetricsTable = "metrics"
	sloResultLabel        = "result"
)

var (
	log *logrus.Entry
)

func init() {
	log = logrus.WithField("component", component)
}

type TimescaleExporter struct {
	statistics      map[string]*timescaleMetric
	statisticsMutex sync.Mutex
	lastPushTime    time.Time
	pushTicker      *time.Ticker
	config          TimescaleConfig
	db              *sql.DB
	sqlWriter       sqlwriter.SqlWriter
}

type timescaleMetric struct {
	value         float64
	lastPushTime  time.Time
	lastEventTime time.Time
}

func checkDatabase(db *sql.DB, checkTimeout time.Duration, checkInterval time.Duration, expectedTable string) error {
	start := time.Now()
	checkFailed := errors.New("failed to connect to timescale db")
	for time.Since(start) < checkTimeout {
		var table sql.NullString
		// SQL to check if table exists in the PostgreSQL
		row := db.QueryRow("SELECT to_regclass($1);", expectedTable)
		err := row.Scan(&table)
		if err != nil {
			log.Errorf("failed to connect: %v", err)
		} else if !table.Valid {
			log.Error("table metrics is missing")
		} else {
			checkFailed = nil
			break
		}
		time.Sleep(checkInterval)
	}
	return checkFailed
}

func NewFromFile(path string) (*TimescaleExporter, error) {
	newConfig := TimescaleConfig{}
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration file: %w", err)
	}
	err = yaml.UnmarshalStrict(yamlFile, &newConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall configuration file: %w", err)
	}
	return NewFromConfig(newConfig)
}

func NewFromConfig(config TimescaleConfig) (*TimescaleExporter, error) {
	db, err := sql.Open("postgres", config.psqlInfo())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to timescale db: %w", err)
	}
	// Check the database is running and has expected schema.
	if err := checkDatabase(db, config.DbInitTimeout, config.DbInitCheckInterval, timescaleMetricsTable); err != nil {
		return nil, err
	}
	return &TimescaleExporter{
		statisticsMutex: sync.Mutex{},
		statistics:      map[string]*timescaleMetric{},
		lastPushTime:    time.Time{},
		pushTicker:      time.NewTicker(config.MaximumPushInterval),
		config:          config,
		db:              db,
		sqlWriter:       sqlwriter.NewSqlBatchWriter(db, log, prometheus.WrapRegistererWithPrefix("slo_exporter_", prometheus.DefaultRegisterer), config.DbBatchWriteSize, config.DbWriteInterval, config.DbWriteRetryInterval, config.DbWriteRetryLimit),
	}, nil
}

func (ts *TimescaleExporter) Close(ctx context.Context) error {
	var errs error
	if err := ts.sqlWriter.Close(ctx); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err := ts.db.Close(); err != nil {
		errs = multierror.Append(errs, err)
	}
	return errs
}

func encodePrometheusMetric(labels string, value float64, metricTime time.Time) string {
	timestamp := metricTime.UnixNano() / int64(time.Millisecond)
	return fmt.Sprintf("%s%s %g %d", sloEventsMetricName, labels, value, timestamp)
}

func (ts *TimescaleExporter) shouldBeMetricPushed(evaluationTime time.Time, metric *timescaleMetric) bool {
	// We want to push only if the last push is longer than MaximumPushInterval or the value changes since last push. Otherwise continue.
	return evaluationTime.Sub(metric.lastPushTime) > ts.config.MaximumPushInterval || metric.lastEventTime.After(metric.lastPushTime)
}

func newerTime(original, new time.Time) time.Time {
	if new.After(original) {
		return new
	}
	return original
}

func renderSqlInsert(labels string, value float64, eventTime time.Time) string {
	metricString := encodePrometheusMetric(labels, value, eventTime)
	return fmt.Sprintf("INSERT INTO %s VALUES ('%s');", timescaleMetricsTable, metricString)
}

func (ts *TimescaleExporter) pushMetricsWithTimestamp(eventTime time.Time) {
	for labels, metric := range ts.statistics {
		if !ts.shouldBeMetricPushed(eventTime, metric) {
			continue
		}
		preparedSql := renderSqlInsert(labels, metric.value, eventTime)
		ts.sqlWriter.Write(preparedSql)
		metric.lastPushTime = newerTime(metric.lastPushTime, eventTime)
	}
}

func (ts *TimescaleExporter) pushAllMetricsWithOffset(offset time.Duration) {
	for labels, metric := range ts.statistics {
		newPushTime := metric.lastPushTime.Add(offset)
		preparedSql := renderSqlInsert(labels, metric.value, newPushTime)
		ts.sqlWriter.Write(preparedSql)
		metric.lastPushTime = newerTime(metric.lastPushTime, newPushTime)
	}
}

func (ts *TimescaleExporter) Run(input <-chan *slo_event_producer.SloEvent) {
	go func() {
		defer ts.Close(context.Background())
		defer log.Info("stopping...")
		for {
			select {
			case event, ok := <-input:
				if !ok {
					log.Info("input channel closed, finishing")
					return
				}
				log.Debugf("processing event %s", event)
				ts.processEvent(event)
				if event.TimeOccurred.Sub(ts.lastPushTime) > ts.config.UpdatedMetricPushInterval {
					ts.pushMetricsWithTimestamp(event.TimeOccurred)
					ts.lastPushTime = newerTime(ts.lastPushTime, event.TimeOccurred)
				}
			case <-ts.pushTicker.C:
				log.Debugf("full sync push")
				ts.pushAllMetricsWithOffset(ts.config.MaximumPushInterval)
				ts.lastPushTime = ts.lastPushTime.Add(ts.config.MaximumPushInterval)
			}
		}
	}()
}

func metadataToString(metadata map[string]string) string {
	var labelValuePairs []string
	for k, v := range metadata {
		labelValuePairs = append(labelValuePairs, fmt.Sprintf("%s=%q", k, v))
	}
	sort.Strings(labelValuePairs)
	return "{" + strings.Join(labelValuePairs, ",") + "}"
}

func (ts *TimescaleExporter) initializeMetricsIfNotExist(metadata map[string]string) {
	key := metadataToString(metadata)
	newCounter, ok := ts.statistics[key]
	if !ok {
		newCounter = &timescaleMetric{
			value:         0,
			lastPushTime:  time.Time{},
			lastEventTime: time.Time{},
		}
		ts.statistics[key] = newCounter
	}
}

func (ts *TimescaleExporter) processEvent(event *slo_event_producer.SloEvent) {
	ts.statisticsMutex.Lock()
	defer ts.statisticsMutex.Unlock()
	for _, possibleResult := range slo_event_producer.EventResults {
		newMetadata := event.SloMetadata
		newMetadata[sloResultLabel] = string(possibleResult)
		ts.initializeMetricsIfNotExist(newMetadata)
	}
	event.SloMetadata[sloResultLabel] = string(event.Result)
	counter := ts.statistics[metadataToString(event.SloMetadata)]
	counter.value++
	counter.lastEventTime = event.TimeOccurred
}
