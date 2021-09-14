package store

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
	"github.com/tushar2708/altcsv"
)

type kustoIngest interface {
	FromReader(ctx context.Context, reader io.Reader, options ...ingest.FileOption) (*ingest.Result, error)
}

type kustoSpanWriter struct {
	ingest kustoIngest
	ch     chan []string
	logger hclog.Logger
}

func newKustoSpanWriter(client *kustoFactory, logger hclog.Logger, database string) *kustoSpanWriter {
	in, err := client.Ingest(database)
	if err != nil {
		logger.Error(fmt.Sprintf("%#v", err))
	}
	ch := make(chan []string, 100)
	writer := &kustoSpanWriter{in, ch, logger}

	go writer.ingestCSV(ch)

	return writer
}

func (k kustoSpanWriter) WriteSpan(ctx context.Context, span *model.Span) error {
	spanStringArray, err := TransformSpanToStringArray(span)

	k.ch <- spanStringArray
	return err
}

func (k kustoSpanWriter) ingestCSV(ch <-chan []string) {
	ticker := time.NewTicker(5 * time.Second)

	b := &bytes.Buffer{}
	writer := altcsv.NewWriter(b)
	writer.AllQuotes = true

	for {
		select {
		case buf, ok := <-ch:
			if !ok {
				return
			}
			if b.Len() > 1048576 {
				ingestBatch(k, b)
				k.logger.Debug("Ingested batch by size")
			}
			err := writer.Write(buf)
			if err != nil {
				k.logger.Error("Failed to write csv" + err.Error())
			}
			writer.Flush()
		case <-ticker.C:
			ingestBatch(k, b)
			k.logger.Debug("Ingested batch by time")
		}
	}
}

func ingestBatch(k kustoSpanWriter, b *bytes.Buffer) {
	if b.Len() == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := k.ingest.FromReader(ctx, b, ingest.FileFormat(ingest.CSV))
	if err == nil {
		b.Reset()
	} else {
		k.logger.Error("Failed to ingest to Kusto" + err.Error())
	}
}
