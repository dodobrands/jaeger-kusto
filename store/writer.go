package store

import (
	"context"
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
	"time"
)

type KustoSpanWriter struct {
	client *kusto.Client
	ingest *ingest.Ingestion
}

func NewKustoSpanWriter(client *kusto.Client, logger hclog.Logger) *KustoSpanWriter {

	in, err := ingest.New(client, "jaeger","Spans")
	if err != nil {
		panic("add error handling")
	}
	writer := &KustoSpanWriter{client, in}
	return writer
}

func (k KustoSpanWriter) WriteSpan(span *model.Span) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	spanReader, err := TransformSpanToCSV(span)
	if err != nil {
		return err
	}

	err = k.ingest.FromReader(ctx, spanReader, ingest.FileFormat(ingest.CSV))
	if err != nil {
		return err
	}

	return err
}

func getTagsValues(tags []model.KeyValue) []string {
	var values []string
	for i := range tags {
		values = append(values, tags[i].VStr)
	}
	return values
}


// TODO: make buffering for up to 4MB of spans