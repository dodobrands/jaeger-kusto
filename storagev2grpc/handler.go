package storagev2grpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/dodopizza/jaeger-kusto/internal/proto/storage/v2"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/dependencystore"
	"github.com/jaegertracing/jaeger/storage/spanstore"
	translator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	"go.opentelemetry.io/collector/pdata/ptrace"
	otlptracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var (
	_ storagev2.TraceReaderServer      = (*Handler)(nil)
	_ storagev2.DependencyReaderServer = (*Handler)(nil)
)

// Handler exposes the Jaeger v2 storage read APIs on top of the existing Kusto readers.
type Handler struct {
	storagev2.UnimplementedTraceReaderServer
	storagev2.UnimplementedDependencyReaderServer

	traceReader      spanstore.Reader
	dependencyReader dependencystore.Reader
	logger           hclog.Logger
}

func NewHandler(
	traceReader spanstore.Reader,
	dependencyReader dependencystore.Reader,
	logger hclog.Logger,
) *Handler {
	return &Handler{
		traceReader:      traceReader,
		dependencyReader: dependencyReader,
		logger:           logger,
	}
}

func (h *Handler) Register(server grpc.ServiceRegistrar) {
	storagev2.RegisterTraceReaderServer(server, h)
	storagev2.RegisterDependencyReaderServer(server, h)
}

