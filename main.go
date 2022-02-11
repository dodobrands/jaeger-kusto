package main

import (
	"flag"
	"github.com/hashicorp/go-plugin"
	"os"

	"github.com/dodopizza/jaeger-kusto/store"
	otGRPC "github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	storageGRPC "github.com/jaegertracing/jaeger/plugin/storage/grpc"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
	googleGRPC "google.golang.org/grpc"
)

func main() {
	pluginConfig := &store.PluginConfig{}

	flag.StringVar(&pluginConfig.KustoConfigPath, "config", "", "The path to the plugin's configuration file")
	flag.StringVar(&pluginConfig.LogLevel, "log-level", "", "The threshold for plugin's logs. Anything below will be ignored")
	flag.BoolVar(&pluginConfig.LogJson, "log-json", true, "The control determines will be logs in JSON format")
	flag.Float64Var(&pluginConfig.TracingSamplerPercentage, "tracing-sample-percentage", 0.0, "The percentage of grpc plugin traces")
	flag.BoolVar(&pluginConfig.TracingRPCMetrics, "tracing-sample-rpc-metrics", false, "The control determines will be RPC metrics emitted")
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

	pluginTracer, err := store.NewPluginTracer(pluginConfig)
	if err != nil {
		logger.Error("error occurred while initializing plugin tracer", "error", err)
		os.Exit(3)
	}
	pluginTracer.EnableGlobalTracer()
	defer pluginTracer.Close()

	pluginServices := shared.PluginServices{
		Store: kustoStore,
	}

	storageGRPC.ServeWithGRPCServer(&pluginServices, func(options []googleGRPC.ServerOption) *googleGRPC.Server {
		so := []googleGRPC.ServerOption{
			googleGRPC.UnaryInterceptor(otGRPC.OpenTracingServerInterceptor(pluginTracer.Tracer())),
			googleGRPC.StreamInterceptor(otGRPC.OpenTracingStreamServerInterceptor(pluginTracer.Tracer())),
		}
		return plugin.DefaultGRPCServer(so)
	})
}
