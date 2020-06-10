package store

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
	"github.com/tushar2708/altcsv"
	"time"
)

type KustoSpanWriter struct {
	client *kusto.Client
	ingest *ingest.Ingestion
	ch chan []string
	logger hclog.Logger
}

func NewKustoSpanWriter(client *kusto.Client, logger hclog.Logger) *KustoSpanWriter {

	in, err := ingest.New(client, "jaeger","Spans")
	if err != nil {
		logger.Error(fmt.Sprintf("%#v", err))
	}
	ch := make(chan []string, 100)
	writer := &KustoSpanWriter{client, in, ch, logger}

	go writer.IngestCSV(ch)

	return writer
}

func (k KustoSpanWriter) WriteSpan(span *model.Span) error {

	spanStringArray, err := TransformSpanToCSV(span)

	k.ch <- spanStringArray
	return err
}

func (k KustoSpanWriter) IngestCSV (ch <-chan []string) {

	ticker := time.NewTicker(5*time.Second)

	b := &bytes.Buffer{}
	writer := altcsv.NewWriter(b)
	writer.AllQuotes = true

	for {
		select {
		case buf, ok := <-ch:
			if ok != true{
				return
			}
			if b.Len() > 1048576 {
				IngestBatch(k, b)
			}
			writer.Write(buf)
			writer.Flush()
		case <-ticker.C:
			IngestBatch(k, b)
		}
	}
	return
}

func IngestBatch(k KustoSpanWriter, b *bytes.Buffer) {
	if b.Len() == 0{
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := k.ingest.FromReader(ctx, b, ingest.FileFormat(ingest.CSV))
	if err == nil {
		b.Reset()
	} else {
		k.logger.Error(fmt.Sprintf("%#v", err))
	}
}

