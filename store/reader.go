package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"

	"github.com/Azure/azure-kusto-go/azkustodata"
	"github.com/Azure/azure-kusto-go/azkustodata/kql"
	kustoquery "github.com/Azure/azure-kusto-go/azkustodata/query"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/spanstore"
)

type kustoSpanReader struct {
	client             kustoReaderClient
	database           string
	tableName          string
	logger             hclog.Logger
	defaultReadOptions []azkustodata.QueryOption
	cache              *discoveryCache      // nil when caching is disabled
	depRefresher       *dependencyRefresher // nil when caching is disabled
}

type kustoReaderClient interface {
	Query(ctx context.Context, db string, query azkustodata.Statement, options ...azkustodata.QueryOption) (kustoquery.Dataset, error)
}


func newKustoSpanReader(factory *kustoFactory, logger hclog.Logger, defaultReadOptions []azkustodata.QueryOption, cache *discoveryCache) (*kustoSpanReader, error) {
	return &kustoSpanReader{
		client:             factory.Reader(),
		database:           factory.Database,
		tableName:          factory.Table,
		logger:             logger,
		defaultReadOptions: defaultReadOptions,
		cache:              cache,
	}, nil
}

const defaultNumTraces = 20

func GetClientId() string {
	// get a UUID and concatenante with the service name
	return fmt.Sprintf("azure-kusto-jaeger-%s", uuid.New().String())
}

// otelStatusCodeToKusto maps Jaeger-style otel.status_code values to the raw SpanStatus column values in Kusto.
var otelStatusCodeToKusto = map[string]string{
	"ERROR": "STATUS_CODE_ERROR",
	"OK":    "STATUS_CODE_OK",
	"UNSET": "STATUS_CODE_UNSET",
}

// spanKindToKusto maps Jaeger-style span.kind values to the raw SpanKind column values in Kusto.
var spanKindToKusto = map[string]string{
	"server":   "SPAN_KIND_SERVER",
	"client":   "SPAN_KIND_CLIENT",
	"consumer": "SPAN_KIND_CONSUMER",
	"producer": "SPAN_KIND_PRODUCER",
	"internal": "SPAN_KIND_INTERNAL",
}

// buildTagFilter returns a KQL filter clause for a tag key/value pair.
// Well-known synthetic tags (otel.status_code, error, span.kind) are mapped to native Kusto columns
// in addition to TraceAttributes/ResourceAttributes, since they may not be stored as span attributes.
func buildTagFilter(k, v string) string {
	switch k {
	case "otel.status_code":
		if kustoVal, ok := otelStatusCodeToKusto[v]; ok {
			return fmt.Sprintf(" | where SpanStatus == '%s' or TraceAttributes['%s'] == '%s' or ResourceAttributes['%s'] == '%s'", kustoVal, k, v, k, v)
		}
		return fmt.Sprintf(" | where TraceAttributes['%s'] == '%s' or ResourceAttributes['%s'] == '%s'", k, v, k, v)
	case "error":
		if v == "true" {
			return fmt.Sprintf(" | where SpanStatus == 'STATUS_CODE_ERROR' or TraceAttributes['%s'] == '%s' or ResourceAttributes['%s'] == '%s'", k, v, k, v)
		}
		return fmt.Sprintf(" | where TraceAttributes['%s'] == '%s' or ResourceAttributes['%s'] == '%s'", k, v, k, v)
	case "span.kind":
		if kustoVal, ok := spanKindToKusto[v]; ok {
			return fmt.Sprintf(" | where SpanKind == '%s' or TraceAttributes['%s'] == '%s' or ResourceAttributes['%s'] == '%s'", kustoVal, k, v, k, v)
		}
		return fmt.Sprintf(" | where TraceAttributes['%s'] == '%s' or ResourceAttributes['%s'] == '%s'", k, v, k, v)
	default:
		return fmt.Sprintf(" | where TraceAttributes['%s'] == '%s' or ResourceAttributes['%s'] == '%s'", k, v, k, v)
	}
}

