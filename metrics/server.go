package metrics

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/go-hclog"
)

// ServerConfig holds configuration for the PromQL shim HTTP server.
type ServerConfig struct {
	ListenAddress string
	Reader        *KustoMetricsReader
	Logger        hclog.Logger
}

// Server implements a Prometheus-compatible /api/v1/query_range endpoint
// backed by Kusto queries.
type Server struct {
	reader *KustoMetricsReader
	logger hclog.Logger
	mux    *http.ServeMux
	server *http.Server
}

// NewServer creates a new PromQL shim server.
func NewServer(cfg ServerConfig) *Server {
	s := &Server{
		reader: cfg.Reader,
		logger: cfg.Logger,
		mux:    http.NewServeMux(),
	}
	s.mux.HandleFunc("GET /api/v1/query_range", s.handleQueryRange)
	s.mux.HandleFunc("POST /api/v1/query_range", s.handleQueryRange)
	// Prometheus client also calls these for health/metadata; return empty but valid responses
	s.mux.HandleFunc("GET /api/v1/status/buildinfo", s.handleBuildInfo)
	return s
}

// Handler returns the HTTP handler for the server (useful for testing).
func (s *Server) Handler() http.Handler {
	return s.mux
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(addr string) error {
	s.logger.Info("starting PromQL shim server", "address", addr)
	s.server = &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// handleQueryRange implements POST/GET /api/v1/query_range.
func (s *Server) handleQueryRange(w http.ResponseWriter, r *http.Request) {
	req, err := ParseQueryRangeForm(r)
	if err != nil {
		s.logger.Warn("failed to parse query_range request", "error", err)
		WritePrometheusError(w, http.StatusBadRequest, "bad_data", err.Error())
		return
	}

	s.logger.Debug("query_range request", "query", req.Query, "start", req.Start, "end", req.End, "step", req.Step)

	parsed, err := ParsePromQL(req.Query)
	if err != nil {
		s.logger.Warn("failed to parse PromQL query", "error", err, "query", req.Query)
		WritePrometheusError(w, http.StatusBadRequest, "bad_data", fmt.Sprintf("unsupported query: %v", err))
		return
	}

	groupByOp := containsSpanName(parsed.GroupBy)
	var rows []MetricRow

	switch parsed.Type {
	case QueryTypeCallRate:
		rows, err = s.reader.QueryCallRates(r.Context(), parsed, req.Start, req.End, req.Step)
	case QueryTypeErrorRate:
		rows, err = s.reader.QueryErrorRates(r.Context(), parsed, req.Start, req.End, req.Step)
	case QueryTypeLatency:
		rows, err = s.reader.QueryLatencies(r.Context(), parsed, req.Start, req.End, req.Step)
	default:
		WritePrometheusError(w, http.StatusBadRequest, "bad_data", "unknown query type")
		return
	}

	if err != nil {
		s.logger.Error("metrics query failed", "error", err, "type", parsed.Type)
		WritePrometheusError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	resp := TranslateToPrometheus(rows, groupByOp)
	WritePrometheusResponse(w, resp)
}

// handleBuildInfo returns a minimal buildinfo response (required by Prometheus client).
func (s *Server) handleBuildInfo(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"success","data":{"version":"kusto-promql-shim","revision":"","branch":"","buildUser":"","buildDate":"","goVersion":""}}`)) //nolint:errcheck
}
