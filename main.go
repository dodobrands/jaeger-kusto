package main

import (
	"flag"
	"github.com/dodopizza/jaeger-kusto/runner"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/dodopizza/jaeger-kusto/config"
	"github.com/dodopizza/jaeger-kusto/store"
)

func main() {
	configPath := ""
	flag.StringVar(&configPath, "config", "", "The path to the plugin's configuration file")
	flag.Parse()

	pluginConfig, err := config.ParseConfig(configPath)
	if err != nil {
		os.Exit(1)
	}

	logger := config.NewLogger(pluginConfig)
	logger.Info("plugin config", "config", pluginConfig)

	if pluginConfig.DiagnosticsProfilingEnabled {
		logger.Debug("starting profiling server at address", "address", pluginConfig.DiagnosticsListenAddress)
		go func() {
			_ = http.ListenAndServe(pluginConfig.DiagnosticsListenAddress, nil)
		}()
	}

	kustoConfig, err := config.ParseKustoConfig(pluginConfig.KustoConfigPath)
	if err != nil {
		logger.Error("error occurred while reading kusto configuration", "error", err)
		os.Exit(1)
	}

	kustoStore, err := store.NewStore(pluginConfig, kustoConfig, logger)
	if err != nil {
		logger.Error("error occurred while initializing kusto storage", "error", err)
		os.Exit(2)
	}

	if err := runner.Serve(pluginConfig, kustoStore, logger); err != nil {
		logger.Error("error occurred while invoking runner", "error", err)
		os.Exit(3)
	}
}
