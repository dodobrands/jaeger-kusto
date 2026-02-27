package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-kusto-go/kusto/data/value"
	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/data/errors"
	"github.com/Azure/azure-kusto-go/kusto/data/table"
	"github.com/Azure/azure-kusto-go/kusto/kql"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/spanstore"
)

type kustoSpanReader struct {
	client             kustoReaderClient
	database           string
	tableName          string
	logger             hclog.Logger
	defaultReadOptions []kusto.QueryOption
	cache              *discoveryCache      // nil when caching is disabled
	depRefresher       *dependencyRefresher // nil when caching is disabled
}

type kustoReaderClient interface {
	Query(ctx context.Context, db string, query kusto.Statement, options ...kusto.QueryOption) (*kusto.RowIterator, error)
}


func newKustoSpanReader(factory *kustoFactory, logger hclog.Logger, defaultReadOptions []kusto.QueryOption, cache *discoveryCache) (*kustoSpanReader, error) {
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

// GetTrace finds trace by TraceID
func (r *kustoSpanReader) GetTrace(ctx context.Context, traceID model.TraceID) (*model.Trace, error) {
	kustoStmt := kql.New("").AddTable(r.tableName).AddLiteral(getTraceQuery)
	kustoStmtParams := kql.NewParameters().AddString("ParamTraceID", traceID.String())

	clientRequestId := GetClientId()
	// Append a client request id as well to the request
	iter, err := r.client.Query(ctx, r.database, kustoStmt, append(r.defaultReadOptions,
		kusto.ClientRequestID(clientRequestId), kusto.QueryParameters(kustoStmtParams))...)
	if err != nil {
		r.logger.Error("Failed running GetTrace query. TraceID: %s. ClientRequestId : %s", traceID.String(), clientRequestId)
		return nil, err
	}
	defer iter.Stop()

	var spans []*model.Span
	err = iter.DoOnRowOrError(
		func(row *table.Row, e *errors.Error) error {
			if e != nil {
				return e
			}
			rec := kustoSpan{}
			if err := row.ToStruct(&rec); err != nil {
				return err
			}
			var span *model.Span
			span, err = transformKustoSpanToModelSpan(&rec, r.logger)
			if err != nil {
				r.logger.Error(fmt.Sprintf("Error in transformKustoSpanToModelSpan. TraceId: %s SpanId: %s", rec.TraceID, rec.SpanID), err)
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
	iter, err := r.client.Query(ctx, r.database, kustoStmt, append(r.defaultReadOptions, kusto.ClientRequestID(clientRequestId))...)

	if err != nil {
		r.logger.Error("Failed running GetServices query. ClientRequestId : %s", clientRequestId)
		return nil, err
	}
	defer iter.Stop()

	type Service struct {
		ServiceName string `kusto:"ProcessServiceName"`
	}

	var services []string
	err = iter.DoOnRowOrError(
		func(row *table.Row, e *errors.Error) error {
			if e != nil {
				return e
			}
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
	var iter *kusto.RowIterator
	var err error
	if query.ServiceName == "" && query.SpanKind == "" {
		kustoStmt := kql.New(queryResultsCacheAge).AddTable(r.tableName).AddLiteral(getOpsWithNoParamsQuery)
		iter, err = r.client.Query(ctx, r.database, kustoStmt, append(r.defaultReadOptions, kusto.ClientRequestID(clientRequestId))...)
	}

	if query.ServiceName != "" && query.SpanKind == "" {
		kustoStmt := kql.New(queryResultsCacheAge).AddTable(r.tableName).AddLiteral(getOpsWithParamsQuery)
		kustoStmtParams := kql.NewParameters().AddString("ParamProcessServiceName", query.ServiceName)

		iter, err = r.client.Query(ctx, r.database, kustoStmt, append(r.defaultReadOptions, kusto.ClientRequestID(clientRequestId), kusto.QueryParameters(kustoStmtParams))...)
	}

	if err != nil {
		r.logger.Error("Failed running GetOperations query. ClientRequestId : %s", clientRequestId)
		return nil, err
	}
	defer iter.Stop()

	operations := []spanstore.Operation{}
	err = iter.DoOnRowOrError(
		func(row *table.Row, e *errors.Error) error {
			if e != nil {
				return e
			}
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
			replacedTag := strings.ReplaceAll(k, ".", TagDotReplacementCharacter)
			tagFilter := fmt.Sprintf(" | where TraceAttributes['%s'] == '%s' or ResourceAttributes['%s'] == '%s'", replacedTag, v, replacedTag, v)
			kustoStmt = kustoStmt.AddUnsafe(tagFilter)
		}
	}

	kustoStmt = kustoStmt.AddLiteral(` | where StartTime > ParamStartTimeMin`)
	kustoParameters = kustoParameters.AddDateTime("ParamStartTimeMin", query.StartTimeMin)

	kustoStmt = kustoStmt.AddLiteral(` | where StartTime < ParamStartTimeMax`)
	kustoParameters = kustoParameters.AddDateTime("ParamStartTimeMax", query.StartTimeMax)

	if query.DurationMin != 0 {
		kustoStmt = kustoStmt.AddLiteral(` | where Duration > ParamDurationMin`)
		kustoParameters = kustoParameters.AddLong("ParamDurationMin", query.DurationMin.Microseconds())
	}

	if query.DurationMax != 0 {
		kustoStmt = kustoStmt.AddLiteral(` | where Duration < ParamDurationMax`)
		kustoParameters = kustoParameters.AddLong("ParamDurationMax", query.DurationMax.Microseconds())
	}

	kustoStmt = kustoStmt.AddLiteral("| summarize by TraceID")

	if query.NumTraces != 0 {
		kustoStmt.AddLiteral(`| sample ParamNumTraces`)
		kustoParameters = kustoParameters.AddInt("ParamNumTraces", int32(query.NumTraces))
	}

	r.logger.Debug("FindTraceIDs query: %s", kustoStmt.String())
	clientRequestId := GetClientId()
	iter, err := r.client.Query(ctx, r.database, kustoStmt, append(r.defaultReadOptions, kusto.ClientRequestID(clientRequestId), kusto.QueryParameters(kustoParameters))...)
	if err != nil {
		return nil, err
	}
	defer iter.Stop()

	var traceIds []model.TraceID
	err = iter.DoOnRowOrError(
		func(row *table.Row, e *errors.Error) error {
			if e != nil {
				return e
			}
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
			tagFilter := fmt.Sprintf(" | where TraceAttributes['%s'] == '%s' or ResourceAttributes['%s'] == '%s'", k, v, k, v)
			kustoStmt = kustoStmt.AddUnsafe(tagFilter)
		}
	}

	kustoStmt = kustoStmt.AddLiteral(` | where StartTime > ParamStartTimeMin`)
	kustoParameters = kustoParameters.AddDateTime("ParamStartTimeMin", query.StartTimeMin)

	kustoStmt = kustoStmt.AddLiteral(` | where StartTime < ParamStartTimeMax`)
	kustoParameters = kustoParameters.AddDateTime("ParamStartTimeMax", query.StartTimeMax)

	if query.DurationMin != 0 {
		kustoStmt = kustoStmt.AddLiteral(` | where Duration > ParamDurationMin`)
		kustoParameters = kustoParameters.AddLong("ParamDurationMin", query.DurationMin.Microseconds())
	}

	if query.DurationMax != 0 {
		kustoStmt = kustoStmt.AddLiteral(` | where Duration < ParamDurationMax`)
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

	r.logger.Debug("FindTraces query: %s", kustoStmt.String())
	clientRequestId := GetClientId()
	iter, err := r.client.Query(ctx, r.database, kustoStmt, append(r.defaultReadOptions, kusto.ClientRequestID(clientRequestId), kusto.QueryParameters(kustoParameters))...)
	if err != nil {
		return nil, err
	}
	defer iter.Stop()

	m := make(map[model.TraceID][]*model.Span)

	err = iter.DoOnRowOrError(
		func(row *table.Row, e *errors.Error) error {
			if e != nil {
				return e
			}
			rec := kustoSpan{}
			if err := row.ToStruct(&rec); err != nil {
				return err
			}

			var span *model.Span
			span, err = transformKustoSpanToModelSpan(&rec, r.logger)

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
		//r.logger.Debug("Trace ==> " + trace.String())
		traces = append(traces, &trace)
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
		Parent    string     `kusto:"Parent"`
		Child     string     `kusto:"Child"`
		CallCount value.Long `kusto:"CallCount"`
	}

	kustoStmt := kql.New(queryResultsCacheAge + "let spans = ").AddTable(r.tableName).AddLiteral(getDependenciesGraphQuery)
	kustoParams := kql.NewParameters().AddDateTime("ParamEndTs", endTs).AddTimespan("ParamLookBack", lookback)
	clientRequestId := GetClientId()
	iter, err := r.client.Query(ctx, r.database, kustoStmt, append(r.defaultReadOptions, kusto.ClientRequestID(clientRequestId), kusto.QueryParameters(kustoParams))...)
	if err != nil {
		return nil, err
	}
	defer iter.Stop()

	var dependencyLinks []model.DependencyLink
	err = iter.DoOnRowOrError(
		func(row *table.Row, e *errors.Error) error {
			if e != nil {
				return e
			}
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
