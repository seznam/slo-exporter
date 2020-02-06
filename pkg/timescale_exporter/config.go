package timescale_exporter

import (
	"fmt"
	"time"
)

type TimescaleConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DbName   string

	DbInitTimeout        time.Duration
	DbInitCheckInterval  time.Duration
	DbBatchWriteSize     int
	DbWriteInterval      time.Duration
	DbWriteRetryInterval time.Duration
	DbWriteRetryLimit    int

	UpdatedMetricPushInterval time.Duration
	MaximumPushInterval       time.Duration
}

func (tc *TimescaleConfig) psqlInfo() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		tc.Host, tc.Port, tc.User, tc.Password, tc.DbName)
}

func (tc *TimescaleConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var YamlConfig struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		DbName   string `yaml:"dbname"`

		DbInitTimeout        string `yaml:"db_init_timeout"`
		DbInitCheckInterval  string `yaml:"db_init_check_interval"`
		DbBatchWriteSize     int    `yaml:"db_batch_write_size"`
		DbWriteInterval      string `yaml:"db_write_interval"`
		DbWriteRetryInterval string `yaml:"db_write_retry_interval"`
		DbWriteRetryLimit    int `yaml:"db_write_retry_limit"`

		UpdatedMetricPushInterval string `yaml:"updated_metric_push_interval"`
		MaximumPushInterval       string `yaml:"maximum_push_interval"`
	}
	if err := unmarshal(&YamlConfig); err != nil {
		return err
	}
	var err error
	tc.MaximumPushInterval, err = time.ParseDuration(YamlConfig.MaximumPushInterval)
	if err != nil {
		return fmt.Errorf("cannot parse maximum_push_interval: %w", err)
	}
	tc.UpdatedMetricPushInterval, err = time.ParseDuration(YamlConfig.UpdatedMetricPushInterval)
	if err != nil {
		return fmt.Errorf("cannot parse updated_metric_push_interval: %w", err)
	}
	tc.DbInitCheckInterval, err = time.ParseDuration(YamlConfig.DbInitCheckInterval)
	if err != nil {
		return fmt.Errorf("cannot parse db_init_check_interval: %w", err)
	}
	tc.DbInitTimeout, err = time.ParseDuration(YamlConfig.DbInitTimeout)
	if err != nil {
		return fmt.Errorf("cannot parse db_init_timeout: %w", err)
	}
	tc.DbWriteInterval, err = time.ParseDuration(YamlConfig.DbWriteInterval)
	if err != nil {
		return fmt.Errorf("cannot parse db_write_interval: %w", err)
	}
	tc.DbWriteRetryInterval, err = time.ParseDuration(YamlConfig.DbWriteRetryInterval)
	if err != nil {
		return fmt.Errorf("cannot parse db_write_retry_interval: %w", err)
	}
	tc.User = YamlConfig.User
	tc.Password = YamlConfig.Password
	tc.Port = YamlConfig.Port
	tc.DbName = YamlConfig.DbName
	tc.Host = YamlConfig.Host
	tc.DbBatchWriteSize = YamlConfig.DbBatchWriteSize
	tc.DbWriteRetryLimit = YamlConfig.DbWriteRetryLimit
	return nil
}
