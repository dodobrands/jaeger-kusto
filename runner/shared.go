package runner

import (
	"github.com/dodopizza/jaeger-kusto/config"
	ot "github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
)

type Runner func(c *config.PluginConfig, store shared.StoragePlugin) error

func ResolveRunner(config *config.PluginConfig) Runner {
	if config.RemoteMode {
		return ServeServer
	}
	return ServePlugin
}

func newGRPCServerWithTracer(tracer opentracing.Tracer) *grpc.Server {
	return grpc.NewServer(
		grpc.UnaryInterceptor(ot.OpenTracingServerInterceptor(tracer)),
		grpc.StreamInterceptor(ot.OpenTracingStreamServerInterceptor(tracer)),
	)
}
