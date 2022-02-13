package runner

import (
	"github.com/dodopizza/jaeger-kusto/config"
	otGRPC "github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/hashicorp/go-plugin"
	storageGRPC "github.com/jaegertracing/jaeger/plugin/storage/grpc"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
	googleGRPC "google.golang.org/grpc"
)

func ServePlugin(store shared.StoragePlugin, tracer *config.PluginTracer) {
	pluginServices := shared.PluginServices{
		Store: store,
	}

	storageGRPC.ServeWithGRPCServer(&pluginServices, func(options []googleGRPC.ServerOption) *googleGRPC.Server {
		so := []googleGRPC.ServerOption{
			googleGRPC.UnaryInterceptor(otGRPC.OpenTracingServerInterceptor(tracer.Tracer())),
			googleGRPC.StreamInterceptor(otGRPC.OpenTracingStreamServerInterceptor(tracer.Tracer())),
		}
		return plugin.DefaultGRPCServer(so)
	})
}
