package store

import (
	"context"
	"errors"
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/data/table"
	"github.com/Azure/azure-kusto-go/kusto/data/types"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/spanstore"
	"time"
)

type KustoSpanReader struct {
	client *kusto.Client
}

func NewKustoSpanReader(client *kusto.Client, logger hclog.Logger) *KustoSpanReader {
	reader := &KustoSpanReader{client}
	return reader
}

const JaegerDatabase = "jaeger"
const defaultNumTraces = 20

func (r *KustoSpanReader) GetTrace(ctx context.Context, traceID model.TraceID) (*model.Trace, error) {

	kustoStmt := kusto.NewStmt("Spans | where TraceID == ParamTraceID").MustDefinitions(
		kusto.NewDefinitions().Must(
			kusto.ParamTypes{
				"ParamTraceID": kusto.ParamType{Type: types.String},
			},
		),).MustParameters(kusto.NewParameters().Must(kusto.QueryValues{"ParamTraceID": traceID.String()}))

	iter, err := r.client.Query(ctx, JaegerDatabase, kustoStmt)
	if err != nil {
		return nil, err
	}
	defer iter.Stop()

	var spans []*model.Span
	err = iter.Do(
		func(row *table.Row) error {
			rec := KustoSpan{}
			if err := row.ToStruct(&rec); err != nil {
				return err
			}
			var span *model.Span
			span, err = TransformKustoSpanToSpan(&rec)
			if err != nil {
				return err
			}
			spans = append(spans, span)
			return nil
		},
	)

	trace := model.Trace{Spans: spans}

	return &trace, err
}

