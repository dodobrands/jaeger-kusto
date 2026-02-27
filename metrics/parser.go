package metrics

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// QueryType identifies which RED metric a PromQL query targets.
type QueryType int

const (
	QueryTypeLatency  QueryType = iota
	QueryTypeCallRate
	QueryTypeErrorRate
)

// ParsedQuery holds the extracted parameters from a Jaeger-generated PromQL query.
type ParsedQuery struct {
	Type       QueryType
	Quantile   float64  // only for latency queries (e.g. 0.95)
	Services   []string // service_name filter values
	SpanKinds  []string // span_kind filter values
	RateWindow time.Duration
	GroupBy    []string // e.g. ["service_name"] or ["service_name","span_name"]
}

// Regex patterns for the 3 PromQL query shapes Jaeger generates.
var (
	// histogram_quantile(0.95, sum(rate(duration_milliseconds_bucket{service_name =~ "svc1|svc2", span_kind =~ "SPAN_KIND_SERVER"}[10m])) by (service_name, span_name, le))
	latencyRe = regexp.MustCompile(
		`histogram_quantile\(\s*([0-9.]+)\s*,\s*sum\(rate\((\w+)_bucket\{([^}]*)\}\[([^\]]+)\]\)\)\s*by\s*\(([^)]+)\)\)`,
	)

	// sum(rate(calls_total{service_name =~ "svc1|svc2", status_code = "STATUS_CODE_ERROR", span_kind =~ "SPAN_KIND_SERVER"}[10m])) by (service_name)
	// sum(rate(calls_total{service_name =~ "svc1|svc2", span_kind =~ "SPAN_KIND_SERVER"}[10m])) by (service_name)
	callRateRe = regexp.MustCompile(
		`sum\(rate\((\w+)\{([^}]*)\}\[([^\]]+)\]\)\)\s*by\s*\(([^)]+)\)`,
	)

	// Label matchers inside the {} block
	labelRe = regexp.MustCompile(`(\w+)\s*=~?\s*"([^"]*)"`)
)

// ParsePromQL parses a Jaeger-generated PromQL query into a ParsedQuery.
func ParsePromQL(query string) (*ParsedQuery, error) {
	query = strings.TrimSpace(query)

	// Try latency pattern first
	if m := latencyRe.FindStringSubmatch(query); m != nil {
		quantile, err := parseFloat(m[1])
		if err != nil {
			return nil, fmt.Errorf("invalid quantile %q: %w", m[1], err)
		}
		labels := parseLabels(m[3])
		rateWindow, err := parsePromDuration(m[4])
		if err != nil {
			return nil, fmt.Errorf("invalid rate window %q: %w", m[4], err)
		}
		groupBy := parseCSV(m[5])
		// Remove "le" from groupBy since it's histogram-specific
		groupBy = removeElement(groupBy, "le")

		return &ParsedQuery{
			Type:       QueryTypeLatency,
			Quantile:   quantile,
			Services:   splitPipe(labels["service_name"]),
			SpanKinds:  splitPipe(labels["span_kind"]),
			RateWindow: rateWindow,
			GroupBy:    groupBy,
		}, nil
	}

	// Try call rate / error rate pattern
	if m := callRateRe.FindStringSubmatch(query); m != nil {
		labels := parseLabels(m[2])
		rateWindow, err := parsePromDuration(m[3])
		if err != nil {
			return nil, fmt.Errorf("invalid rate window %q: %w", m[3], err)
		}
		groupBy := parseCSV(m[4])

		qtype := QueryTypeCallRate
		if _, hasStatusCode := labels["status_code"]; hasStatusCode {
			qtype = QueryTypeErrorRate
		}

		return &ParsedQuery{
			Type:       qtype,
			Services:   splitPipe(labels["service_name"]),
			SpanKinds:  splitPipe(labels["span_kind"]),
			RateWindow: rateWindow,
			GroupBy:    groupBy,
		}, nil
	}

	return nil, fmt.Errorf("unrecognized PromQL query pattern: %s", query)
}

func parseLabels(s string) map[string]string {
	labels := make(map[string]string)
	for _, m := range labelRe.FindAllStringSubmatch(s, -1) {
		labels[m[1]] = m[2]
	}
	return labels
}

func parseCSV(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func splitPipe(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "|")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func removeElement(slice []string, elem string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != elem {
			result = append(result, s)
		}
	}
	return result
}

// parsePromDuration parses a Prometheus duration like "10m", "1h", "30s".
func parsePromDuration(s string) (time.Duration, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty duration")
	}
	suffix := s[len(s)-1]
	numStr := s[:len(s)-1]
	val, err := parseFloat(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid duration number %q: %w", numStr, err)
	}
	switch suffix {
	case 's':
		return time.Duration(val * float64(time.Second)), nil
	case 'm':
		return time.Duration(val * float64(time.Minute)), nil
	case 'h':
		return time.Duration(val * float64(time.Hour)), nil
	case 'd':
		return time.Duration(val * 24 * float64(time.Hour)), nil
	default:
		return 0, fmt.Errorf("unknown duration unit %q", string(suffix))
	}
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
