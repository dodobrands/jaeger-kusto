package config

const (
	ServiceName             = "jaeger-kusto"
	PluginEnvironmentPrefix = "JAEGER_KUSTO_PLUGIN"
)

// PluginConfig contains global options
type PluginConfig struct {
	DiagnosticsProfilingEnabled bool    `json:"diagnosticsProfilingEnabled"`
	DiagnosticsListenAddress    string  `json:"diagnosticsListenAddress"`
	KustoConfigPath             string  `json:"kustoConfigPath"`
	LogLevel                    string  `json:"logLevel"`
	LogJson                     bool    `json:"logJson"`
	RemoteMode                  bool    `json:"remoteMode"`
	RemoteListenAddress         string  `json:"remoteListenAddress"`
	TracingSamplerPercentage    float64 `json:"tracingSamplerPercentage"`
	TracingRPCMetrics           bool    `json:"tracingRPCMetrics"`
	ReadNoTruncation            bool    `json:"readNoTruncation"`
	ReadNoTimeout               bool    `json:"readNoTimeout"`
	MetricsEnabled              bool    `json:"metricsEnabled"`
	MetricsListenAddress        string  `json:"metricsListenAddress"`
	CacheDiscoveryQueries       bool    `json:"cacheDiscoveryQueries"`
	CacheDiscoveryTTL           string  `json:"cacheDiscoveryTTL"`
}

// NewDefaultPluginConfig returns default configuration options
func NewDefaultPluginConfig() *PluginConfig {
	return &PluginConfig{
		DiagnosticsProfilingEnabled: false,
		DiagnosticsListenAddress:    ":6060",
		KustoConfigPath:             "",
		LogLevel:                    "warn",
		LogJson:                     false,
		RemoteMode:                  false,
		RemoteListenAddress:         "tcp://:8989",
		TracingSamplerPercentage:    0.0,   // disabled by default
		TracingRPCMetrics:           false, // disabled by default
		ReadNoTruncation:            false,
		ReadNoTimeout:               false,
		MetricsEnabled:              false,
		MetricsListenAddress:        ":9090",
		CacheDiscoveryQueries:       false,
		CacheDiscoveryTTL:           "6h",
	}
}

// ParseConfig reads file at path and returns instance of PluginConfig or error
func ParseConfig(path string) (*PluginConfig, error) {
	pc := NewDefaultPluginConfig()
	if err := load(path, pc); err != nil {
		return nil, err
	}

	if err := override(PluginEnvironmentPrefix, pc); err != nil {
		return nil, err
	}

	return pc, nil
}