func (r *KustoSpanReader) GetServices(ctx context.Context) ([]string, error) {
	iter, err := r.client.Query(ctx, JaegerDatabase, kusto.NewStmt("Spans | summarize count() by ProcessServiceName | sort by count_ | project ProcessServiceName"))
	if err != nil {
		return nil, err
	}
	defer iter.Stop()

	type Service struct {
		ServiceName string `kusto:"ProcessServiceName"`
	}

	var services []string
	err = iter.Do(
		func(row *table.Row) error {
			service := Service{}
			if err := row.ToStruct(&service); err != nil {
				return err
			}
			services = append(services, service.ServiceName)
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return services, err
}

func (r *KustoSpanReader) GetOperations(ctx context.Context, query spanstore.OperationQueryParameters) ([]spanstore.Operation, error) {

	type Operation struct {
		OperationName string `kusto:"OperationName"`
		SpanKind string `kusto:"SpanKind"`
	}

	var kustoStmt kusto.Stmt
	if query.ServiceName == "" && query.SpanKind == "" {
		kustoStmt = kusto.NewStmt(`Spans
| summarize count() by OperationName, SpanKind=tostring(Tags.span_kind)
| sort by count_
| project-away count_`)
	}

	if query.ServiceName != "" && query.SpanKind == "" {
		kustoStmt = kusto.NewStmt(`Spans
| where ProcessServiceName == ParamProcessServiceName
| summarize count() by OperationName, SpanKind=tostring(Tags.span_kind)
| sort by count_
| project-away count_`).MustDefinitions(
			kusto.NewDefinitions().Must(
				kusto.ParamTypes{
					"ParamProcessServiceName": kusto.ParamType{Type: types.String},
				},
			),).MustParameters(kusto.NewParameters().Must(kusto.QueryValues{"ParamProcessServiceName": query.ServiceName}))
	}

	iter, err := r.client.Query(ctx, JaegerDatabase, kustoStmt)
	if err != nil {
		return nil, err
	}
	defer iter.Stop()

	operations := []spanstore.Operation{}
	err = iter.Do(
		func(row *table.Row) error {
			operation := Operation{}
			if err := row.ToStruct(&operation); err != nil {
				return err
			}
			operations = append(operations, spanstore.Operation{
				Name:     operation.OperationName,
				SpanKind: operation.SpanKind,
			})
			return nil
		},
	)

	if err != nil {
		return nil, err
	}

	return operations, err
}


func (r *KustoSpanReader) FindTraceIDs(ctx context.Context, query *spanstore.TraceQueryParameters) ([]model.TraceID, error) {

	if err := validateQuery(query); err != nil {
		return nil, err
	}

	type TraceID struct {
		TraceID string `kusto:"TraceID"`
	}

	kustoStmt := kusto.NewStmt("Spans")
	kustoDefinitions := make(kusto.ParamTypes)
	kustoParameters := make(kusto.QueryValues)

	if query.ServiceName != ""  {
		kustoStmt = kustoStmt.Add(` | where ProcessServiceName == ParamProcessServiceName`)
		kustoDefinitions["ParamProcessServiceName"] = kusto.ParamType{Type: types.String}
		kustoParameters["ParamProcessServiceName"] = query.ServiceName
	}

	if query.OperationName != ""  {
		kustoStmt = kustoStmt.Add(` | where OperationName == ParamOperationName`)
		kustoDefinitions["ParamOperationName"] = kusto.ParamType{Type: types.String}
		kustoParameters["ParamOperationName"] = query.OperationName
	}

	if query.Tags != nil  {
		//TODO: not implemented
	}

	// StartTimeMin
	kustoStmt = kustoStmt.Add(` | where StartTime > ParamStartTimeMin`)
	kustoDefinitions["ParamStartTimeMin"] = kusto.ParamType{Type: types.DateTime}
	kustoParameters["ParamStartTimeMin"] = query.StartTimeMin

	// StartTimeMax
	kustoStmt = kustoStmt.Add(` | where StartTime < ParamStartTimeMax`)
	kustoDefinitions["ParamStartTimeMax"] = kusto.ParamType{Type: types.DateTime}
	kustoParameters["ParamStartTimeMax"] = query.StartTimeMax

	if query.DurationMin != 0  {
		kustoStmt = kustoStmt.Add(` | where Duration > ParamDurationMin`)
		kustoDefinitions["ParamDurationMin"] = kusto.ParamType{Type: types.Timespan}
		kustoParameters["ParamDurationMin"] = query.DurationMin
	}

	if query.DurationMax != 0  {
		kustoStmt = kustoStmt.Add(` | where Duration > ParamDurationMax`)
		kustoDefinitions["ParamDurationMax"] = kusto.ParamType{Type: types.Timespan}
		kustoParameters["ParamDurationMax"] = query.DurationMax
	}

	// Last aggregation
	kustoStmt = kustoStmt.Add("| summarize by TraceID")

	if query.NumTraces != 0  {
		kustoStmt.Add(`| sample ParamNumTraces`)
		kustoDefinitions["ParamNumTraces"] = kusto.ParamType{Type: types.Int}
		kustoParameters["ParamNumTraces"] = int32(query.NumTraces)
	}

	kustoStmt = kustoStmt.MustDefinitions(kusto.NewDefinitions().Must(kustoDefinitions)).MustParameters(kusto.NewParameters().Must(kustoParameters))

	iter, err := r.client.Query(ctx, JaegerDatabase, kustoStmt)
	if err != nil {
		return nil, err
	}
	defer iter.Stop()

	var traceIds []model.TraceID
	err = iter.Do(
		func(row *table.Row) error {
			rec := TraceID{}
			if err := row.ToStruct(&rec); err != nil {
				return err
			}
			traceId, err := model.TraceIDFromString(rec.TraceID)
			traceIds = append(traceIds, traceId)
			return err
		},
	)
	if err != nil {
		return nil, err
	}

	return traceIds, err
}

func (r *KustoSpanReader) FindTraces(ctx context.Context, query *spanstore.TraceQueryParameters) ([]*model.Trace, error) {
	if err := validateQuery(query); err != nil {
		return nil, err
	}

	if query.NumTraces == 0 {
		query.NumTraces = defaultNumTraces
	}

	kustoStmt := kusto.NewStmt("set truncationmaxsize=268435456; let TraceIDs = (Spans")
	kustoDefinitions := make(kusto.ParamTypes)
	kustoParameters := make(kusto.QueryValues)

	if query.ServiceName != ""  {
		kustoStmt = kustoStmt.Add(` | where ProcessServiceName == ParamProcessServiceName`)
		kustoDefinitions["ParamProcessServiceName"] = kusto.ParamType{Type: types.String}
		kustoParameters["ParamProcessServiceName"] = query.ServiceName
	}

	if query.OperationName != ""  {
		kustoStmt = kustoStmt.Add(` | where OperationName == ParamOperationName`)
		kustoDefinitions["ParamOperationName"] = kusto.ParamType{Type: types.String}
		kustoParameters["ParamOperationName"] = query.OperationName
	}

	if query.Tags != nil  {
		//TODO: not implemented
	}

	// StartTimeMin
	kustoStmt = kustoStmt.Add(` | where StartTime > ParamStartTimeMin`)
	kustoDefinitions["ParamStartTimeMin"] = kusto.ParamType{Type: types.DateTime}
	kustoParameters["ParamStartTimeMin"] = query.StartTimeMin

	// StartTimeMax
	kustoStmt = kustoStmt.Add(` | where StartTime < ParamStartTimeMax`)
	kustoDefinitions["ParamStartTimeMax"] = kusto.ParamType{Type: types.DateTime}
	kustoParameters["ParamStartTimeMax"] = query.StartTimeMax

	if query.DurationMin != 0  {
		kustoStmt = kustoStmt.Add(` | where Duration > ParamDurationMin`)
		kustoDefinitions["ParamDurationMin"] = kusto.ParamType{Type: types.Timespan}
		kustoParameters["ParamDurationMin"] = query.DurationMin
	}

	if query.DurationMax != 0  {
		kustoStmt = kustoStmt.Add(` | where Duration > ParamDurationMax`)
		kustoDefinitions["ParamDurationMax"] = kusto.ParamType{Type: types.Timespan}
		kustoParameters["ParamDurationMax"] = query.DurationMax
	}

	// Last aggregation
	kustoStmt = kustoStmt.Add(" | summarize by TraceID")

	kustoStmt = kustoStmt.Add(` | sample ParamNumTraces`)
	kustoDefinitions["ParamNumTraces"] = kusto.ParamType{Type: types.Int}
	kustoParameters["ParamNumTraces"] = int32(query.NumTraces)

	kustoStmt = kustoStmt.Add("); Spans")

	// StartTimeMin
	kustoStmt = kustoStmt.Add(` | where StartTime > ParamStartTimeMin`)
	kustoDefinitions["ParamStartTimeMin"] = kusto.ParamType{Type: types.DateTime}
	kustoParameters["ParamStartTimeMin"] = query.StartTimeMin

	// StartTimeMax
	kustoStmt = kustoStmt.Add(` | where StartTime < ParamStartTimeMax`)
	kustoDefinitions["ParamStartTimeMax"] = kusto.ParamType{Type: types.DateTime}
	kustoParameters["ParamStartTimeMax"] = query.StartTimeMax

	kustoStmt = kustoStmt.Add(` | where TraceID in (TraceIDs)`)

	kustoStmt = kustoStmt.MustDefinitions(kusto.NewDefinitions().Must(kustoDefinitions)).MustParameters(kusto.NewParameters().Must(kustoParameters))

	iter, err := r.client.Query(ctx, JaegerDatabase, kustoStmt)
	if err != nil {
		return nil, err
	}
	defer iter.Stop()

	m := make(map[model.TraceID][]*model.Span)

	err = iter.Do(
		func(row *table.Row) error {

			rec := KustoSpan{}
			if err := row.ToStruct(&rec); err != nil {
				return err
			}

			var span *model.Span
			span, err = TransformKustoSpanToSpan(&rec)
			if err != nil {
				return err
			}

			m[span.TraceID] = append(m[span.TraceID], span)
			return nil
		},
	)

	var traces []*model.Trace

	for _, spanArray := range m {
		trace := model.Trace{Spans: spanArray}
		traces = append(traces, &trace)
	}

	return traces, err
}

func (r *KustoSpanReader) GetDependencies(endTs time.Time, lookback time.Duration) ([]model.DependencyLink, error) {
	return nil, errors.New("not implemented")
}
