package config

import (
	"github.com/hashicorp/go-hclog"
)

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
