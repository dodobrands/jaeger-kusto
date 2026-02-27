package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePromQL_Latency(t *testing.T) {
	query := `histogram_quantile(0.95, sum(rate(duration_milliseconds_bucket{service_name =~ "svc1|svc2", span_kind =~ "SPAN_KIND_SERVER"}[10m])) by (service_name, span_name, le))`

	parsed, err := ParsePromQL(query)
	require.NoError(t, err)
	assert.Equal(t, QueryTypeLatency, parsed.Type)
	assert.InDelta(t, 0.95, parsed.Quantile, 0.001)
	assert.Equal(t, []string{"svc1", "svc2"}, parsed.Services)
	assert.Equal(t, []string{"SPAN_KIND_SERVER"}, parsed.SpanKinds)
	assert.Equal(t, 10*time.Minute, parsed.RateWindow)
	assert.Equal(t, []string{"service_name", "span_name"}, parsed.GroupBy)
}

func TestParsePromQL_LatencyP99(t *testing.T) {
	query := `histogram_quantile(0.99, sum(rate(duration_milliseconds_bucket{service_name =~ "frontend", span_kind =~ "SPAN_KIND_SERVER|SPAN_KIND_CLIENT"}[5m])) by (service_name, le))`

	parsed, err := ParsePromQL(query)
	require.NoError(t, err)
	assert.Equal(t, QueryTypeLatency, parsed.Type)
	assert.InDelta(t, 0.99, parsed.Quantile, 0.001)
	assert.Equal(t, []string{"frontend"}, parsed.Services)
	assert.Equal(t, []string{"SPAN_KIND_SERVER", "SPAN_KIND_CLIENT"}, parsed.SpanKinds)
	assert.Equal(t, 5*time.Minute, parsed.RateWindow)
	assert.Equal(t, []string{"service_name"}, parsed.GroupBy)
}

func TestParsePromQL_CallRate(t *testing.T) {
	query := `sum(rate(calls_total{service_name =~ "svc1", span_kind =~ "SPAN_KIND_SERVER"}[10m])) by (service_name)`

	parsed, err := ParsePromQL(query)
	require.NoError(t, err)
	assert.Equal(t, QueryTypeCallRate, parsed.Type)
	assert.Equal(t, []string{"svc1"}, parsed.Services)
	assert.Equal(t, []string{"SPAN_KIND_SERVER"}, parsed.SpanKinds)
	assert.Equal(t, 10*time.Minute, parsed.RateWindow)
	assert.Equal(t, []string{"service_name"}, parsed.GroupBy)
}

func TestParsePromQL_ErrorRate(t *testing.T) {
	query := `sum(rate(calls_total{service_name =~ "svc1", status_code = "STATUS_CODE_ERROR", span_kind =~ "SPAN_KIND_SERVER"}[10m])) by (service_name)`

	parsed, err := ParsePromQL(query)
	require.NoError(t, err)
	assert.Equal(t, QueryTypeErrorRate, parsed.Type)
	assert.Equal(t, []string{"svc1"}, parsed.Services)
	assert.Equal(t, []string{"SPAN_KIND_SERVER"}, parsed.SpanKinds)
	assert.Equal(t, 10*time.Minute, parsed.RateWindow)
	assert.Equal(t, []string{"service_name"}, parsed.GroupBy)
}

func TestParsePromQL_CallRateGroupByOperation(t *testing.T) {
	query := `sum(rate(calls_total{service_name =~ "svc1", span_kind =~ "SPAN_KIND_SERVER"}[10m])) by (service_name, span_name)`

	parsed, err := ParsePromQL(query)
	require.NoError(t, err)
	assert.Equal(t, QueryTypeCallRate, parsed.Type)
	assert.Equal(t, []string{"service_name", "span_name"}, parsed.GroupBy)
}

func TestParsePromQL_Invalid(t *testing.T) {
	_, err := ParsePromQL("some random query")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized PromQL query pattern")
}

func TestParsePromDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"10m", 10 * time.Minute},
		{"1h", time.Hour},
		{"30s", 30 * time.Second},
		{"1d", 24 * time.Hour},
	}
	for _, tt := range tests {
		d, err := parsePromDuration(tt.input)
		require.NoError(t, err, "input: %s", tt.input)
		assert.Equal(t, tt.expected, d, "input: %s", tt.input)
	}
}

func TestTranslateToPrometheus(t *testing.T) {
	ts1 := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2024, 3, 15, 10, 1, 0, 0, time.UTC)

	rows := []MetricRow{
		{Timestamp: ts1, ServiceName: "svc1", SpanName: "GET /api", Value: 10.5},
		{Timestamp: ts2, ServiceName: "svc1", SpanName: "GET /api", Value: 12.3},
		{Timestamp: ts1, ServiceName: "svc2", SpanName: "POST /submit", Value: 5.0},
	}

	// Without groupByOperation
	resp := TranslateToPrometheus(rows, false)
	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, "matrix", resp.Data.ResultType)
	assert.Len(t, resp.Data.Result, 2)

	// svc1 should have 2 values merged
	assert.Equal(t, "svc1", resp.Data.Result[0].Metric["service_name"])
	assert.Len(t, resp.Data.Result[0].Values, 2)

	// svc2 should have 1 value
	assert.Equal(t, "svc2", resp.Data.Result[1].Metric["service_name"])
	assert.Len(t, resp.Data.Result[1].Values, 1)

	// With groupByOperation
	resp = TranslateToPrometheus(rows, true)
	assert.Len(t, resp.Data.Result, 2)
	assert.Equal(t, "GET /api", resp.Data.Result[0].Metric["span_name"])
	assert.Equal(t, "POST /submit", resp.Data.Result[1].Metric["span_name"])
}

func TestTranslateToPrometheus_Empty(t *testing.T) {
	resp := TranslateToPrometheus(nil, false)
	assert.Equal(t, "success", resp.Status)
	assert.Empty(t, resp.Data.Result)
}

func TestParsePrometheusTime(t *testing.T) {
	// Unix timestamp
	ts, err := parsePrometheusTime("1709000000")
	require.NoError(t, err)
	assert.Equal(t, int64(1709000000), ts.Unix())

	// Fractional
	ts, err = parsePrometheusTime("1709000000.5")
	require.NoError(t, err)
	assert.Equal(t, int64(1709000000), ts.Unix())

	// RFC3339
	ts, err = parsePrometheusTime("2024-03-15T10:00:00Z")
	require.NoError(t, err)
	assert.Equal(t, 2024, ts.Year())

	// Invalid
	_, err = parsePrometheusTime("")
	assert.Error(t, err)
}

func TestParseStepDuration(t *testing.T) {
	// Float seconds
	d, err := parseStepDuration("60")
	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, d)

	// Prometheus duration
	d, err = parseStepDuration("5m")
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, d)

	// Empty defaults to 60s
	d, err = parseStepDuration("")
	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, d)
}
