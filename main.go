package main

import (
	"flag"
	"os"

	"github.com/dodopizza/jaeger-kusto/store"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
)

func main() {
	pluginConfig := &store.PluginConfig{}

	flag.StringVar(&pluginConfig.KustoConfigPath, "config", "", "The path to the plugin's configuration file")
	flag.StringVar(&pluginConfig.LogLevel, "log-level", "", "The threshold for plugin's logs. Anything below will be ignored")
	flag.BoolVar(&pluginConfig.LogJson, "log-json", true, "The control determines will be logs in JSON format")
	flag.Parse()

	logger := store.NewLogger(pluginConfig)
	kustoConfig, err := store.NewKustoConfig(pluginConfig, logger)
	if err != nil {
		logger.Error("error occurred while reading kusto configuration", "error", err)
		os.Exit(1)
	}

	kustoStore, err := store.NewStore(kustoConfig, logger)
	if err != nil {
		logger.Error("error occurred while initializing kusto storage", "error", err)
		os.Exit(2)
	}

	pluginServices := shared.PluginServices{
		Store: kustoStore,
	}
	grpc.Serve(&pluginServices)
}
