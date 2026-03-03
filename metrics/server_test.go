package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-kusto-go/azkustodata"
	"github.com/Azure/azure-kusto-go/azkustodata/query"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockKustoClient is a test double that returns no results.
type mockKustoClient struct{}

func (m *mockKustoClient) Query(_ context.Context, _ string, _ azkustodata.Statement, _ ...azkustodata.QueryOption) (query.Dataset, error) {
	// Return a nil dataset - we can't easily construct one without a real Kusto connection.
	// For unit tests, we test the parser/translator independently; server integration tests
	// verify the HTTP wiring.
	return nil, nil
}

func newTestServer() *Server {
	logger := hclog.NewNullLogger()
	reader := NewKustoMetricsReader(KustoMetricsReaderConfig{
		Client:      &mockKustoClient{},
		Database:    "testdb",
		TraceTable:  "OTELTraces",
		MetricsView: "",
		Logger:      logger,
		ReadOptions: nil,
	})
	return NewServer(ServerConfig{
		Reader: reader,
		Logger: logger,
	})
}

func TestServer_BuildInfo(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status/buildinfo", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "success", resp["status"])
}

func TestServer_QueryRange_MissingQuery(t *testing.T) {
	srv := newTestServer()

	form := url.Values{}
	form.Set("start", "1709000000")
	form.Set("end", "1709003600")
	form.Set("step", "60")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/query_range", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServer_QueryRange_InvalidQuery(t *testing.T) {
	srv := newTestServer()

	form := url.Values{}
	form.Set("query", "invalid_promql_query")
	form.Set("start", "1709000000")
	form.Set("end", "1709003600")
	form.Set("step", "60")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/query_range", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp["status"])
}

func TestServer_QueryRange_CallRate_NilDataset(t *testing.T) {
	// The mock returns nil dataset which will cause an error - this tests error handling
	srv := newTestServer()

	form := url.Values{}
	form.Set("query", `sum(rate(calls_total{service_name =~ "svc1", span_kind =~ "SPAN_KIND_SERVER"}[10m])) by (service_name)`)
	form.Set("start", "1709000000")
	form.Set("end", "1709003600")
	form.Set("step", "60")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/query_range", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	// Expect internal server error because mock returns nil dataset
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestFormatKustoDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{60 * time.Second, "1m"},
		{300 * time.Second, "5m"},
		{3600 * time.Second, "1h"},
		{30 * time.Second, "30s"},
		{90 * time.Second, "90s"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatKustoDuration(tt.d), "duration: %v", tt.d)
	}
}

func TestBuildBaseQuery(t *testing.T) {
	logger := hclog.NewNullLogger()

	t.Run("with materialized view", func(t *testing.T) {
		reader := NewKustoMetricsReader(KustoMetricsReaderConfig{
			Client:      &mockKustoClient{},
			Database:    "testdb",
			TraceTable:  "OTELTraces",
			MetricsView: "SpanMetrics",
			Logger:      logger,
		})

		parsed := &ParsedQuery{
			Services:  []string{"svc1", "svc2"},
			SpanKinds: []string{"SPAN_KIND_SERVER"},
		}
		start := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)
		end := time.Date(2024, 3, 15, 11, 0, 0, 0, time.UTC)

		q := reader.buildBaseQuery(parsed, start, end, time.Minute)

		assert.Contains(t, q, "SpanMetrics")
		assert.NotContains(t, q, "ResourceAttributes", "MV should not reference ResourceAttributes")
		assert.Contains(t, q, "where StartTime between")
		assert.Contains(t, q, "where ServiceName in~ ('svc1', 'svc2')")
		assert.Contains(t, q, "where SpanKind in~ ('SPAN_KIND_SERVER')")
	})

	t.Run("with raw table", func(t *testing.T) {
		reader := NewKustoMetricsReader(KustoMetricsReaderConfig{
			Client:      &mockKustoClient{},
			Database:    "testdb",
			TraceTable:  "OTELTraces",
			MetricsView: "",
			Logger:      logger,
		})

		parsed := &ParsedQuery{
			Services:  []string{"svc1"},
			SpanKinds: []string{"SPAN_KIND_SERVER"},
		}
		start := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)
		end := time.Date(2024, 3, 15, 11, 0, 0, 0, time.UTC)

		q := reader.buildBaseQuery(parsed, start, end, time.Minute)

		assert.Contains(t, q, "OTELTraces")
		assert.Contains(t, q, "ServiceName = tostring(ResourceAttributes.['service.name'])")
		assert.Contains(t, q, "where StartTime between")
	})
}

func TestContainsSpanName(t *testing.T) {
	assert.True(t, containsSpanName([]string{"service_name", "span_name"}))
	assert.False(t, containsSpanName([]string{"service_name"}))
	assert.False(t, containsSpanName(nil))
}
