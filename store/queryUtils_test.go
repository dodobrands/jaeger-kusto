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
	assert.Contains(t, getDependenciesGraphQuery(), `ProcessServiceName = tostring(column_ifexists("ServiceName", ResourceAttributes.['service.name']))`)
}

func normalizeKQL(query string) string {
	return strings.Join(strings.Fields(query), "")
}
