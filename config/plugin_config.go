package config

const (
	ServiceName = "jaeger-kusto"
)

// PluginConfig contains global options
type PluginConfig struct {
	KustoConfigPath           string  `json:"kustoConfigPath"`
	LogLevel                  string  `json:"logLevel"`
	LogJson                   bool    `json:"logJson"`
	RemoteMode                bool    `json:"remoteMode"`
	RemotePort                int     `json:"remotePort"`
	ProfilingEnabled          bool    `json:"profilingEnabled"`
	ProfilingPort             int     `json:"profilingPort"`
	TracingSamplerPercentage  float64 `json:"tracingSamplerPercentage"`
	TracingRPCMetrics         bool    `json:"tracingRPCMetrics"`
	WriterSpanBufferSize      int     `json:"writerSpanBufferSize"`
	WriterBatchMaxBytes       int     `json:"writerBatchMaxBytes"`
	WriterBatchTimeoutSeconds int     `json:"writerBatchTimeoutSeconds"`
}

// NewDefaultPluginConfig returns default configuration options
func NewDefaultPluginConfig() *PluginConfig {
	return &PluginConfig{
		KustoConfigPath:           "",
		LogLevel:                  "warn",
		LogJson:                   false,
		RemoteMode:                false,
		RemotePort:                8989,
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
