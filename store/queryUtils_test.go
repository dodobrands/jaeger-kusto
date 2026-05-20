package store

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceNameExpression_PrefersPhysicalColumn(t *testing.T) {
	assert.Equal(t, `tostring(column_ifexists("ServiceName", ResourceAttributes.['service.name']))`, serviceNameExpression())
}

func TestBuildGetServicesStatement_UsesTraceTableByDefault(t *testing.T) {
	stmt := buildGetServicesStatement("OTELTraces", "")

	assert.Equal(
		t,
		normalizeKQL(`set query_results_cache_max_age = time(5m); OTELTraces | extend ProcessServiceName=tostring(column_ifexists("ServiceName", ResourceAttributes.['service.name'])) | where ProcessServiceName!="" | summarize by ProcessServiceName | sort by ProcessServiceName asc`),
		normalizeKQL(stmt.String()),
	)
}

func TestBuildGetServicesStatement_UsesMaterializedViewWhenConfigured(t *testing.T) {
	stmt := buildGetServicesStatement("OTELTraces", "ServiceCatalog")

	assert.Equal(
		t,
		normalizeKQL(`set query_results_cache_max_age = time(5m); materialized_view("ServiceCatalog") | extend ProcessServiceName=ServiceName | where ProcessServiceName!="" | summarize by ProcessServiceName | sort by ProcessServiceName asc`),
		normalizeKQL(stmt.String()),
	)
}

func TestQueriesUseResolvedServiceName(t *testing.T) {
	assert.Contains(t, getTraceQuery(), `ProcessServiceName=tostring(column_ifexists("ServiceName", ResourceAttributes.['service.name']))`)
	assert.Contains(t, getOpsWithParamsQuery(), `ProcessServiceName=tostring(column_ifexists("ServiceName", ResourceAttributes.['service.name']))`)
	assert.Contains(t, getTraceIdBaseQuery(), `ProcessServiceName=tostring(column_ifexists("ServiceName", ResourceAttributes.['service.name']))`)
	assert.Contains(t, getTracesBaseQuery(), `ProcessServiceName=tostring(column_ifexists("ServiceName", ResourceAttributes.['service.name']))`)
	assert.Contains(t, dependencyGraphSpansQuery(), `ProcessServiceName = tostring(column_ifexists("ServiceName", ResourceAttributes.['service.name']))`)
}

func TestNormalizeDependencySkipServices_NormalizesAndDeduplicates(t *testing.T) {
	assert.Equal(t, []string{"proxy", "gateway"}, normalizeDependencySkipServices([]string{" Proxy ", "proxy", "", "Gateway"}))
}

func TestDependencyGraphSpansQuery_UsesTraceScopedNodeIDs(t *testing.T) {
	query := dependencyGraphSpansQuery()

	assert.Contains(t, query, `SpanNodeID=strcat(TraceID, "/", SpanID)`)
	assert.Contains(t, query, `ParentNodeID=strcat(TraceID, "/", ParentID)`)
	assert.Contains(t, query, `where isnotempty(ParentID)`)
}

func TestGetDependenciesGraphQuery_UsesDirectEdges(t *testing.T) {
	query := getDependenciesGraphQuery()

	assert.NotContains(t, query, `ParamDependencySkipServices`)
	assert.Contains(t, query, `graph-match (parent)-[]->(child)`)
}

func TestGetCollapsedDependenciesGraphQuery_CollapsesSkippedServices(t *testing.T) {
	query := getCollapsedDependenciesGraphQuery([]string{"proxy"})

	assert.Contains(t, query, `graph-match (parent)-[dependencyPath*1..32]->(child)`)
	assert.Contains(t, query, `ParamDependencySkipServices`)
	assert.Contains(t, query, `all(inner_nodes(dependencyPath), set_has_element(ParamDependencySkipServices, tolower(ServiceName)))`)
	assert.Contains(t, query, `tolower(parent.ServiceName)`)
	assert.Contains(t, query, `tolower(child.ServiceName)`)
}

func normalizeKQL(query string) string {
	return strings.Join(strings.Fields(query), "")
}
