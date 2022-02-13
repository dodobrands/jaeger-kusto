package runner

import (
	"errors"
	"github.com/dodopizza/jaeger-kusto/config"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
)

func ServeServer(store shared.StoragePlugin, tracer *config.PluginTracer) {
	panic(errors.New("not implemented"))
}
