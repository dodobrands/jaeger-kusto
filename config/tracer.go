package config

import (
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go/config"
	"io"
	"os"
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

	agentHost, agentHostExists := os.LookupEnv("JAEGER_AGENT_HOST")
	agentPort, agentPortExists := os.LookupEnv("JAEGER_AGENT_PORT")

	if agentHostExists && agentPortExists {
		c.Reporter = &config.ReporterConfig{
			LogSpans:           true,
			LocalAgentHostPort: fmt.Sprintf("%s:%s", agentHost, agentPort),
		}
	}

	tracer, closer, err := c.NewTracer()
	if err != nil {
		return nil, nil, err
	}

	opentracing.SetGlobalTracer(tracer)

	return tracer, closer, nil
}
