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
	KustoConfigPath           string  `json:"kustoConfigPath"`
	LogLevel                  string  `json:"logLevel"`
	LogJson                   bool    `json:"logJson"`
	ProfilingEnabled          bool    `json:"profilingEnabled"`
	ProfilingPort             int     `json:"profilingPort"`
	TracingSamplerPercentage  float64 `json:"tracingSamplerPercentage"`
	TracingRPCMetrics         bool    `json:"tracingRPCMetrics"`
	WriterSpanBufferSize      int     `json:"writerSpanBufferSize"`
	WriterBatchMaxBytes       int     `json:"writerBatchMaxBytes"`
	WriterBatchTimeoutSeconds int     `json:"writerBatchTimeoutSeconds"`
}

// KustoConfig contains AzureAD service principal and Kusto cluster configs
type KustoConfig struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	TenantID     string `json:"tenantId"`
	Endpoint     string `json:"endpoint"`
	Database     string `json:"database"`
}

// NewDefaultPluginConfig returns default configuration options
func NewDefaultPluginConfig() *PluginConfig {
	return &PluginConfig{
		KustoConfigPath:           "",
		LogLevel:                  "warn",
		LogJson:                   false,
		ProfilingEnabled:          false,
		ProfilingPort:             6060,
		TracingSamplerPercentage:  0.0,   // disabled by default
		TracingRPCMetrics:         false, // disabled by default
		WriterSpanBufferSize:      100,
		WriterBatchMaxBytes:       1048576, // 1 Mb by default
		WriterBatchTimeoutSeconds: 5,
	}
}

// ParseConfig reads file at path and returns instance of PluginConfig or error
func ParseConfig(path string) (*PluginConfig, error) {
	pc := NewDefaultPluginConfig()
	if err := load(path, pc); err != nil {
		return nil, err
	}
	return pc, nil
}

// ParseKustoConfig reads file at path and returns instance of KustoConfig or error
func ParseKustoConfig(path string) (*KustoConfig, error) {
	c := &KustoConfig{}

	if err := load(path, c); err != nil {
		return nil, err
	}

	if err := c.validate(); err != nil {
		return nil, err
	}

	return c, nil
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

func load(path string, data interface{}) error {
	if path == "" {
		return errors.New("empty path to config")
	}

	_, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("json")

	if err := v.ReadInConfig(); err != nil {
		return err
	}

	return v.Unmarshal(data)
}
