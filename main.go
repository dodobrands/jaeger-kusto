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

func flags(pc *store.PluginConfig) {
	flag.StringVar(&pc.KustoConfigPath,
		"config",
		pc.KustoConfigPath,
		"The path to the plugin's configuration file")
	flag.StringVar(&pc.LogLevel,
		"log-level",
		pc.LogLevel,
		"The threshold for plugin's logs. Anything below will be ignored")
	flag.BoolVar(&pc.LogJson,
		"log-json",
		pc.LogJson,
		"The control option determines will be logs in JSON format")
	flag.Float64Var(
		&pc.TracingSamplerPercentage,
		"tracing-sample-percentage",
		pc.TracingSamplerPercentage,
		"The percentage of grpc plugin traces. Value 0.0 disables tracing")
	flag.BoolVar(&pc.TracingRPCMetrics,
		"tracing-sample-rpc-metrics",
		pc.TracingRPCMetrics,
		"The control determines will be RPC metrics emitted")
	flag.IntVar(&pc.WriterSpanBufferSize,
		"writer-span-buffer-size",
		pc.WriterSpanBufferSize,
		"The size of in-memory buffer for new spans")
	flag.IntVar(&pc.WriterBatchMaxBytes,
		"writer-batch-max-bytes",
		pc.WriterBatchMaxBytes,
		"The size of ingest batch in bytes")
	flag.IntVar(&pc.WriterBatchTimeoutSeconds,
		"writer-batch-timeout-seconds",
		pc.WriterBatchTimeoutSeconds,
		"The timeout of ingest batch in seconds")
}

func main() {
	pluginConfig := store.NewPluginConfig()

	flags(pluginConfig)
	flag.Parse()

	logger := store.NewLogger(pluginConfig)
	kustoConfig, err := store.NewKustoConfig(pluginConfig, logger)
	if err != nil {
		logger.Error("error occurred while reading kusto configuration", "error", err)
		os.Exit(1)
	}

	kustoStore, err := store.NewStore(pluginConfig, kustoConfig, logger)
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
