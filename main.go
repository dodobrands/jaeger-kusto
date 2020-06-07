package main

import (
	"flag"
	"fmt"
	"github.com/hashicorp/go-hclog"

	"github.com/dodopizza/jaeger-kusto/store"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc"
)

func main() {

	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Warn,
		Name:       "jaeger-kusto",
		JSONFormat: true,
	})

	logger.Info("Initializing Kusto storage")

	var configPath string

	flag.StringVar(&configPath, "config", "", "A path to the plugin's configuration file")
	flag.Parse()

	kustoConfig := store.InitConfig(configPath)

	logger.Warn(fmt.Sprintf("%#v", kustoConfig))

	kustoStore := store.NewStore(*kustoConfig, logger)
	grpc.Serve(kustoStore)
}
