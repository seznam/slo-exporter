package timescale_exporter

import (
	"fmt"
	"time"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DbName   string

	Instance string

	DbInitTimeout        time.Duration
	DbInitCheckInterval  time.Duration
	DbBatchWriteSize     int
	DbWriteInterval      time.Duration
	DbWriteRetryInterval time.Duration
	DbWriteRetryLimit    int

	UpdatedMetricPushInterval time.Duration
	MaximumPushInterval       time.Duration
}

func (tc *Config) psqlInfo() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		tc.Host, tc.Port, tc.User, tc.Password, tc.DbName)
}
