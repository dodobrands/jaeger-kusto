package runner

import (
	"fmt"
	"github.com/dodopizza/jaeger-kusto/config"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
	"net"
)

func ServeServer(config *config.PluginConfig, store shared.StoragePlugin, tracer *config.PluginTracer) error {
	plugin := shared.StorageGRPCPlugin{
		Impl: store,
	}

	server := newGRPCServerWithTracer(tracer)

	if err := plugin.GRPCServer(nil, server); err != nil {
		return err
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", config.RemotePort))
	if err != nil {
		return err
	}

	if err := server.Serve(listener); err != nil {
		return err
	}

	return nil
}
