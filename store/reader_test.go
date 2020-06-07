package store

import (
	"context"
	"fmt"
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
	"testing"
	"time"
)




func TestKustoSpanReader_GetTrace(tester *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Debug,
		Name:       "jaeger-kusto-test",
		JSONFormat: true,
	})

	trace := model.NewTraceID(555, 5)

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