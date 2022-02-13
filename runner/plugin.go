package runner

import (
	"github.com/dodopizza/jaeger-kusto/config"
	storageGRPC "github.com/jaegertracing/jaeger/plugin/storage/grpc"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
	googleGRPC "google.golang.org/grpc"
)

func ServePlugin(_ *config.PluginConfig, store shared.StoragePlugin, tracer *config.PluginTracer) error {
	pluginServices := shared.PluginServices{
		Store: store,
	}

	storageGRPC.ServeWithGRPCServer(&pluginServices, func(options []googleGRPC.ServerOption) *googleGRPC.Server {
		return newGRPCServerWithTracer(tracer)
	})

	return nil
}
