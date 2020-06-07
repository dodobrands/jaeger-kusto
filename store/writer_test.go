package store

import (
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const (
	testOperation = "testOperation"
	testService   = "testService"
)


func TestWriteSpan(tester *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Debug,
		Name:       "jaeger-kusto-test",
		JSONFormat: true,
	})

	date, _ := time.Parse(time.RFC3339, "1990-12-02T16:50:41+00:00")
	span := &model.Span{
		TraceID:       model.NewTraceID(555, 5),
		SpanID:        model.NewSpanID(5676767557),
		OperationName: testOperation,
		Process: &model.Process{
			ServiceName: testService,
		},
		StartTime: date,
		Duration: 34252523*time.Millisecond,
	}

	config := InitConfig("")

	authorizer := kusto.Authorization{
		Config: auth.NewClientCredentialsConfig(config.ClientID, config.ClientSecret, config.TenantID),
	}

	client, err := kusto.New(config.Endpoint, authorizer)
	if err != nil {
		panic("add error handling")
	}

	writer := NewKustoSpanWriter(client, logger)

	assert.NoError(tester, writer.WriteSpan(span))

}