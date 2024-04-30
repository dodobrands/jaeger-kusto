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
	WriterBatchMaxBytes         int     `json:"writerBatchMaxBytes"`
	WriterBatchTimeoutSeconds   int     `json:"writerBatchTimeoutSeconds"`
	WriterSpanBufferSize        int     `json:"writerSpanBufferSize"`
	WriterWorkersCount          int     `json:"writerWorkersCount"`
	DisableJaegerUiTraces       bool    `json:"disableJaegerUiTraces"`
	ReadNoTruncation            bool    `json:"readNoTruncation"`
	ReadNoTimeout               bool    `json:"readNoTimeout"`
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
		TracingSamplerPercentage:    0.0,     // disabled by default
		TracingRPCMetrics:           false,   // disabled by default
		WriterBatchMaxBytes:         1048576, // 1 Mb by default
		WriterBatchTimeoutSeconds:   5,
		WriterSpanBufferSize:        100,
		WriterWorkersCount:          5,
		DisableJaegerUiTraces:       true, //disable UI logs of jaeger into OTELTraces. No traces from Jaeger UI will be sent
		ReadNoTruncation:            false,
		ReadNoTimeout:               false,
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
