package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-kusto-go/kusto/data/value"

	"github.com/Azure/azure-kusto-go/kusto/unsafe"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/data/table"
	"github.com/Azure/azure-kusto-go/kusto/data/types"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/spanstore"
)

type kustoSpanReader struct {
	client   kustoReaderClient
	database string
}

type kustoReaderClient interface {
	Query(ctx context.Context, db string, query kusto.Stmt, options ...kusto.QueryOption) (*kusto.RowIterator, error)
}

func newKustoSpanReader(client *kustoFactory, database string) *kustoSpanReader {
	reader := &kustoSpanReader{client.Reader(), database}
	return reader
}

const defaultNumTraces = 20

var safetySwitch = unsafe.Stmt{
	Add:             true,
	SuppressWarning: true,
}

// GetTrace finds trace by TraceID
func (r *kustoSpanReader) GetTrace(ctx context.Context, traceID model.TraceID) (*model.Trace, error) {

	kustoStmt := kusto.NewStmt("Spans | where TraceID == ParamTraceID").MustDefinitions(
		kusto.NewDefinitions().Must(
			kusto.ParamTypes{
				"ParamTraceID": kusto.ParamType{Type: types.String},
			},
		)).MustParameters(kusto.NewParameters().Must(kusto.QueryValues{"ParamTraceID": traceID.String()}))

	iter, err := r.client.Query(ctx, r.database, kustoStmt)
	if err != nil {
		return nil, err
	}
	defer iter.Stop()

	var spans []*model.Span
	err = iter.Do(
		func(row *table.Row) error {
			rec := kustoSpan{}
			if err := row.ToStruct(&rec); err != nil {
				return err
			}
			var span *model.Span
			span, err = transformKustoSpanToModelSpan(&rec)
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

// GetServices finds all possible services that spanstore contains
func (r *kustoSpanReader) GetServices(ctx context.Context) ([]string, error) {
	iter, err := r.client.Query(ctx, r.database, kusto.NewStmt("set query_results_cache_max_age = time(5m); Spans | summarize by ProcessServiceName | sort by ProcessServiceName asc"))
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

// GetOperations finds all operations by provided Service and SpanKind
func (r *kustoSpanReader) GetOperations(ctx context.Context, query spanstore.OperationQueryParameters) ([]spanstore.Operation, error) {

	type Operation struct {
		OperationName string `kusto:"OperationName"`
		SpanKind      string `kusto:"SpanKind"`
	}

	var kustoStmt kusto.Stmt
	if query.ServiceName == "" && query.SpanKind == "" {
		kustoStmt = kusto.NewStmt(`Spans
| summarize count() by OperationName
| sort by count_
| project-away count_`)
	}

	if query.ServiceName != "" && query.SpanKind == "" {
		kustoStmt = kusto.NewStmt(`Spans
| where ProcessServiceName == ParamProcessServiceName
| summarize count() by OperationName
| sort by count_
| project-away count_`).MustDefinitions(
			kusto.NewDefinitions().Must(
				kusto.ParamTypes{
					"ParamProcessServiceName": kusto.ParamType{Type: types.String},
				},
			)).MustParameters(kusto.NewParameters().Must(kusto.QueryValues{"ParamProcessServiceName": query.ServiceName}))
	}

	iter, err := r.client.Query(ctx, r.database, kustoStmt)
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

// FindTraceIDs finds TraceIDs by provided query
func (r *kustoSpanReader) FindTraceIDs(ctx context.Context, query *spanstore.TraceQueryParameters) ([]model.TraceID, error) {

	if err := validateQuery(query); err != nil {
		return nil, err
	}

	type TraceID struct {
		TraceID string `kusto:"TraceID"`
	}

	kustoStmt := kusto.NewStmt("Spans", kusto.UnsafeStmt(safetySwitch))
	kustoDefinitions := make(kusto.ParamTypes)
	kustoParameters := make(kusto.QueryValues)

	if query.ServiceName != "" {
		kustoStmt = kustoStmt.Add(` | where ProcessServiceName == ParamProcessServiceName`)
		kustoDefinitions["ParamProcessServiceName"] = kusto.ParamType{Type: types.String}
		kustoParameters["ParamProcessServiceName"] = query.ServiceName
	}

	if query.OperationName != "" {
		kustoStmt = kustoStmt.Add(` | where OperationName == ParamOperationName`)
		kustoDefinitions["ParamOperationName"] = kusto.ParamType{Type: types.String}
		kustoParameters["ParamOperationName"] = query.OperationName
	}

	if query.Tags != nil {
		for k, v := range query.Tags {
			replacedTag := strings.Replace(k, ".", TagDotReplacementCharacter, -1)
			tagFilter := fmt.Sprintf(" | where Tags.%s == '%s' or ProcessTags.%s == '%s'", replacedTag, v, replacedTag, v)
			kustoStmt = kustoStmt.UnsafeAdd(tagFilter)
		}
	}

	kustoStmt = kustoStmt.Add(` | where StartTime > ParamStartTimeMin`)
	kustoDefinitions["ParamStartTimeMin"] = kusto.ParamType{Type: types.DateTime}
	kustoParameters["ParamStartTimeMin"] = query.StartTimeMin

	kustoStmt = kustoStmt.Add(` | where StartTime < ParamStartTimeMax`)
	kustoDefinitions["ParamStartTimeMax"] = kusto.ParamType{Type: types.DateTime}
	kustoParameters["ParamStartTimeMax"] = query.StartTimeMax

	if query.DurationMin != 0 {
		kustoStmt = kustoStmt.Add(` | where Duration > ParamDurationMin`)
		kustoDefinitions["ParamDurationMin"] = kusto.ParamType{Type: types.Timespan}
		kustoParameters["ParamDurationMin"] = query.DurationMin
	}

	if query.DurationMax != 0 {
		kustoStmt = kustoStmt.Add(` | where Duration > ParamDurationMax`)
		kustoDefinitions["ParamDurationMax"] = kusto.ParamType{Type: types.Timespan}
		kustoParameters["ParamDurationMax"] = query.DurationMax
	}

	kustoStmt = kustoStmt.Add("| summarize by TraceID")

	if query.NumTraces != 0 {
		kustoStmt.Add(`| sample ParamNumTraces`)
		kustoDefinitions["ParamNumTraces"] = kusto.ParamType{Type: types.Int}
		kustoParameters["ParamNumTraces"] = int32(query.NumTraces)
	}

	kustoStmt = kustoStmt.MustDefinitions(kusto.NewDefinitions().Must(kustoDefinitions)).MustParameters(kusto.NewParameters().Must(kustoParameters))

	iter, err := r.client.Query(ctx, r.database, kustoStmt)
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
			traceID, err := model.TraceIDFromString(rec.TraceID)
			traceIds = append(traceIds, traceID)
			return err
		},
	)
	if err != nil {
		return nil, err
	}

	return traceIds, err
}

// FindTraces finds and returns full traces with spans
func (r *kustoSpanReader) FindTraces(ctx context.Context, query *spanstore.TraceQueryParameters) ([]*model.Trace, error) {
	if err := validateQuery(query); err != nil {
		return nil, err
	}

	if query.NumTraces == 0 {
		query.NumTraces = defaultNumTraces
	}

	kustoStmt := kusto.NewStmt("let TraceIDs = (Spans", kusto.UnsafeStmt(safetySwitch))
	kustoDefinitions := make(kusto.ParamTypes)
	kustoParameters := make(kusto.QueryValues)

	if query.ServiceName != "" {
		kustoStmt = kustoStmt.Add(` | where ProcessServiceName == ParamProcessServiceName`)
		kustoDefinitions["ParamProcessServiceName"] = kusto.ParamType{Type: types.String}
		kustoParameters["ParamProcessServiceName"] = query.ServiceName
	}

	if query.OperationName != "" {
		kustoStmt = kustoStmt.Add(` | where OperationName == ParamOperationName`)
		kustoDefinitions["ParamOperationName"] = kusto.ParamType{Type: types.String}
		kustoParameters["ParamOperationName"] = query.OperationName
	}

	if query.Tags != nil {
		for k, v := range query.Tags {
			replacedTag := strings.Replace(k, ".", TagDotReplacementCharacter, -1)
			tagFilter := fmt.Sprintf(" | where Tags.%s == '%s' or ProcessTags.%s == '%s'", replacedTag, v, replacedTag, v)
			kustoStmt = kustoStmt.UnsafeAdd(tagFilter)
		}
	}

	kustoStmt = kustoStmt.Add(` | where StartTime > ParamStartTimeMin`)
	kustoDefinitions["ParamStartTimeMin"] = kusto.ParamType{Type: types.DateTime}
	kustoParameters["ParamStartTimeMin"] = query.StartTimeMin

	kustoStmt = kustoStmt.Add(` | where StartTime < ParamStartTimeMax`)
	kustoDefinitions["ParamStartTimeMax"] = kusto.ParamType{Type: types.DateTime}
	kustoParameters["ParamStartTimeMax"] = query.StartTimeMax

	if query.DurationMin != 0 {
		kustoStmt = kustoStmt.Add(` | where Duration > ParamDurationMin`)
		kustoDefinitions["ParamDurationMin"] = kusto.ParamType{Type: types.Timespan}
		kustoParameters["ParamDurationMin"] = query.DurationMin
	}

	if query.DurationMax != 0 {
		kustoStmt = kustoStmt.Add(` | where Duration > ParamDurationMax`)
		kustoDefinitions["ParamDurationMax"] = kusto.ParamType{Type: types.Timespan}
		kustoParameters["ParamDurationMax"] = query.DurationMax
	}

	kustoStmt = kustoStmt.Add(" | summarize by TraceID")

	kustoStmt = kustoStmt.Add(` | sample ParamNumTraces`)
	kustoDefinitions["ParamNumTraces"] = kusto.ParamType{Type: types.Int}
	kustoParameters["ParamNumTraces"] = int32(query.NumTraces)

	kustoStmt = kustoStmt.Add("); Spans")

	kustoStmt = kustoStmt.Add(` | where StartTime > ParamStartTimeMin`)
	kustoDefinitions["ParamStartTimeMin"] = kusto.ParamType{Type: types.DateTime}
	kustoParameters["ParamStartTimeMin"] = query.StartTimeMin

	kustoStmt = kustoStmt.Add(` | where StartTime < ParamStartTimeMax`)
	kustoDefinitions["ParamStartTimeMax"] = kusto.ParamType{Type: types.DateTime}
	kustoParameters["ParamStartTimeMax"] = query.StartTimeMax

	kustoStmt = kustoStmt.Add(` | where TraceID in (TraceIDs)`)

	kustoStmt = kustoStmt.MustDefinitions(kusto.NewDefinitions().Must(kustoDefinitions)).MustParameters(kusto.NewParameters().Must(kustoParameters))

	iter, err := r.client.Query(ctx, r.database, kustoStmt)
	if err != nil {
		return nil, err
	}
	defer iter.Stop()

	m := make(map[model.TraceID][]*model.Span)

	err = iter.Do(
		func(row *table.Row) error {

			rec := kustoSpan{}
			if err := row.ToStruct(&rec); err != nil {
				return err
			}

			var span *model.Span
			span, err = transformKustoSpanToModelSpan(&rec)
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

// GetDependencies returns DependencyLinks of services
func (r *kustoSpanReader) GetDependencies(endTs time.Time, lookback time.Duration) ([]model.DependencyLink, error) {

	type kustoDependencyLink struct {
		Parent    string     `kusto:"Parent"`
		Child     string     `kusto:"Child"`
		CallCount value.Long `kusto:"CallCount"`
	}

	kustoStmt := kusto.NewStmt(`Spans
| where StartTime < ParamEndTs and StartTime > (ParamEndTs-ParamLookBack)
| project ProcessServiceName, SpanID, ChildOfSpanId = tostring(References[0].spanID)
| join (Spans | project ChildOfSpanId=SpanID, ParentService=ProcessServiceName) on ChildOfSpanId
| where ProcessServiceName != ParentService
| extend Call=pack('Parent', ParentService, 'Child', ProcessServiceName)
| summarize CallCount=count() by tostring(Call)
| extend Call=parse_json(Call)
| evaluate bag_unpack(Call)`).MustDefinitions(
		kusto.NewDefinitions().Must(
			kusto.ParamTypes{
				"ParamEndTs":    kusto.ParamType{Type: types.DateTime},
				"ParamLookBack": kusto.ParamType{Type: types.Timespan},
			},
		)).MustParameters(kusto.NewParameters().Must(kusto.QueryValues{"ParamEndTs": endTs, "ParamLookBack": lookback}))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	iter, err := r.client.Query(ctx, r.database, kustoStmt)
	if err != nil {
		return nil, err
	}
	defer iter.Stop()

	var dependencyLinks []model.DependencyLink
	err = iter.Do(
		func(row *table.Row) error {
			rec := kustoDependencyLink{}
			if err := row.ToStruct(&rec); err != nil {
				return err
			}

			dependencyLinks = append(dependencyLinks, model.DependencyLink{
				Parent:    rec.Parent,
				Child:     rec.Child,
				CallCount: uint64(rec.CallCount.Value),
			})
			return nil
		},
	)

	return dependencyLinks, err

}
