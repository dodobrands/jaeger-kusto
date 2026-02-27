package store

import (
	"context"

	"github.com/jaegertracing/jaeger/model"
)

// noopSpanWriter satisfies the spanstore.Writer interface but does nothing.
// Span ingestion into Kusto is handled externally by the OTEL exporter.
type noopSpanWriter struct{}

func (w *noopSpanWriter) WriteSpan(_ context.Context, _ *model.Span) error {
	return nil
}
