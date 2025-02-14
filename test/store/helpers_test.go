//go:build integration
// +build integration

package test

import (
	"github.com/dodopizza/jaeger-kusto/config"
)

const (
	testOperation = "testOperation"
	testService   = "testService"
)

var (
	testPluginConfig = NewTestPluginConfig()
	logger           = config.NewLogger(testPluginConfig)
)

func NewTestPluginConfig() *config.PluginConfig {
	pc := config.NewDefaultPluginConfig()

	// override values for testing purpose
	pc.KustoConfigPath = "../../jaeger-kusto-config.json"
	pc.LogLevel = "debug"
	pc.WriterWorkersCount = 1

	return pc
}
