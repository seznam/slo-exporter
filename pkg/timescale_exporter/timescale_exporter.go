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
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/shutdown_handler"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/sqlwriter"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"os"
	"sync"
	"time"
)

const (
	component             = "timescale_exporter"
	timescaleMetricsTable = "metrics"
)

var (
	log *logrus.Entry
)

func init() {
	log = logrus.WithField("component", component)
}

type TimescaleExporter struct {
	instanceName    string
	metricName      string
	labelNames      labelsNamesConfig
	statistics      map[string]*timescaleMetric
	statisticsMutex sync.Mutex
	lastPushTime    time.Time
	pushTicker      *time.Ticker
	config          Config
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
			log.Errorf("failed to connect: %+v", err)
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

func NewFromViper(viperConfig *viper.Viper) (*TimescaleExporter, error) {
	viperConfig.SetDefault("Instance", os.Getenv("HOSTNAME"))
	viperConfig.SetDefault("DbInitCheckInterval", time.Minute)
	viperConfig.SetDefault("DbBatchWriteSize", 1000)
	viperConfig.SetDefault("DbWriteInterval", 30*time.Second)
	viperConfig.SetDefault("DbWriteRetryInterval", time.Minute)
	viperConfig.SetDefault("DbWriteRetryLimit", 3)
	var config Config
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshall configuration file: %w", err)
	}
	return New(config)
}

func New(config Config) (*TimescaleExporter, error) {
	db, err := sql.Open("postgres", config.psqlInfo())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to timescale db: %w", err)
	}
	// Check the database is running and has expected schema.
	if err := checkDatabase(db, config.DbInitTimeout, config.DbInitCheckInterval, timescaleMetricsTable); err != nil {
		return nil, err
	}
	return &TimescaleExporter{
		instanceName:    config.Instance,
		metricName:      config.metricName,
		labelNames:      config.LabelNames,
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

func (ts *TimescaleExporter) encodePrometheusMetric(labels string, value float64, metricTime time.Time) string {
	timestamp := metricTime.UnixNano() / int64(time.Millisecond)
	return fmt.Sprintf("%s{%s} %g %d", ts.metricName, labels, value, timestamp)
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

func (ts *TimescaleExporter) renderSqlInsert(labels string, value float64, eventTime time.Time) string {
	metricString := ts.encodePrometheusMetric(labels, value, eventTime)
	return fmt.Sprintf("INSERT INTO %s VALUES ('%s');", timescaleMetricsTable, metricString)
}

func (ts *TimescaleExporter) pushMetricsWithTimestamp(eventTime time.Time) {
	for labels, metric := range ts.statistics {
		if !ts.shouldBeMetricPushed(eventTime, metric) {
			continue
		}
		preparedSql := ts.renderSqlInsert(labels, metric.value, eventTime)
		ts.sqlWriter.Write(preparedSql)
		metric.lastPushTime = newerTime(metric.lastPushTime, eventTime)
	}
}

func (ts *TimescaleExporter) pushAllMetricsWithOffset(offset time.Duration) {
	for labels, metric := range ts.statistics {
		newPushTime := metric.lastPushTime.Add(offset)
		preparedSql := ts.renderSqlInsert(labels, metric.value, newPushTime)
		ts.sqlWriter.Write(preparedSql)
		metric.lastPushTime = newerTime(metric.lastPushTime, newPushTime)
	}
}

func (ts *TimescaleExporter) Run(shutdownHandler *shutdown_handler.GracefulShutdownHandler, input <-chan *event.Slo) {
	go func() {
		defer ts.Close(context.Background())
		for {
			select {
			case newEvent, ok := <-input:
				if !ok {
					log.Info("input channel closed, finishing")
					shutdownHandler.Done()
					return
				}
				log.Debugf("processing newEvent %s", newEvent)
				ts.processEvent(newEvent)
				if newEvent.Occurred.Sub(ts.lastPushTime) > ts.config.UpdatedMetricPushInterval {
					ts.pushMetricsWithTimestamp(newEvent.Occurred)
					ts.lastPushTime = newerTime(ts.lastPushTime, newEvent.Occurred)
				}
			case <-ts.pushTicker.C:
				log.Debugf("full sync push")
				ts.pushAllMetricsWithOffset(ts.config.MaximumPushInterval)
				ts.lastPushTime = ts.lastPushTime.Add(ts.config.MaximumPushInterval)
			}
		}
	}()
}

func (ts *TimescaleExporter) initializeMetricsIfNotExist(metadata stringmap.StringMap) {
	key := metadata.String()
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

func (ts *TimescaleExporter) labelsFromEvent(sloEvent *event.Slo) stringmap.StringMap {
	return sloEvent.Metadata.Merge(stringmap.StringMap{
		ts.labelNames.Result:    string(sloEvent.Result),
		ts.labelNames.SloDomain: sloEvent.Domain,
		ts.labelNames.SloClass:  sloEvent.Class,
		ts.labelNames.SloApp:    sloEvent.App,
		ts.labelNames.EventKey:  sloEvent.Key,
		ts.labelNames.Instance:  ts.instanceName,
	})
}

func (ts *TimescaleExporter) processEvent(newEvent *event.Slo) {
	ts.statisticsMutex.Lock()
	defer ts.statisticsMutex.Unlock()
	labels := ts.labelsFromEvent(newEvent)

	for _, possibleResult := range event.PossibleResults {
		ts.initializeMetricsIfNotExist(labels.NewWith(ts.labelNames.Result, possibleResult.String()))
	}
	labels[ts.labelNames.Result] = newEvent.Result.String()
	counter := ts.statistics[labels.String()]
	counter.value++
	counter.lastEventTime = newEvent.Occurred
}
