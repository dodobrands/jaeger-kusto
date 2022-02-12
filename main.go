package main

import (
	"flag"
	"fmt"
	"github.com/hashicorp/go-plugin"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/dodopizza/jaeger-kusto/store"
	otGRPC "github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	storageGRPC "github.com/jaegertracing/jaeger/plugin/storage/grpc"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
	googleGRPC "google.golang.org/grpc"
)

func main() {
	configPath := ""
	flag.StringVar(&configPath, "config", "", "The path to the plugin's configuration file")
	flag.Parse()

	pluginConfig, err := store.ParseConfig(configPath)
	if err != nil {
		os.Exit(1)
	}

	logger := store.NewLogger(pluginConfig)

	if pluginConfig.ProfilingEnabled {
		logger.Debug("starting profiling server at port", "port", pluginConfig.ProfilingPort)
		go func() {
			_ = http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", pluginConfig.ProfilingPort), nil)
		}()
	}

	kustoConfig, err := store.ParseKustoConfig(pluginConfig.KustoConfigPath)
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
