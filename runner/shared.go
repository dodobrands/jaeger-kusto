package runner

import (
	"github.com/dodopizza/jaeger-kusto/config"
	ot "github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
	"google.golang.org/grpc"
)

type Runner func(config *config.PluginConfig, store shared.StoragePlugin, tracer *config.PluginTracer) error

func ResolveRunner(config *config.PluginConfig) Runner {
	if config.RemoteMode {
		return ServeServer
	}
	return ServePlugin
}

func newGRPCServerWithTracer(tracer *config.PluginTracer) *grpc.Server {
	t := tracer.Tracer()

	return grpc.NewServer(
		grpc.UnaryInterceptor(ot.OpenTracingServerInterceptor(t)),
		grpc.StreamInterceptor(ot.OpenTracingStreamServerInterceptor(t)),
	)
}
