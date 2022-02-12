package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/spanstore"
)

func NewTestPluginConfig() *PluginConfig {
	pc := NewDefaultPluginConfig()

	// override values for testing purpose
	pc.KustoConfigPath = ".././jaeger-kusto-config.json"
	pc.LogLevel = "debug"

	return pc
}

var (
	testPluginConfig = NewTestPluginConfig()
	logger           = NewLogger(testPluginConfig)
)

func TestKustoSpanReader_GetTrace(tester *testing.T) {
	kustoConfig, _ := ParseKustoConfig(testPluginConfig.KustoConfigPath)
	kustoStore, _ := NewStore(testPluginConfig, kustoConfig, logger)
	trace, _ := model.TraceIDFromString("0232d7f26e2317b1")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fulltrace, err := kustoStore.SpanReader().GetTrace(ctx, trace)
	if err != nil {
		logger.Error("can't get trace", err.Error())
	}
	fmt.Printf("%+v\n", fulltrace)
}

func TestKustoSpanReader_GetServices(t *testing.T) {
	kustoConfig, _ := ParseKustoConfig(testPluginConfig.KustoConfigPath)
	kustoStore, _ := NewStore(testPluginConfig, kustoConfig, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	services, err := kustoStore.SpanReader().GetServices(ctx)
	if err != nil {
		logger.Error("can't get services", err.Error())
	}
	fmt.Printf("%+v\n", services)
}

func TestKustoSpanReader_GetOperations(t *testing.T) {
	kustoConfig, _ := ParseKustoConfig(testPluginConfig.KustoConfigPath)
	kustoStore, _ := NewStore(testPluginConfig, kustoConfig, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	operations, err := kustoStore.SpanReader().GetOperations(ctx, spanstore.OperationQueryParameters{
		ServiceName: "frontend",
		SpanKind:    "",
	})
	if err != nil {
		logger.Error("can't get operations", err.Error())
	}
	fmt.Printf("%+v\n", operations)
}

func TestFindTraces(tester *testing.T) {
	query := spanstore.TraceQueryParameters{
		ServiceName:   "frontend",
		OperationName: "",
		StartTimeMin:  time.Date(2020, time.June, 10, 13, 0, 0, 0, time.UTC),
		StartTimeMax:  time.Date(2020, time.June, 10, 14, 0, 0, 0, time.UTC),
		NumTraces:     20,
		Tags: map[string]string{
			"http_method": "GET",
		},
	}

	kustoConfig, _ := ParseKustoConfig(testPluginConfig.KustoConfigPath)
	kustoStore, _ := NewStore(testPluginConfig, kustoConfig, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	traces, err := kustoStore.SpanReader().FindTraces(ctx, &query)
	if err != nil {
		logger.Error("can't find traces", err.Error())
	}
	fmt.Printf("%+v\n", traces)
}

func TestStore_DependencyReader(t *testing.T) {
	kustoConfig, _ := ParseKustoConfig(testPluginConfig.KustoConfigPath)
	kustoStore, _ := NewStore(testPluginConfig, kustoConfig, logger)
	dependencyLinks, err := kustoStore.DependencyReader().GetDependencies(context.Background(), time.Now(), 168*time.Hour)
	if err != nil {
		logger.Error("can't find dependencyLinks", err.Error())
	}
	fmt.Printf("%+v\n", dependencyLinks)
}
