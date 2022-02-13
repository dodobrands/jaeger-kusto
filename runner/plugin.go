package runner

import (
	"github.com/dodopizza/jaeger-kusto/config"
	storageGRPC "github.com/jaegertracing/jaeger/plugin/storage/grpc"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
	googleGRPC "google.golang.org/grpc"
)

func ServePlugin(c *config.PluginConfig, store shared.StoragePlugin) error {
	pluginServices := shared.PluginServices{
		Store: store,
	}

	tracer, closer, err := config.NewPluginTracer(c)
	if err != nil {
		return err
	}
	defer closer.Close()

	storageGRPC.ServeWithGRPCServer(&pluginServices, func(options []googleGRPC.ServerOption) *googleGRPC.Server {
		return newGRPCServerWithTracer(tracer)
	})

	return nil
}
