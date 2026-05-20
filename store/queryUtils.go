package store

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/azure-kusto-go/azkustodata/kql"
	"github.com/jaegertracing/jaeger/storage/spanstore"
)

// taken from https://github.com/logzio/jaeger-logzio/blob/master/store/store.go
var (
	// ErrServiceNameNotSet occurs when attempting to query with an empty service name
	ErrServiceNameNotSet = errors.New("service Name must be set")

	// ErrStartTimeMinGreaterThanMax occurs when start time min is above start time max
	ErrStartTimeMinGreaterThanMax = errors.New("start Time Minimum is above Maximum")

	// ErrDurationMinGreaterThanMax occurs when duration min is above duration max
	ErrDurationMinGreaterThanMax = errors.New("duration Minimum is above Maximum")

	// ErrMalformedRequestObject occurs when a request object is nil
	ErrMalformedRequestObject = errors.New("malformed request object")

	// ErrStartAndEndTimeNotSet occurs when start time and end time are not set
	ErrStartAndEndTimeNotSet = errors.New("start and End Time must be set")
)

const (
	queryResultsCacheAge  = `set query_results_cache_max_age = time(5m);`
	dependencySkipMaxHops = 32

	getOpsWithNoParams      = `getOpsWithNoParams`
	getOpsWithNoParamsQuery = `
	| summarize count() by SpanName , SpanKind
	| sort by count_
	| project OperationName=SpanName,SpanKind`
)

func serviceNameExpression() string {
	return `tostring(column_ifexists("ServiceName", ResourceAttributes.['service.name']))`
}

func getTraceQuery() string {
	return fmt.Sprintf(` | where TraceID == ParamTraceID | extend Duration=datetime_diff('microsecond',EndTime,StartTime) , ProcessServiceName=%s | project-rename Tags=TraceAttributes,Logs=Events,ProcessTags=ResourceAttributes| extend References=iff(isempty(ParentID),todynamic("[]"),pack_array(bag_pack("refType","CHILD_OF","traceID",TraceID,"spanID",ParentID)))`, serviceNameExpression())
}

func getServicesQuery(processServiceNameExpr string) string {
	return fmt.Sprintf(`| extend ProcessServiceName=%s
	| where ProcessServiceName!=""
	| summarize by ProcessServiceName
	| sort by ProcessServiceName asc`, processServiceNameExpr)
}

func buildGetServicesStatement(traceTableName, serviceCatalogViewName string) *kql.Builder {
	stmt := kql.New(queryResultsCacheAge)
	if serviceCatalogViewName != "" {
		return stmt.
			AddFunction("materialized_view").
			AddLiteral("(").
			AddString(serviceCatalogViewName).
			AddLiteral(")").
			AddUnsafe(getServicesQuery("ServiceName"))
	}

	return stmt.AddTable(traceTableName).AddUnsafe(getServicesQuery(serviceNameExpression()))
}

func getOpsWithParamsQuery() string {
	return fmt.Sprintf(` | extend ProcessServiceName=%s
	| where ProcessServiceName == ParamProcessServiceName
	| summarize count() by SpanName , SpanKind
	| sort by count_
	| project OperationName=SpanName,SpanKind`, serviceNameExpression())
}

func normalizeDependencySkipServices(services []string) []string {
	normalized := make([]string, 0, len(services))
	seen := make(map[string]struct{}, len(services))

	for _, service := range services {
		service = strings.TrimSpace(strings.ToLower(service))
		if service == "" {
			continue
		}
		if _, ok := seen[service]; ok {
			continue
		}
		seen[service] = struct{}{}
		normalized = append(normalized, service)
	}

	return normalized
}

func dependencyGraphSpansQuery() string {
	return fmt.Sprintf(`
	| where StartTime between ((ParamEndTs - ParamLookBack) .. ParamEndTs)
	| where isnotempty(ParentID)
	| extend ProcessServiceName = %s
	| project SpanNodeID=strcat(TraceID, "/", SpanID), ParentNodeID=strcat(TraceID, "/", ParentID), ServiceName=ProcessServiceName`, serviceNameExpression())
}

// getDependenciesGraphQuery uses Kusto graph semantics for a single time-filtered scan
// instead of the previous self-join which scanned the full table on the parent side.
// The table name is injected via AddTable before this literal.
func getDependenciesGraphQuery() string {
	return `
	spans
	| make-graph ParentNodeID --> SpanNodeID with spans on SpanNodeID
	| graph-match (parent)-[]->(child)
		where parent.ServiceName != child.ServiceName
		project Parent=parent.ServiceName, Child=child.ServiceName
	| summarize CallCount=count() by Parent, Child`
}

func getCollapsedDependenciesGraphQuery(skipServices []string) string {
	if len(skipServices) == 0 {
		return getDependenciesGraphQuery()
	}

	return fmt.Sprintf(`
	spans
	| make-graph ParentNodeID --> SpanNodeID with spans on SpanNodeID
	| graph-match (parent)-[dependencyPath*1..%d]->(child)
		where parent.ServiceName != child.ServiceName
			and not(set_has_element(ParamDependencySkipServices, tolower(parent.ServiceName)))
			and not(set_has_element(ParamDependencySkipServices, tolower(child.ServiceName)))
			and all(inner_nodes(dependencyPath), set_has_element(ParamDependencySkipServices, tolower(ServiceName)))
		project Parent=parent.ServiceName, Child=child.ServiceName
	| summarize CallCount=count() by Parent, Child`, dependencySkipMaxHops)
}

func getTraceIdBaseQuery() string {
	return fmt.Sprintf(` | extend Duration=datetime_diff('microsecond',EndTime,StartTime) , ProcessServiceName=%s`, serviceNameExpression())
}

func getTracesBaseQuery() string {
	return fmt.Sprintf(` | extend ProcessServiceName=%s,Duration=datetime_diff('microsecond',EndTime,StartTime)`, serviceNameExpression())
}

// taken from https://github.com/logzio/jaeger-logzio/blob/master/store/queryUtils.go
func validateQuery(p *spanstore.TraceQueryParameters) error {
	if p == nil {
		return ErrMalformedRequestObject
	}
	if p.ServiceName == "" && len(p.Tags) > 0 {
		return ErrServiceNameNotSet
	}
	if p.StartTimeMin.IsZero() || p.StartTimeMax.IsZero() {
		return ErrStartAndEndTimeNotSet
	}
	if p.StartTimeMax.Before(p.StartTimeMin) {
		return ErrStartTimeMinGreaterThanMax
	}
	if p.DurationMin != 0 && p.DurationMax != 0 && p.DurationMin > p.DurationMax {
		return ErrDurationMinGreaterThanMax
	}
	if p.NumTraces > 500 {
		p.NumTraces = 500
	}
	return nil
}
