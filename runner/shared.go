package runner

import (
	"github.com/dodopizza/jaeger-kusto/config"
	kustostore "github.com/dodopizza/jaeger-kusto/store"
	ot "github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/hashicorp/go-hclog"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
)

func Serve(c *config.PluginConfig, store *kustostore.Store, logger hclog.Logger) error {
	if c.RemoteMode {
		return serveServer(c, store, logger)
	}
	return servePlugin(c, store, logger)
}

func newGRPCServerWithTracer(tracer opentracing.Tracer) *grpc.Server {
	return grpc.NewServer(
		grpc.UnaryInterceptor(ot.OpenTracingServerInterceptor(tracer)),
		grpc.StreamInterceptor(ot.OpenTracingStreamServerInterceptor(tracer)),
	)
}