// GetTrace finds trace by TraceID
func (r *kustoSpanReader) GetTrace(ctx context.Context, traceID model.TraceID) (*model.Trace, error) {
	kustoStmt := kql.New("").AddTable(r.tableName).AddLiteral(getTraceQuery)
	kustoStmtParams := kql.NewParameters().AddString("ParamTraceID", traceID.String())

	clientRequestId := GetClientId()
	// Append a client request id as well to the request
	dataset, err := r.client.Query(ctx, r.database, kustoStmt, append(r.defaultReadOptions,
		azkustodata.ClientRequestID(clientRequestId), azkustodata.QueryParameters(kustoStmtParams))...)
	if err != nil {
		r.logger.Error("Failed running GetTrace query. TraceID: %s. ClientRequestId : %s", traceID.String(), clientRequestId)
		return nil, err
	}

	var spans []*model.Span
	for _, row := range dataset.Tables()[0].Rows() {
		rec := kustoSpan{}
		if err := row.ToStruct(&rec); err != nil {
			return nil, err
		}
		var span *model.Span
		span, err = transformKustoSpanToModelSpan(&rec, r.logger)
		if err != nil {
			r.logger.Error(fmt.Sprintf("Error in transformKustoSpanToModelSpan. TraceId: %s SpanId: %s", rec.TraceID, rec.SpanID), err)
			return nil, err
		}
		spans = append(spans, span)
	}
	trace := model.Trace{Spans: spans}
	return &trace, err
}

// GetServices finds all possible services that spanstore contains
func (r *kustoSpanReader) GetServices(ctx context.Context) ([]string, error) {
	const cacheKey = "services"
	if r.cache != nil {
		if cached, ok := r.cache.get(cacheKey); ok {
			r.logger.Debug("GetServices: returning cached result")
			return cached.([]string), nil
		}
	}

	clientRequestId := GetClientId()
	kustoStmt := kql.New(queryResultsCacheAge).AddTable(r.tableName).AddLiteral(getServicesQuery)
	r.logger.Debug("GetServicesQuery : %s ", kustoStmt.String())
	dataset, err := r.client.Query(ctx, r.database, kustoStmt, append(r.defaultReadOptions, azkustodata.ClientRequestID(clientRequestId))...)

	if err != nil {
		r.logger.Error("Failed running GetServices query. ClientRequestId : %s", clientRequestId)
		return nil, err
	}

	type Service struct {
		ServiceName string `kusto:"ProcessServiceName"`
	}

	var services []string
	for _, row := range dataset.Tables()[0].Rows() {
		service := Service{}
		if err := row.ToStruct(&service); err != nil {
			return nil, err
		}
		services = append(services, service.ServiceName)
	}

	if r.cache != nil {
		r.cache.set(cacheKey, services)
		r.logger.Debug("GetServices: cached %d services", len(services))
	}

	return services, err
}

