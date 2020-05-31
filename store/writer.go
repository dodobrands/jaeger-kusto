package store

import (
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
)

type KustoSpanWriter struct {
	client *kusto.Client
}

func NewKustoSpanWriter(client *kusto.Client, logger hclog.Logger) *KustoSpanWriter {
	writer := &KustoSpanWriter{client}
	return writer
}

func (k KustoSpanWriter) WriteSpan(span *model.Span) error {
	panic("implement me")
}
