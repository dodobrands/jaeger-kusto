package config

import (
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go/config"
	"io"
)

func NewPluginTracer(pc *PluginConfig) (opentracing.Tracer, io.Closer, error) {
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
		return nil, nil, err
	}

	opentracing.SetGlobalTracer(tracer)

	return tracer, closer, nil
}
