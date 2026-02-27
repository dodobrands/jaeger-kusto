# Copilot Instructions for jaeger-kusto

## Build, Test, and Lint

```bash
# Build
go build -v -o jaeger-kusto

# Lint
golangci-lint run

# Unit tests (all packages)
go test ./... -count=1

# Single package tests
go test ./metrics/... -count=1
go test ./store/... -count=1
go test ./config/... -count=1

# Single test
go test ./metrics/... -run TestParsePromQL_Latency -count=1

# Integration tests (requires Kusto cluster connection)
go test -v --tags=integration -timeout 300s ./test/...
```

### Docker & Docker Compose

```bash
# Build Docker image
docker build -f build/server/Dockerfile -t jaeger-kusto .

# Run locally with docker-compose (requires jaeger-kusto-config.json in repo root)
docker compose -f build/server/docker-compose.yml up --build
# Jaeger UI at http://localhost:16686, metrics shim at :9090
```

### Helm

```bash
# Deploy to Kubernetes (customize build/server/helm/values.yaml first)
helm install jaeger-kusto build/server/helm/ -n <namespace>

# Upgrade
helm upgrade jaeger-kusto build/server/helm/ -n <namespace>
```

## Architecture

This plugin exists to provide a **Jaeger UI** for traces that are collected via the **OpenTelemetry Collector** and stored in **Azure Data Explorer (Kusto)**. It is a read-only bridge — the OTEL Collector's ADX exporter handles ingestion; this plugin only queries Kusto to render traces in Jaeger. It does not collect or store metrics itself; the `metrics/` package is a PromQL translation shim that computes RED metrics on-the-fly from the same trace data in Kusto.

It implements `shared.StoragePlugin` from Jaeger's gRPC plugin framework. The `SpanWriter` is a no-op.

### Data Flow

```
Jaeger UI → Jaeger Query → gRPC → runner/ → store/ → Kusto (OTELTraces table)
                                                        ↑
Jaeger V2 Monitor tab → PromQL → metrics/ ──────────────┘
                                  (HTTP shim)   (SpanMetrics materialized view)
```

### Key Packages

- **`store/`** — Jaeger `SpanReader`/`DependencyReader` backed by Kusto. Reads query the `OTELTraces` table via KQL. The `SpanWriter` is a no-op (ingestion is handled by the OTEL exporter).
- **`runner/`** — Startup logic. Two modes: `servePlugin` (Jaeger V1 hashicorp/go-plugin over stdio) or `serveServer` (standalone gRPC server when `remoteMode: true`).
- **`config/`** — Two config files parsed by Viper: a plugin config (JSON, `--config` flag) and a Kusto config (JSON, path set in plugin config via `kustoConfigPath`). Environment variables override plugin config with prefix `JAEGER_KUSTO_PLUGIN`.
- **`metrics/`** — PromQL shim HTTP server for Jaeger V2 SPM/RED metrics. Parses Jaeger's PromQL queries (3 fixed patterns: latency histogram, call rate, error rate), translates them to KQL, and returns Prometheus-compatible JSON responses. Can query a pre-computed `SpanMetrics` materialized view or fall back to raw `OTELTraces`. Enabled via `metricsEnabled` in plugin config.

### Kusto Client Pattern

`store.NewKustoClient()` handles auth (AAD app key, managed identity, or workload identity). The `kustoFactory` provides a reader client (`kusto.Client.Query`). The same auth pattern is replicated for the metrics reader.

## Conventions

- **Logging**: `hashicorp/go-hclog` throughout — not `log` or `zap`.
- **Testing**: `stretchr/testify` for assertions (`assert`/`require`). Integration tests use `//go:build integration` tags and live in `test/`. Unit tests live alongside source files.
- **KQL queries**: Built using `kusto/kql` builder with parameterized queries (`kql.NewParameters()`). Tag filters use `AddUnsafe` for dynamic key interpolation.
- **Kusto struct mapping**: Use `kusto:"ColumnName"` struct tags for row deserialization via `row.ToStruct()`.
- **Config loading**: JSON files parsed via Viper (`viper.SetConfigType("json")`), with environment variable overrides using the `JAEGER_KUSTO_PLUGIN_` prefix.
