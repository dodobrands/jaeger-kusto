package metrics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-kusto-go/azkustodata"
	"github.com/Azure/azure-kusto-go/azkustodata/kql"
	"github.com/Azure/azure-kusto-go/azkustodata/query"
	"github.com/hashicorp/go-hclog"
)

// KustoMetricsReader queries Kusto for RED metrics.
type KustoMetricsReader struct {
	client         kustoQueryClient
	database       string
	traceTable     string
	metricsView    string
	logger         hclog.Logger
	readOptions    []azkustodata.QueryOption
	useRawTable    bool // if true, query OTELTraces directly instead of materialized view
}

type kustoQueryClient interface {
	Query(ctx context.Context, db string, query azkustodata.Statement, options ...azkustodata.QueryOption) (query.Dataset, error)
}

// KustoMetricsReaderConfig holds configuration for the metrics reader.
type KustoMetricsReaderConfig struct {
	Client      kustoQueryClient
	Database    string
	TraceTable  string
	MetricsView string // materialized view name; if empty, uses TraceTable directly
	Logger      hclog.Logger
	ReadOptions []azkustodata.QueryOption
}

// NewKustoMetricsReader creates a new Kusto metrics reader.
func NewKustoMetricsReader(cfg KustoMetricsReaderConfig) *KustoMetricsReader {
	metricsView := cfg.MetricsView
	useRawTable := metricsView == ""
	if useRawTable {
		metricsView = cfg.TraceTable
	}
	return &KustoMetricsReader{
		client:      cfg.Client,
		database:    cfg.Database,
		traceTable:  cfg.TraceTable,
		metricsView: metricsView,
		logger:      cfg.Logger,
		readOptions: cfg.ReadOptions,
		useRawTable: useRawTable,
	}
}

// metricsRow maps to Kusto query result columns.
type metricsRow struct {
	TimeBucket  time.Time `kusto:"TimeBucket"`
	ServiceName string    `kusto:"ServiceName"`
	SpanName    string    `kusto:"SpanName"`
	MetricValue float64   `kusto:"MetricValue"`
}

// QueryCallRates returns call rate metrics from Kusto.
func (r *KustoMetricsReader) QueryCallRates(ctx context.Context, parsed *ParsedQuery, start, end time.Time, step time.Duration) ([]MetricRow, error) {
	groupByOp := containsSpanName(parsed.GroupBy)
	rateSeconds := parsed.RateWindow.Seconds()

	q := r.buildBaseQuery(parsed, start, end, step)

	if r.useRawTable {
		q += fmt.Sprintf(`| summarize total_calls = count() by ServiceName, SpanName, bin(StartTime, %s)
| extend MetricValue = todouble(total_calls) / %f
| project TimeBucket = StartTime, ServiceName, SpanName, MetricValue
| order by ServiceName asc, SpanName asc, TimeBucket asc`, formatKustoDuration(step), rateSeconds)
	} else {
		q += fmt.Sprintf(`| summarize total_calls = sum(call_count) by ServiceName, SpanName, bin(StartTime, %s)
| extend MetricValue = todouble(total_calls) / %f
| project TimeBucket = StartTime, ServiceName, SpanName, MetricValue
| order by ServiceName asc, SpanName asc, TimeBucket asc`, formatKustoDuration(step), rateSeconds)
	}

	return r.executeMetricsQuery(ctx, q, groupByOp)
}

// QueryErrorRates returns error rate metrics from Kusto.
func (r *KustoMetricsReader) QueryErrorRates(ctx context.Context, parsed *ParsedQuery, start, end time.Time, step time.Duration) ([]MetricRow, error) {
	groupByOp := containsSpanName(parsed.GroupBy)
	rateSeconds := parsed.RateWindow.Seconds()

	q := r.buildBaseQuery(parsed, start, end, step)

	if r.useRawTable {
		q += fmt.Sprintf(`| summarize total_errors = countif(SpanStatus == 'STATUS_CODE_ERROR') by ServiceName, SpanName, bin(StartTime, %s)
| extend MetricValue = todouble(total_errors) / %f
| project TimeBucket = StartTime, ServiceName, SpanName, MetricValue
| order by ServiceName asc, SpanName asc, TimeBucket asc`, formatKustoDuration(step), rateSeconds)
	} else {
		q += fmt.Sprintf(`| summarize total_errors = sum(error_count) by ServiceName, SpanName, bin(StartTime, %s)
| extend MetricValue = todouble(total_errors) / %f
| project TimeBucket = StartTime, ServiceName, SpanName, MetricValue
| order by ServiceName asc, SpanName asc, TimeBucket asc`, formatKustoDuration(step), rateSeconds)
	}

	return r.executeMetricsQuery(ctx, q, groupByOp)
}

