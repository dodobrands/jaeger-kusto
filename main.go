package main

import (
	"flag"

	"github.com/dodopizza/jaeger-kusto/store"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
)

func main() {
	pluginConfig := store.PluginConfig{}

	flag.StringVar(&pluginConfig.KustoConfigPath, "config", "", "The path to the plugin's configuration file")
	flag.StringVar(&pluginConfig.LogLevel, "log-level", "", "The threshold for plugin's logs. Anything below will be ignored")
	flag.BoolVar(&pluginConfig.LogJson, "log-json", true, "The control determines will be logs in JSON format")
	flag.Parse()

	logger := store.NewLogger(pluginConfig)
	kustoConfig := store.NewKustoConfig(pluginConfig, logger)
	kustoStore := store.NewStore(*kustoConfig, logger)

	grpc.Serve(&shared.PluginServices{
		Store: kustoStore,
	})
}
