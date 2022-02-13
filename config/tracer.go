package config

import (
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go/config"
	"io"
)

type PluginTracer struct {
	tracer opentracing.Tracer
	closer io.Closer
}

func NewPluginTracer(pc *PluginConfig) (*PluginTracer, error) {
	c := &config.Configuration{
		ServiceName: ServiceName,
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: pc.TracingSamplerPercentage,
		},
		RPCMetrics: pc.TracingRPCMetrics,
	}

	tracer, closer, err := c.NewTracer()
	if err != nil {
		return nil, err
	}

	return &PluginTracer{tracer, closer}, nil
}

func (pt *PluginTracer) EnableGlobalTracer() {
	opentracing.SetGlobalTracer(pt.tracer)
}

func (pt *PluginTracer) Close() error {
	return pt.closer.Close()
}

func (pt *PluginTracer) Tracer() opentracing.Tracer {
	return pt.tracer
}
