package store

import (
	"io/ioutil"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/viper"
)

const (
	ServiceName = "jaeger-kusto"
)

// PluginConfig contains global options
type PluginConfig struct {
	KustoConfigPath string
	LogLevel        string
	LogJson         bool
}

// KustoConfig contains AzureAD service principal and Kusto cluster configs
type KustoConfig struct {
	ClientID     string
	ClientSecret string
	TenantID     string
	Endpoint     string
	Database     string
}

// NewKustoConfig reads config from plugin settings
func NewKustoConfig(pc PluginConfig, logger hclog.Logger) *KustoConfig {

	var kustoConfig *KustoConfig

	v := viper.New()

	if pc.KustoConfigPath != "" {
		logger.Debug("trying to read config file")
		logger.Debug("configPath is " + pc.KustoConfigPath)

		f, err := ioutil.ReadFile(pc.KustoConfigPath)

		if err != nil { // Handle errors reading the config file
			logger.Error("error reading config file", err.Error())
		}
		logger.Debug("file contents:" + string(f))

		logger.Debug("initializing Kusto storage")

		v.SetConfigFile(pc.KustoConfigPath)
		v.SetConfigType("json")
		err = v.ReadInConfig() // Find and read the config file
		if err != nil {        // Handle errors reading the config file
			logger.Error("error reading config file", err.Error())
		}
	}
	v.AutomaticEnv()

	kustoConfig = &KustoConfig{
		ClientID:     v.GetString("clientId"),
		ClientSecret: v.GetString("clientSecret"),
		TenantID:     v.GetString("tenantId"),
		Endpoint:     v.GetString("endpoint"),
		Database:     v.GetString("database"),
	}

	return kustoConfig
}

func NewLogger(pc PluginConfig) hclog.Logger {
	// log level used by default
	logLevel := hclog.Warn

	switch pc.LogLevel {
	case "warn":
		logLevel = hclog.Warn
	case "info":
		logLevel = hclog.Info
	case "debug":
		logLevel = hclog.Debug
	case "off":
		logLevel = hclog.Off
	}

	return hclog.New(
		&hclog.LoggerOptions{
			Level:      logLevel,
			Name:       ServiceName,
			JSONFormat: pc.LogJson,
		},
	)
}
