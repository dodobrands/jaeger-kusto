package store

import (
	"bytes"
	"context"
	"io"
	"sync"
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
	batchMaxBytes int
	batchTimeout  time.Duration
	ingest        kustoIngest
	logger        hclog.Logger
	spanInput     chan []string
	shutdown      chan bool
	shutdownWg    sync.WaitGroup
}

func newKustoSpanWriter(factory *kustoFactory, logger hclog.Logger) (*kustoSpanWriter, error) {
	in, err := factory.Ingest()
	if err != nil {
		return nil, err
	}

	writer := &kustoSpanWriter{
		batchMaxBytes: factory.PluginConfig.WriterBatchMaxBytes,
		batchTimeout:  time.Duration(factory.PluginConfig.WriterBatchTimeoutSeconds) * time.Second,
		ingest:        in,
		logger:        logger,
		spanInput:     make(chan []string, factory.PluginConfig.WriterSpanBufferSize),
		shutdown:      make(chan bool),
		shutdownWg:    sync.WaitGroup{},
	}

	go writer.ingestCSV()

	return writer, nil
}

func (kw *kustoSpanWriter) WriteSpan(_ context.Context, span *model.Span) error {
	spanStringArray, err := TransformSpanToStringArray(span)

	kw.spanInput <- spanStringArray
	return err
}

func (kw *kustoSpanWriter) Close() error {
	kw.logger.Debug("plugin shutdown started")

	kw.shutdownWg.Add(1)
	kw.shutdown <- true
	kw.shutdownWg.Wait()
	close(kw.spanInput)

	kw.logger.Debug("plugin shutdown completed")
	return nil
}

func (kw *kustoSpanWriter) ingestCSV() {
	ticker := time.NewTicker(kw.batchTimeout)

	b := &bytes.Buffer{}
	writer := altcsv.NewWriter(b)
	writer.AllQuotes = true

	for {
		select {
		case spans, ok := <-kw.spanInput:
			if !ok {
				return
			}
			batchSize := b.Len()
			if batchSize > kw.batchMaxBytes {
				kw.logger.Debug("Ingested batch by size", "batchSize", batchSize)
				kw.ingestBatch(b)
			}
			kw.logger.Debug("Append spans to batch buffer", "spanCount", len(spans))
			err := writer.Write(spans)
			if err != nil {
				kw.logger.Error("Failed to write csv", "error", err)
			}
			writer.Flush()
		case <-ticker.C:
			batchSize := b.Len()
			kw.ingestBatch(b)
			kw.logger.Debug("Ingested batch by time", "batchSize", batchSize)
		case <-kw.shutdown:
			batchSize := b.Len()
			kw.ingestBatch(b)
			kw.logger.Debug("Ingested batch by shutdown", "batchSize", batchSize)
			kw.shutdownWg.Done()
		}
	}
}

func (kw *kustoSpanWriter) ingestBatch(b *bytes.Buffer) {
	if b.Len() == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := kw.ingest.FromReader(ctx, b, ingest.FileFormat(ingest.CSV))
	if err == nil {
		b.Reset()
	} else {
		kw.logger.Error("Failed to ingest to Kusto", "error", err)
	}
}
