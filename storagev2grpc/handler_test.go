package storagev2grpc

import (
	"context"
	"testing"
	"time"

	storagev2 "github.com/dodopizza/jaeger-kusto/internal/proto/storage/v2"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/spanstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeSpanReader struct {
	getTraceFn      func(context.Context, model.TraceID) (*model.Trace, error)
	getServicesFn   func(context.Context) ([]string, error)
	getOperationsFn func(context.Context, spanstore.OperationQueryParameters) ([]spanstore.Operation, error)
	findTracesFn    func(context.Context, *spanstore.TraceQueryParameters) ([]*model.Trace, error)
	findTraceIDsFn  func(context.Context, *spanstore.TraceQueryParameters) ([]model.TraceID, error)
}

func (f *fakeSpanReader) GetTrace(ctx context.Context, traceID model.TraceID) (*model.Trace, error) {
	if f.getTraceFn != nil {
		return f.getTraceFn(ctx, traceID)
	}
	return nil, nil
}

func (f *fakeSpanReader) GetServices(ctx context.Context) ([]string, error) {
	if f.getServicesFn != nil {
		return f.getServicesFn(ctx)
	}
	return nil, nil
}

func (f *fakeSpanReader) GetOperations(ctx context.Context, query spanstore.OperationQueryParameters) ([]spanstore.Operation, error) {
	if f.getOperationsFn != nil {
		return f.getOperationsFn(ctx, query)
	}
	return nil, nil
}

func (f *fakeSpanReader) FindTraces(ctx context.Context, query *spanstore.TraceQueryParameters) ([]*model.Trace, error) {
	if f.findTracesFn != nil {
		return f.findTracesFn(ctx, query)
	}
	return nil, nil
}

func (f *fakeSpanReader) FindTraceIDs(ctx context.Context, query *spanstore.TraceQueryParameters) ([]model.TraceID, error) {
	if f.findTraceIDsFn != nil {
		return f.findTraceIDsFn(ctx, query)
	}
	return nil, nil
}

type fakeDependencyReader struct {
	getDependenciesFn func(context.Context, time.Time, time.Duration) ([]model.DependencyLink, error)
}

func (f *fakeDependencyReader) GetDependencies(
	ctx context.Context,
	endTs time.Time,
	lookback time.Duration,
) ([]model.DependencyLink, error) {
	if f.getDependenciesFn != nil {
		return f.getDependenciesFn(ctx, endTs, lookback)
	}
	return nil, nil
}

func TestToSpanstoreQuery_ConvertsAttributesAndCapsDepth(t *testing.T) {
	start := time.Date(2026, time.May, 20, 10, 0, 0, 0, time.UTC)
	end := start.Add(15 * time.Minute)

	query, err := toSpanstoreQuery(&storagev2.TraceQueryParameters{
		ServiceName:   "checkout",
		OperationName: "POST /api/orders",
		Attributes: []*storagev2.KeyValue{
			{
				Key: "error",
				Value: &storagev2.AnyValue{
					Value: &storagev2.AnyValue_BoolValue{BoolValue: true},
				},
			},
			{
				Key: "http.status_code",
				Value: &storagev2.AnyValue{
					Value: &storagev2.AnyValue_IntValue{IntValue: 500},
				},
			},
		},
		StartTimeMin: timestamppb.New(start),
		StartTimeMax: timestamppb.New(end),
		DurationMin:  durationpb.New(100 * time.Millisecond),
		DurationMax:  durationpb.New(2 * time.Second),
		SearchDepth:  999,
	})

	require.NoError(t, err)
	assert.Equal(t, "checkout", query.ServiceName)
	assert.Equal(t, "POST /api/orders", query.OperationName)
	assert.Equal(t, "true", query.Tags["error"])
	assert.Equal(t, "500", query.Tags["http.status_code"])
	assert.Equal(t, start, query.StartTimeMin)
	assert.Equal(t, end, query.StartTimeMax)
	assert.Equal(t, 100*time.Millisecond, query.DurationMin)
	assert.Equal(t, 2*time.Second, query.DurationMax)
	assert.Equal(t, 500, query.NumTraces)
}

func TestGetDependencies_UsesLookbackWindow(t *testing.T) {
	start := time.Date(2026, time.May, 20, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)

	var gotEnd time.Time
	var gotLookback time.Duration

	handler := NewHandler(
		&fakeSpanReader{},
		&fakeDependencyReader{
			getDependenciesFn: func(_ context.Context, endTs time.Time, lookback time.Duration) ([]model.DependencyLink, error) {
				gotEnd = endTs
				gotLookback = lookback
				return []model.DependencyLink{
					{Parent: "frontend", Child: "orders", CallCount: 7},
				}, nil
			},
		},
		hclog.NewNullLogger(),
	)

	resp, err := handler.GetDependencies(context.Background(), &storagev2.GetDependenciesRequest{
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	})

	require.NoError(t, err)
	assert.Equal(t, end, gotEnd)
	assert.Equal(t, 2*time.Hour, gotLookback)
	require.Len(t, resp.Dependencies, 1)
	assert.Equal(t, "frontend", resp.Dependencies[0].Parent)
	assert.Equal(t, "orders", resp.Dependencies[0].Child)
	assert.EqualValues(t, 7, resp.Dependencies[0].CallCount)
}

func TestFindTraceIDs_ReturnsFixedWidthTraceIDs(t *testing.T) {
	start := time.Date(2026, time.May, 20, 10, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Minute)

	handler := NewHandler(
		&fakeSpanReader{
			findTraceIDsFn: func(_ context.Context, query *spanstore.TraceQueryParameters) ([]model.TraceID, error) {
				assert.Equal(t, "checkout", query.ServiceName)
				return []model.TraceID{model.NewTraceID(0, 1)}, nil
			},
		},
		&fakeDependencyReader{},
		hclog.NewNullLogger(),
	)

	resp, err := handler.FindTraceIDs(context.Background(), &storagev2.FindTracesRequest{
		Query: &storagev2.TraceQueryParameters{
			ServiceName:  "checkout",
			StartTimeMin: timestamppb.New(start),
			StartTimeMax: timestamppb.New(end),
		},
	})

	require.NoError(t, err)
	require.Len(t, resp.TraceIds, 1)
	assert.Equal(t, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, resp.TraceIds[0].TraceId)
}

func TestModelTraceToProto_ConvertsJaegerTrace(t *testing.T) {
	trace := &model.Trace{
		Spans: []*model.Span{
			{
				TraceID:       model.NewTraceID(0, 1),
				SpanID:        model.NewSpanID(2),
				OperationName: "POST /checkout",
				StartTime:     time.Unix(0, 0).UTC(),
				Duration:      250 * time.Millisecond,
				Process: &model.Process{
					ServiceName: "frontend",
				},
				Tags:       []model.KeyValue{},
				Logs:       []model.Log{},
				References: []model.SpanRef{},
			},
		},
	}

	otelTrace, err := modelTraceToProto(trace)

	require.NoError(t, err)
	require.NotNil(t, otelTrace)
	require.Len(t, otelTrace.ResourceSpans, 1)
	require.Len(t, otelTrace.ResourceSpans[0].ScopeSpans, 1)
	require.Len(t, otelTrace.ResourceSpans[0].ScopeSpans[0].Spans, 1)
	assert.Equal(t, "POST /checkout", otelTrace.ResourceSpans[0].ScopeSpans[0].Spans[0].Name)
}
