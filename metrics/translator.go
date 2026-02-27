package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// PrometheusResponse is the top-level response envelope for /api/v1/query_range.
type PrometheusResponse struct {
	Status string         `json:"status"`
	Data   PrometheusData `json:"data"`
}

// PrometheusData contains the result type and result set.
type PrometheusData struct {
	ResultType string             `json:"resultType"`
	Result     []PrometheusResult `json:"result"`
}

// PrometheusResult represents a single time series in a matrix result.
type PrometheusResult struct {
	Metric map[string]string `json:"metric"`
	Values []SamplePair      `json:"values"`
}

// SamplePair is a [timestamp, value] tuple in Prometheus format.
type SamplePair [2]interface{}

// NewSamplePair creates a Prometheus-compatible [unixTimestamp, "stringValue"] pair.
func NewSamplePair(ts time.Time, val float64) SamplePair {
	return SamplePair{float64(ts.Unix()), formatFloat(val)}
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// MetricRow represents a single row from a Kusto RED metrics query.
type MetricRow struct {
	Timestamp   time.Time
	ServiceName string
	SpanName    string
	Value       float64
}

// TranslateToPrometheus converts Kusto metric rows into a Prometheus matrix response.
// It groups rows by their label set (service_name, optionally span_name).
func TranslateToPrometheus(rows []MetricRow, groupByOperation bool) *PrometheusResponse {
	type seriesKey struct {
		ServiceName string
		SpanName    string
	}

	// Preserve insertion order
	keyOrder := make([]seriesKey, 0)
	seriesMap := make(map[seriesKey]*PrometheusResult)

	for _, row := range rows {
		key := seriesKey{ServiceName: row.ServiceName}
		if groupByOperation {
			key.SpanName = row.SpanName
		}

		result, exists := seriesMap[key]
		if !exists {
			labels := map[string]string{
				"service_name": row.ServiceName,
			}
			if groupByOperation {
				labels["span_name"] = row.SpanName
			}
			result = &PrometheusResult{
				Metric: labels,
				Values: make([]SamplePair, 0),
			}
			seriesMap[key] = result
			keyOrder = append(keyOrder, key)
		}
		result.Values = append(result.Values, NewSamplePair(row.Timestamp, row.Value))
	}

	results := make([]PrometheusResult, 0, len(keyOrder))
	for _, key := range keyOrder {
		results = append(results, *seriesMap[key])
	}

	return &PrometheusResponse{
		Status: "success",
		Data: PrometheusData{
			ResultType: "matrix",
			Result:     results,
		},
	}
}

// WritePrometheusResponse writes a PrometheusResponse as JSON to the HTTP response.
func WritePrometheusResponse(w http.ResponseWriter, resp *PrometheusResponse) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

// WritePrometheusError writes an error response in Prometheus API format.
func WritePrometheusError(w http.ResponseWriter, statusCode int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := map[string]interface{}{
		"status":    "error",
		"errorType": errType,
		"error":     message,
	}
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// ParseQueryRangeRequest extracts query, start, end, and step from a /api/v1/query_range request.
type QueryRangeRequest struct {
	Query string
	Start time.Time
	End   time.Time
	Step  time.Duration
}

// ParseQueryRangeForm parses form values from a query_range request.
func ParseQueryRangeForm(r *http.Request) (*QueryRangeRequest, error) {
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			return nil, fmt.Errorf("failed to parse form: %w", err)
		}
	}

	query := r.FormValue("query")
	if query == "" {
		body, _ := io.ReadAll(r.Body)
		return nil, fmt.Errorf("missing 'query' parameter, body: %s", string(body))
	}

	startStr := r.FormValue("start")
	endStr := r.FormValue("end")
	stepStr := r.FormValue("step")

	start, err := parsePrometheusTime(startStr)
	if err != nil {
		return nil, fmt.Errorf("invalid 'start' %q: %w", startStr, err)
	}
	end, err := parsePrometheusTime(endStr)
	if err != nil {
		return nil, fmt.Errorf("invalid 'end' %q: %w", endStr, err)
	}
	step, err := parseStepDuration(stepStr)
	if err != nil {
		return nil, fmt.Errorf("invalid 'step' %q: %w", stepStr, err)
	}

	return &QueryRangeRequest{
		Query: query,
		Start: start,
		End:   end,
		Step:  step,
	}, nil
}

// parsePrometheusTime parses a Prometheus timestamp (unix seconds, possibly fractional).
func parsePrometheusTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		// Try RFC3339
		t, terr := time.Parse(time.RFC3339, s)
		if terr != nil {
			return time.Time{}, fmt.Errorf("cannot parse %q as float or RFC3339", s)
		}
		return t, nil
	}
	sec := int64(f)
	nsec := int64((f - float64(sec)) * 1e9)
	return time.Unix(sec, nsec).UTC(), nil
}

// parseStepDuration parses step as either seconds (float) or a Prometheus duration string.
func parseStepDuration(s string) (time.Duration, error) {
	if s == "" {
		return 60 * time.Second, nil // default 60s
	}
	// Try as float seconds first
	f, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return time.Duration(f * float64(time.Second)), nil
	}
	// Try as Prometheus duration
	return parsePromDuration(s)
}
