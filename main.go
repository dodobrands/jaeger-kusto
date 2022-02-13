package main

import (
	"flag"
	"fmt"
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
	logger.Debug("plugin config", "config", pluginConfig)

	if pluginConfig.ProfilingEnabled {
		logger.Debug("starting profiling server at port", "port", pluginConfig.ProfilingPort)
		go func() {
			_ = http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", pluginConfig.ProfilingPort), nil)
		}()
	}

	pluginTracer, err := config.NewPluginTracer(pluginConfig)
	if err != nil {
		logger.Error("error occurred while initializing plugin tracer", "error", err)
		os.Exit(1)
	}
	pluginTracer.EnableGlobalTracer()
	defer pluginTracer.Close()

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

	r := runner.ResolveRunner(pluginConfig)
	if err := r(pluginConfig, kustoStore, pluginTracer); err != nil {
		logger.Error("error occurred while starting serve", "error", err)
		os.Exit(3)
	}
}
