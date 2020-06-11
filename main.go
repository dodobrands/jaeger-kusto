package main

import (
	"flag"

	"github.com/dodopizza/jaeger-kusto/store"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc"
)

func main() {

	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Warn,
		Name:       "jaeger-kusto",
		JSONFormat: true,
	})

	var configPath string

	flag.StringVar(&configPath, "config", "", "A path to the plugin's configuration file")
	flag.Parse()

	kustoConfig := store.InitConfig(configPath, logger)

	kustoStore := store.NewStore(*kustoConfig, logger)
	grpc.Serve(kustoStore)
}