// GetOperations finds all operations by provided Service and SpanKind
func (r *kustoSpanReader) GetOperations(ctx context.Context, query spanstore.OperationQueryParameters) ([]spanstore.Operation, error) {
	cacheKey := fmt.Sprintf("operations:%s:%s", query.ServiceName, query.SpanKind)
	if r.cache != nil {
		if cached, ok := r.cache.get(cacheKey); ok {
			r.logger.Debug("GetOperations: returning cached result for %s", cacheKey)
			return cached.([]spanstore.Operation), nil
		}
	}

	type Operation struct {
		OperationName string `kusto:"OperationName"`
		SpanKind      string `kusto:"SpanKind"`
	}
	clientRequestId := GetClientId()
	var dataset kustoquery.Dataset
	var err error
	if query.ServiceName == "" && query.SpanKind == "" {
		kustoStmt := kql.New(queryResultsCacheAge).AddTable(r.tableName).AddLiteral(getOpsWithNoParamsQuery)
		dataset, err = r.client.Query(ctx, r.database, kustoStmt, append(r.defaultReadOptions, azkustodata.ClientRequestID(clientRequestId))...)
	}

	if query.ServiceName != "" && query.SpanKind == "" {
		kustoStmt := kql.New(queryResultsCacheAge).AddTable(r.tableName).AddLiteral(getOpsWithParamsQuery)
		kustoStmtParams := kql.NewParameters().AddString("ParamProcessServiceName", query.ServiceName)

		dataset, err = r.client.Query(ctx, r.database, kustoStmt, append(r.defaultReadOptions, azkustodata.ClientRequestID(clientRequestId), azkustodata.QueryParameters(kustoStmtParams))...)
	}

	if err != nil {
		r.logger.Error("Failed running GetOperations query. ClientRequestId : %s", clientRequestId)
		return nil, err
	}

	operations := []spanstore.Operation{}
	for _, row := range dataset.Tables()[0].Rows() {
		operation := Operation{}
		if err := row.ToStruct(&operation); err != nil {
			return nil, err
		}
		operations = append(operations, spanstore.Operation{
			Name:     operation.OperationName,
			SpanKind: operation.SpanKind,
		})
	}

	if err != nil {
		return nil, err
	}

	if r.cache != nil {
		r.cache.set(cacheKey, operations)
		r.logger.Debug("GetOperations: cached %d operations for %s", len(operations), cacheKey)
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

	kustoStmt := kql.New("").AddTable(r.tableName).AddLiteral(getTraceIdBaseQuery)
	kustoParameters := kql.NewParameters()

	if query.ServiceName != "" {
		kustoStmt = kustoStmt.AddLiteral(` | where ProcessServiceName == ParamProcessServiceName`)
		kustoParameters = kustoParameters.AddString("ParamProcessServiceName", query.ServiceName)

	}

	if query.OperationName != "" {
		kustoStmt = kustoStmt.AddLiteral(` | where SpanName == ParamOperationName`)
		kustoParameters = kustoParameters.AddString("ParamOperationName", query.OperationName)
	}

	if query.Tags != nil {
		for k, v := range query.Tags {
			kustoStmt = kustoStmt.AddUnsafe(buildTagFilter(k, v))
		}
	}

	kustoStmt = kustoStmt.AddLiteral(` | where StartTime > ParamStartTimeMin`)
	kustoParameters = kustoParameters.AddDateTime("ParamStartTimeMin", query.StartTimeMin)

	kustoStmt = kustoStmt.AddLiteral(` | where StartTime < ParamStartTimeMax`)
	kustoParameters = kustoParameters.AddDateTime("ParamStartTimeMax", query.StartTimeMax)

	if query.DurationMin != 0 {
		kustoStmt = kustoStmt.AddLiteral(` | where datetime_diff('microsecond', EndTime, StartTime) >= ParamDurationMin`)
		kustoParameters = kustoParameters.AddLong("ParamDurationMin", query.DurationMin.Microseconds())
	}

	if query.DurationMax != 0 {
		kustoStmt = kustoStmt.AddLiteral(` | where datetime_diff('microsecond', EndTime, StartTime) <= ParamDurationMax`)
		kustoParameters = kustoParameters.AddLong("ParamDurationMax", query.DurationMax.Microseconds())
	}

	kustoStmt = kustoStmt.AddLiteral("| summarize by TraceID")

	if query.NumTraces != 0 {
		kustoStmt.AddLiteral(`| sample ParamNumTraces`)
		kustoParameters = kustoParameters.AddInt("ParamNumTraces", int32(query.NumTraces))
	}

	r.logger.Info("FindTraceIDs query", "kql", kustoStmt.String(), "durationMin_us", query.DurationMin.Microseconds(), "durationMax_us", query.DurationMax.Microseconds(), "service", query.ServiceName, "startMin", query.StartTimeMin, "startMax", query.StartTimeMax)
	clientRequestId := GetClientId()
	dataset, err := r.client.Query(ctx, r.database, kustoStmt, append(r.defaultReadOptions, azkustodata.ClientRequestID(clientRequestId), azkustodata.QueryParameters(kustoParameters))...)
	if err != nil {
		return nil, err
	}

	var traceIds []model.TraceID
	for _, row := range dataset.Tables()[0].Rows() {
		rec := TraceID{}
		if err := row.ToStruct(&rec); err != nil {
			return nil, err
		}
		traceID, err := model.TraceIDFromString(rec.TraceID)
		if err != nil {
			return nil, err
		}
		traceIds = append(traceIds, traceID)
	}

	if len(traceIds) == 0 {
		r.logger.Warn("FindTraceIDs: query returned 0 results", "tags", query.Tags, "service", query.ServiceName, "durationMin", query.DurationMin, "durationMax", query.DurationMax, "startMin", query.StartTimeMin, "startMax", query.StartTimeMax)
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

	kustoStmt := kql.New("let TraceIDs = (").AddTable(r.tableName).AddLiteral(getTracesBaseQuery)
	kustoParameters := kql.NewParameters()

	if query.ServiceName != "" {
		kustoStmt = kustoStmt.AddLiteral(` | where ProcessServiceName == ParamProcessServiceName`)
		kustoParameters = kustoParameters.AddString("ParamProcessServiceName", query.ServiceName)
	}

	if query.OperationName != "" {
		kustoStmt = kustoStmt.AddLiteral(` | where SpanName == ParamOperationName`)
		kustoParameters = kustoParameters.AddString("ParamOperationName", query.OperationName)
	}

	if query.Tags != nil {
		for k, v := range query.Tags {
			kustoStmt = kustoStmt.AddUnsafe(buildTagFilter(k, v))
		}
	}

	kustoStmt = kustoStmt.AddLiteral(` | where StartTime > ParamStartTimeMin`)
	kustoParameters = kustoParameters.AddDateTime("ParamStartTimeMin", query.StartTimeMin)

	kustoStmt = kustoStmt.AddLiteral(` | where StartTime < ParamStartTimeMax`)
	kustoParameters = kustoParameters.AddDateTime("ParamStartTimeMax", query.StartTimeMax)

	if query.DurationMin != 0 {
		kustoStmt = kustoStmt.AddLiteral(` | where datetime_diff('microsecond', EndTime, StartTime) >= ParamDurationMin`)
		kustoParameters = kustoParameters.AddLong("ParamDurationMin", query.DurationMin.Microseconds())
	}

	if query.DurationMax != 0 {
		kustoStmt = kustoStmt.AddLiteral(` | where datetime_diff('microsecond', EndTime, StartTime) <= ParamDurationMax`)
		kustoParameters = kustoParameters.AddLong("ParamDurationMax", query.DurationMax.Microseconds())
	}

	kustoStmt = kustoStmt.AddLiteral(" | summarize by TraceID")

	kustoStmt = kustoStmt.AddLiteral(` | sample ParamNumTraces`)
	kustoParameters = kustoParameters.AddInt("ParamNumTraces", int32(query.NumTraces))

	kustoStmt = kustoStmt.AddLiteral(`); `).AddTable(r.tableName).AddLiteral(getTracesBaseQuery)

	kustoStmt = kustoStmt.AddLiteral(` | where StartTime > ParamStartTimeMin`)
	kustoParameters = kustoParameters.AddDateTime("ParamStartTimeMin", query.StartTimeMin)

	kustoStmt = kustoStmt.AddLiteral(` | where StartTime < ParamStartTimeMax`)
	kustoParameters = kustoParameters.AddDateTime("ParamStartTimeMax", query.StartTimeMax)

	kustoStmt = kustoStmt.AddLiteral(` | where TraceID in (TraceIDs) | project-rename Tags=TraceAttributes,Logs=Events,ProcessTags=ResourceAttributes|extend References=iff(isempty(ParentID),todynamic("[]"),pack_array(bag_pack("refType","CHILD_OF","traceID",TraceID,"spanID",ParentID)))`)

	r.logger.Info("FindTraces query", "kql", kustoStmt.String(), "durationMin_us", query.DurationMin.Microseconds(), "durationMax_us", query.DurationMax.Microseconds(), "service", query.ServiceName, "startMin", query.StartTimeMin, "startMax", query.StartTimeMax)
	clientRequestId := GetClientId()
	dataset, err := r.client.Query(ctx, r.database, kustoStmt, append(r.defaultReadOptions, azkustodata.ClientRequestID(clientRequestId), azkustodata.QueryParameters(kustoParameters))...)
	if err != nil {
		return nil, err
	}

	m := make(map[model.TraceID][]*model.Span)

	for _, row := range dataset.Tables()[0].Rows() {
		rec := kustoSpan{}
		if err := row.ToStruct(&rec); err != nil {
			return nil, err
		}

		var span *model.Span
		span, err = transformKustoSpanToModelSpan(&rec, r.logger)

		if err != nil {
			return nil, err
		}
		m[span.TraceID] = append(m[span.TraceID], span)
	}

	var traces []*model.Trace

	for _, spanArray := range m {
		trace := model.Trace{Spans: spanArray}
		//r.logger.Debug("Trace ==> " + trace.String())
		traces = append(traces, &trace)
	}

	if len(traces) == 0 {
		r.logger.Warn("FindTraces: query returned 0 results", "tags", query.Tags, "service", query.ServiceName, "durationMin", query.DurationMin, "durationMax", query.DurationMax, "startMin", query.StartTimeMin, "startMax", query.StartTimeMax)
	}

	return traces, err
}

// maxDependencyLookback caps the time window for dependency queries to avoid OOM on large datasets.
const maxDependencyLookback = 2 * time.Hour

// GetDependencies returns DependencyLinks of services.
// When caching is enabled, results are served from a background-refreshed cache.
func (r *kustoSpanReader) GetDependencies(ctx context.Context, endTs time.Time, lookback time.Duration) ([]model.DependencyLink, error) {
	// When background refresh is active, always serve from cache
	if r.depRefresher != nil {
		if links, ok := r.depRefresher.getCachedDependencies(ctx); ok {
			r.logger.Debug("GetDependencies: returning from background cache", "links", len(links))
			return links, nil
		}
		r.logger.Warn("GetDependencies: cache not available, falling back to direct query")
	}

	if lookback > maxDependencyLookback {
		r.logger.Warn("Capping dependency lookback", "requested", lookback, "max", maxDependencyLookback)
		lookback = maxDependencyLookback
	}

	return r.fetchDependencies(ctx, endTs, lookback)
}

// fetchDependencies executes the dependency graph query against Kusto directly.
func (r *kustoSpanReader) fetchDependencies(ctx context.Context, endTs time.Time, lookback time.Duration) ([]model.DependencyLink, error) {
	type kustoDependencyLink struct {
		Parent    string `kusto:"Parent"`
		Child     string `kusto:"Child"`
		CallCount int64  `kusto:"CallCount"`
	}

	kustoStmt := kql.New(queryResultsCacheAge + "let spans = ").AddTable(r.tableName).AddLiteral(getDependenciesGraphQuery)
	kustoParams := kql.NewParameters().AddDateTime("ParamEndTs", endTs).AddTimespan("ParamLookBack", lookback)
	clientRequestId := GetClientId()
	dataset, err := r.client.Query(ctx, r.database, kustoStmt, append(r.defaultReadOptions, azkustodata.ClientRequestID(clientRequestId), azkustodata.QueryParameters(kustoParams))...)
	if err != nil {
		return nil, err
	}

	var dependencyLinks []model.DependencyLink
	for _, row := range dataset.Tables()[0].Rows() {
		rec := kustoDependencyLink{}
		if err := row.ToStruct(&rec); err != nil {
			return nil, err
		}

		dependencyLinks = append(dependencyLinks, model.DependencyLink{
			Parent:    rec.Parent,
			Child:     rec.Child,
			CallCount: uint64(rec.CallCount),
		})
	}

	return dependencyLinks, err
}