// QueryLatencies returns latency percentile metrics from Kusto.
func (r *KustoMetricsReader) QueryLatencies(ctx context.Context, parsed *ParsedQuery, start, end time.Time, step time.Duration) ([]MetricRow, error) {
	groupByOp := containsSpanName(parsed.GroupBy)
	percentile := parsed.Quantile * 100 // e.g. 0.95 -> 95

	q := r.buildBaseQuery(parsed, start, end, step)

	if r.useRawTable {
		q += fmt.Sprintf(`| extend Duration_ms = datetime_diff('millisecond', EndTime, StartTime)
| summarize MetricValue = percentile(Duration_ms, %g) by ServiceName, SpanName, bin(StartTime, %s)
| project TimeBucket = StartTime, ServiceName, SpanName, MetricValue
| order by ServiceName asc, SpanName asc, TimeBucket asc`, percentile, formatKustoDuration(step))
	} else {
		// MV has pre-computed percentiles; pick the closest one via weighted average approximation
		pCol := percentileColumn(percentile)
		q += fmt.Sprintf(`| summarize MetricValue = avg(%s) by ServiceName, SpanName, bin(StartTime, %s)
| project TimeBucket = StartTime, ServiceName, SpanName, MetricValue
| order by ServiceName asc, SpanName asc, TimeBucket asc`, pCol, formatKustoDuration(step))
	}

	return r.executeMetricsQuery(ctx, q, groupByOp)
}

// percentileColumn maps a percentile value to the corresponding MV column name.
func percentileColumn(p float64) string {
	switch {
	case p <= 50:
		return "p50_ms"
	case p <= 75:
		return "p75_ms"
	case p <= 95:
		return "p95_ms"
	default:
		return "p99_ms"
	}
}

// buildBaseQuery generates the common prefix for all metrics KQL queries.
func (r *KustoMetricsReader) buildBaseQuery(parsed *ParsedQuery, start, end time.Time, _ time.Duration) string {
	var sb strings.Builder

	sb.WriteString(r.metricsView)

	// MV already has ServiceName column; raw table needs to extract it from ResourceAttributes
	if r.useRawTable {
		sb.WriteString("\n| extend ServiceName = tostring(ResourceAttributes.['service.name'])")
	}

	sb.WriteString(fmt.Sprintf("\n| where StartTime between (datetime(%s) .. datetime(%s))",
		start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339)))

	if len(parsed.Services) > 0 {
		quoted := quoteStrings(parsed.Services)
		sb.WriteString(fmt.Sprintf("\n| where ServiceName in~ (%s)", strings.Join(quoted, ", ")))
	}

	if len(parsed.SpanKinds) > 0 {
		quoted := quoteStrings(parsed.SpanKinds)
		sb.WriteString(fmt.Sprintf("\n| where SpanKind in~ (%s)", strings.Join(quoted, ", ")))
	}

	sb.WriteString("\n")
	return sb.String()
}

func (r *KustoMetricsReader) executeMetricsQuery(ctx context.Context, query string, _ bool) ([]MetricRow, error) {
	r.logger.Debug("executing metrics KQL query", "query", query)

	stmt := kql.New("").AddUnsafe(query)
	dataset, err := r.client.Query(ctx, r.database, stmt, r.readOptions...)
	if err != nil {
		r.logger.Error("failed executing metrics query", "error", err)
		return nil, fmt.Errorf("kusto metrics query failed: %w", err)
	}
	if dataset == nil {
		return nil, fmt.Errorf("kusto metrics query returned nil dataset")
	}

	tables := dataset.Tables()
	if len(tables) == 0 {
		return nil, fmt.Errorf("kusto metrics query returned no tables")
	}

	var rows []MetricRow
	for _, row := range tables[0].Rows() {
		rec := metricsRow{}
		if err := row.ToStruct(&rec); err != nil {
			return nil, err
		}
		rows = append(rows, MetricRow{
			Timestamp:   rec.TimeBucket,
			ServiceName: rec.ServiceName,
			SpanName:    rec.SpanName,
			Value:       rec.MetricValue,
		})
	}

	return rows, nil
}

func containsSpanName(groupBy []string) bool {
	for _, g := range groupBy {
		if g == "span_name" {
			return true
		}
	}
	return false
}

func escapeKustoString(s string) string {
	// In KQL, single quotes inside single-quoted string literals are escaped by doubling them.
	return strings.ReplaceAll(s, "'", "''")
}

func quoteStrings(ss []string) []string {
	result := make([]string, len(ss))
	for i, s := range ss {
		result[i] = fmt.Sprintf("'%s'", escapeKustoString(s))
	}
	return result
}

func formatKustoDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	if totalSeconds >= 3600 && totalSeconds%3600 == 0 {
		return fmt.Sprintf("%dh", totalSeconds/3600)
	}
	if totalSeconds >= 60 && totalSeconds%60 == 0 {
		return fmt.Sprintf("%dm", totalSeconds/60)
	}
	return fmt.Sprintf("%ds", totalSeconds)
}
