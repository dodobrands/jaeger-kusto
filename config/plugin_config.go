package config

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
	ServeServer               bool    `json:"serveServer"`
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
		ProfilingEnabled:          false,
		ProfilingPort:             6060,
		ServeServer:               false,
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
