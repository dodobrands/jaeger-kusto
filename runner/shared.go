package runner

import (
	"github.com/dodopizza/jaeger-kusto/config"
	ot "github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
)

func Serve(c *config.PluginConfig, store shared.StoragePlugin) error {
	if c.RemoteMode {
		return serveServer(c, store)
	}
	return servePlugin(c, store)
}

func newGRPCServerWithTracer(tracer opentracing.Tracer) *grpc.Server {
	return grpc.NewServer(
		grpc.UnaryInterceptor(ot.OpenTracingServerInterceptor(tracer)),
		grpc.StreamInterceptor(ot.OpenTracingStreamServerInterceptor(tracer)),
	)
}
