package store

import (
	"errors"
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
func NewKustoConfig(pc *PluginConfig, logger hclog.Logger) (*KustoConfig, error) {
	v := viper.New()

	if pc.KustoConfigPath != "" {
		logger.Debug("trying to read config file", "file", pc.KustoConfigPath)

		f, err := ioutil.ReadFile(pc.KustoConfigPath)
		if err != nil {
			return nil, err
		}

		logger.Debug("file content is", "content", string(f))
		logger.Debug("initializing Kusto storage")

		v.SetConfigFile(pc.KustoConfigPath)
		v.SetConfigType("json")
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	}
	v.AutomaticEnv()

	config := KustoConfig{
		ClientID:     v.GetString("clientId"),
		ClientSecret: v.GetString("clientSecret"),
		TenantID:     v.GetString("tenantId"),
		Endpoint:     v.GetString("endpoint"),
		Database:     v.GetString("database"),
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// NewLogger returns configured logger from global options
func NewLogger(pc *PluginConfig) hclog.Logger {
	level := hclog.LevelFromString(pc.LogLevel)
	if level == hclog.NoLevel {
		// log level used by default
		level = hclog.Warn
	}

	return hclog.New(
		&hclog.LoggerOptions{
			Level:      level,
			Name:       ServiceName,
			JSONFormat: pc.LogJson,
		},
	)
}

// Validate returns error if any of required fields missing
func (kc *KustoConfig) validate() error {
	if kc.Database == "" {
		return errors.New("missing database in kusto configuration")
	}
	if kc.Endpoint == "" {
		return errors.New("missing endpoint in kusto configuration")
	}
	if kc.ClientID == "" || kc.ClientSecret == "" || kc.TenantID == "" {
		return errors.New("missing client configuration (ClientId, ClientSecret, TenantId) for kusto")
	}
	return nil
}
