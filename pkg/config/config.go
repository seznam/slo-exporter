package config

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"time"
)

func New(logger *logrus.Entry) *Config {
	return &Config{logger: logger}
}

type Config struct {
	Pipeline                        []string
	LogLevel                        string
	WebServerListenAddress          string
	MaximumGracefulShutdownDuration time.Duration
	AfterPipelineShutdownDelay      time.Duration
	EventKeyMetadataKey             string
	Modules                         map[string]interface{}
	logger                          *logrus.Entry
}

func (c *Config) setupViper() {
	viper.SetConfigType("yaml")
	viper.SetEnvPrefix("slo_exporter")
	viper.AutomaticEnv()
}

func (c *Config) LoadFromFile(path string) error {
	c.setupViper()
	viper.SetDefault("LogLevel", "info")
	viper.SetDefault("WebServerListenAddress", "0.0.0.0:8080")
	viper.SetDefault("MaximumGracefulShutdownDuration", 20*time.Second)
	viper.SetDefault("AfterPipelineShutdownDelay", 0*time.Second)
	yamlFile, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open configuration file: %w", err)
	}
	if err := viper.ReadConfig(yamlFile); err != nil {
		return fmt.Errorf("failed to load configuration file: %w", err)
	}
	if err := viper.UnmarshalExact(c); err != nil {
		return fmt.Errorf("failed to unmarshall configuration file: %w", err)
	}
	return nil
}

func (c *Config) ModuleConfig(moduleName string) (*viper.Viper, error) {
	subConfig := viper.Sub("modules." + moduleName)
	if subConfig == nil {
		return nil, fmt.Errorf("missing configuration for module %s", moduleName)
	}
	subConfig.SetEnvPrefix("slo_exporter_" + moduleName)
	subConfig.AutomaticEnv()
	return subConfig, nil
}

// TODO FUSAKLA: remove once we have dynamic module loading
func (c *Config) MustModuleConfig(moduleName string) *viper.Viper {
	conf, err := c.ModuleConfig(moduleName)
	if err != nil {
		c.logger.Fatalf("failed to load %s configuration: %+v", moduleName, err)
	}
	return conf
}