func (h *Handler) GetTraces(req *storagev2.GetTracesRequest, srv grpc.ServerStreamingServer[otlptracev1.TracesData]) error {
	h.logger.Debug("GetTraces request", "count", len(req.GetQuery()))

	for _, query := range req.GetQuery() {
		traceID, err := model.TraceIDFromBytes(query.GetTraceId())
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "invalid trace_id: %v", err)
		}

		trace, err := h.traceReader.GetTrace(srv.Context(), traceID)
		if err != nil {
			if errors.Is(err, spanstore.ErrTraceNotFound) {
				continue
			}
			return status.Error(codes.Internal, err.Error())
		}

		otelTrace, err := modelTraceToProto(trace)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		if otelTrace == nil {
			continue
		}
		if err := srv.Send(otelTrace); err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) GetServices(ctx context.Context, _ *storagev2.GetServicesRequest) (*storagev2.GetServicesResponse, error) {
	services, err := h.traceReader.GetServices(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &storagev2.GetServicesResponse{Services: services}, nil
}

func (h *Handler) GetOperations(
	ctx context.Context,
	req *storagev2.GetOperationsRequest,
) (*storagev2.GetOperationsResponse, error) {
	operations, err := h.traceReader.GetOperations(ctx, spanstore.OperationQueryParameters{
		ServiceName: req.GetService(),
		SpanKind:    req.GetSpanKind(),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &storagev2.GetOperationsResponse{
		Operations: make([]*storagev2.Operation, 0, len(operations)),
	}
	for _, operation := range operations {
		resp.Operations = append(resp.Operations, &storagev2.Operation{
			Name:     operation.Name,
			SpanKind: operation.SpanKind,
		})
	}

	return resp, nil
}

func (h *Handler) FindTraces(req *storagev2.FindTracesRequest, srv grpc.ServerStreamingServer[otlptracev1.TracesData]) error {
	query, err := toSpanstoreQuery(req.GetQuery())
	if err != nil {
		return err
	}
	h.logger.Debug("FindTraces request", "service", query.ServiceName, "operation", query.OperationName, "numTraces", query.NumTraces)

	traces, err := h.traceReader.FindTraces(srv.Context(), query)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, trace := range traces {
		otelTrace, err := modelTraceToProto(trace)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		if otelTrace == nil {
			continue
		}
		if err := srv.Send(otelTrace); err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) FindTraceIDs(ctx context.Context, req *storagev2.FindTracesRequest) (*storagev2.FindTraceIDsResponse, error) {
	query, err := toSpanstoreQuery(req.GetQuery())
	if err != nil {
		return nil, err
	}
	h.logger.Debug("FindTraceIDs request", "service", query.ServiceName, "operation", query.OperationName, "numTraces", query.NumTraces)

	traceIDs, err := h.traceReader.FindTraceIDs(ctx, query)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &storagev2.FindTraceIDsResponse{
		TraceIds: make([]*storagev2.FoundTraceID, 0, len(traceIDs)),
	}
	for _, traceID := range traceIDs {
		resp.TraceIds = append(resp.TraceIds, &storagev2.FoundTraceID{
			TraceId: traceIDToBytes(traceID),
		})
	}

	return resp, nil
}

func (h *Handler) GetDependencies(
	ctx context.Context,
	req *storagev2.GetDependenciesRequest,
) (*storagev2.GetDependenciesResponse, error) {
	if req.GetStartTime() == nil || req.GetEndTime() == nil {
		return nil, status.Error(codes.InvalidArgument, "start_time and end_time are required")
	}

	startTime := req.GetStartTime().AsTime()
	endTime := req.GetEndTime().AsTime()
	if endTime.Before(startTime) {
		return nil, status.Error(codes.InvalidArgument, "end_time must not be before start_time")
	}
	h.logger.Debug("GetDependencies request", "startTime", startTime, "endTime", endTime)

	dependencies, err := h.dependencyReader.GetDependencies(ctx, endTime, endTime.Sub(startTime))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &storagev2.GetDependenciesResponse{
		Dependencies: make([]*storagev2.Dependency, 0, len(dependencies)),
	}
	for _, dependency := range dependencies {
		resp.Dependencies = append(resp.Dependencies, &storagev2.Dependency{
			Parent:    dependency.Parent,
			Child:     dependency.Child,
			CallCount: dependency.CallCount,
			Source:    dependency.Source,
		})
	}

	return resp, nil
}

func toSpanstoreQuery(query *storagev2.TraceQueryParameters) (*spanstore.TraceQueryParameters, error) {
	if query == nil {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	params := &spanstore.TraceQueryParameters{
		ServiceName:   query.GetServiceName(),
		OperationName: query.GetOperationName(),
		Tags:          tagsFromKeyValues(query.GetAttributes()),
		NumTraces:     int(query.GetSearchDepth()),
	}
	if query.GetStartTimeMin() != nil {
		params.StartTimeMin = query.GetStartTimeMin().AsTime()
	}
	if query.GetStartTimeMax() != nil {
		params.StartTimeMax = query.GetStartTimeMax().AsTime()
	}
	if query.GetDurationMin() != nil {
		params.DurationMin = query.GetDurationMin().AsDuration()
	}
	if query.GetDurationMax() != nil {
		params.DurationMax = query.GetDurationMax().AsDuration()
	}

	if err := validateTraceQuery(params); err != nil {
		return nil, err
	}

	return params, nil
}

func validateTraceQuery(query *spanstore.TraceQueryParameters) error {
	if query.ServiceName == "" && len(query.Tags) > 0 {
		return status.Error(codes.InvalidArgument, "service_name must be set when attributes are present")
	}
	if query.StartTimeMin.IsZero() || query.StartTimeMax.IsZero() {
		return status.Error(codes.InvalidArgument, "start_time_min and start_time_max are required")
	}
	if query.StartTimeMax.Before(query.StartTimeMin) {
		return status.Error(codes.InvalidArgument, "start_time_max must not be before start_time_min")
	}
	if query.DurationMin != 0 && query.DurationMax != 0 && query.DurationMin > query.DurationMax {
		return status.Error(codes.InvalidArgument, "duration_min must not be greater than duration_max")
	}
	if query.NumTraces > 500 {
		query.NumTraces = 500
	}
	return nil
}

func tagsFromKeyValues(keyValues []*storagev2.KeyValue) map[string]string {
	if len(keyValues) == 0 {
		return nil
	}

	tags := make(map[string]string, len(keyValues))
	for _, keyValue := range keyValues {
		if keyValue == nil {
			continue
		}
		tags[keyValue.GetKey()] = anyValueToString(keyValue.GetValue())
	}
	return tags
}

func anyValueToString(value *storagev2.AnyValue) string {
	typed := anyValueToInterface(value)
	switch typed := typed.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(data)
	}
}

func anyValueToInterface(value *storagev2.AnyValue) any {
	if value == nil || value.Value == nil {
		return nil
	}

	switch typed := value.Value.(type) {
	case *storagev2.AnyValue_StringValue:
		return typed.StringValue
	case *storagev2.AnyValue_BoolValue:
		return typed.BoolValue
	case *storagev2.AnyValue_IntValue:
		return typed.IntValue
	case *storagev2.AnyValue_DoubleValue:
		return typed.DoubleValue
	case *storagev2.AnyValue_BytesValue:
		return base64.StdEncoding.EncodeToString(typed.BytesValue)
	case *storagev2.AnyValue_ArrayValue:
		values := make([]any, 0, len(typed.ArrayValue.GetValues()))
		for _, item := range typed.ArrayValue.GetValues() {
			values = append(values, anyValueToInterface(item))
		}
		return values
	case *storagev2.AnyValue_KvlistValue:
		values := make(map[string]any, len(typed.KvlistValue.GetValues()))
		for _, item := range typed.KvlistValue.GetValues() {
			if item == nil {
				continue
			}
			values[item.GetKey()] = anyValueToInterface(item.GetValue())
		}
		return values
	default:
		return nil
	}
}

func modelTraceToProto(trace *model.Trace) (*otlptracev1.TracesData, error) {
	if trace == nil || len(trace.Spans) == 0 {
		return nil, nil
	}

	otelTrace, err := translator.ProtoToTraces([]*model.Batch{{Spans: trace.Spans}})
	if err != nil {
		return nil, err
	}

	bytes, err := new(ptrace.ProtoMarshaler).MarshalTraces(otelTrace)
	if err != nil {
		return nil, err
	}

	resp := &otlptracev1.TracesData{}
	if err := proto.Unmarshal(bytes, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func traceIDToBytes(traceID model.TraceID) []byte {
	bytes := make([]byte, 16)
	_, _ = traceID.MarshalTo(bytes)
	return bytes
}
