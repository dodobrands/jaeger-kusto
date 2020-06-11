package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/spanstore"
)

func TestKustoSpanReader_GetTrace(tester *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Debug,
		Name:       "jaeger-kusto-test",
		JSONFormat: true,
	})

	trace, err := model.TraceIDFromString("0232d7f26e2317b1")

	config := InitConfig("")

	authorizer := kusto.Authorization{
		Config: auth.NewClientCredentialsConfig(config.ClientID, config.ClientSecret, config.TenantID),
	}

	client, err := kusto.New(config.Endpoint, authorizer)
	if err != nil {
		panic("add error handling")
	}

	reader := NewKustoSpanReader(client, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fulltrace, err := reader.GetTrace(ctx, trace)
	fmt.Printf("%+v\n", fulltrace)
}

func TestKustoSpanReader_GetServices(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Debug,
		Name:       "jaeger-kusto-test",
		JSONFormat: true,
	})

	config := InitConfig("")

	authorizer := kusto.Authorization{
		Config: auth.NewClientCredentialsConfig(config.ClientID, config.ClientSecret, config.TenantID),
	}

	client, err := kusto.New(config.Endpoint, authorizer)
	if err != nil {
		panic("add error handling")
	}

	reader := NewKustoSpanReader(client, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	services, err := reader.GetServices(ctx)
	fmt.Printf("%+v\n", services)
}

func TestKustoSpanReader_GetOperations(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Debug,
		Name:       "jaeger-kusto-test",
		JSONFormat: true,
	})

	config := InitConfig("")

	authorizer := kusto.Authorization{
		Config: auth.NewClientCredentialsConfig(config.ClientID, config.ClientSecret, config.TenantID),
	}

	client, err := kusto.New(config.Endpoint, authorizer)
	if err != nil {
		panic("add error handling")
	}

	reader := NewKustoSpanReader(client, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	operations, err := reader.GetOperations(ctx, spanstore.OperationQueryParameters{
		ServiceName: "frontend",
		SpanKind:    "",
	})
	fmt.Printf("%+v\n", operations)
}

func TestFindTraces(tester *testing.T) {
	query := spanstore.TraceQueryParameters{
		ServiceName:   "frontend",
		OperationName: "",
		StartTimeMin:  time.Date(2020, time.June, 10, 13, 0, 0, 0, time.UTC),
		StartTimeMax:  time.Date(2020, time.June, 10, 14, 0, 0, 0, time.UTC),
		NumTraces:     20,
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Debug,
		Name:       "jaeger-kusto-test",
		JSONFormat: true,
	})

	config := InitConfig("")

	authorizer := kusto.Authorization{
		Config: auth.NewClientCredentialsConfig(config.ClientID, config.ClientSecret, config.TenantID),
	}

	client, err := kusto.New(config.Endpoint, authorizer)
	if err != nil {
		panic("add error handling")
	}

	reader := NewKustoSpanReader(client, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	traces, err := reader.FindTraces(ctx, &query)
	fmt.Printf("%+v\n", traces)

}
