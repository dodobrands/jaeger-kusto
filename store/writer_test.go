package store

import (
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
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
		Duration: 34523*time.Millisecond,
		Tags: []model.KeyValue{model.KeyValue{
			Key:  "abc",
			VStr: "sdf",
		}},
	}

	span2 := &model.Span{
		TraceID:       model.NewTraceID(5355, 51),
		SpanID:        model.NewSpanID(3434343434),
		OperationName: testOperation,
		Process: &model.Process{
			ServiceName: testService,
		},
		StartTime: date,
		Duration: 34242*time.Millisecond,
		Tags: []model.KeyValue{model.KeyValue{
			Key:  "rty",
			VStr: "fgh",
		}},
	}

	span3 := &model.Span{
		TraceID:       model.NewTraceID(5555, 55),
		SpanID:        model.NewSpanID(567676755756767),
		OperationName: testOperation,
		Process: &model.Process{
			ServiceName: testService,
		},
		StartTime: date,
		Duration: 12121*time.Millisecond,
		Tags: []model.KeyValue{model.KeyValue{
			Key:  "qwe",
			VStr: "zxc",
		}},
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

	writer.WriteSpan(span)
	writer.WriteSpan(span2)
	writer.WriteSpan(span3)

}