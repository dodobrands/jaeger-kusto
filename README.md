# Azure Data Explorer (Kusto) gRPC backend for Jaeger


This is a storage grpc-plugin for [Jaeger end-to-end distributed tracing system](https://www.jaegertracing.io/) and was originally forked from https://github.com/dodopizza/jaeger-kusto and extended now to support OTEL exporter used with ADX.



## Installation and testing

For local testing, you need Docker and docker-compose.

First, you have to have Azure Data Explorer cluster, here's a quickstart: <https://docs.microsoft.com/en-us/azure/data-explorer/create-cluster-database-portal>

Then, the setup needed for Kusto/ADX exporter with the tables required for storing OTEL traces data can be set up as explained in the documentation [here](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/azuredataexplorerexporter/README.md).

The plugin can query OTELTraces table and provide trace UI details on Jaeger


## Authentication
Extending the authentication table provided in the Jaeger plugin, the application uses a similar config file to render Jaeger traces as well.
```json
{
  "clientId": "",
  "clientSecret": "",
  "database": "<database>",
  "endpoint": "https://<cluster>.<region>.kusto.windows.net",
  "tenantId": "",
  "traceTableName":"<trace_table>",// defaults to `OTELTraces` if not provided
  "useManagedIdentity": false, // defaults to false, if true, the plugin will use managed identity to authenticate. Use the clientId field to pass the clientId of the managed identity
  "useWorkloadIdentity": false // defaults to false, if true, the plugin will use WorkloadIdentity to authenticate. Note that the plugin will use the default credentials of the VM/Container to authenticate, it will first look for Azure environment variables to authenticate, followed by the workload identity
}
```

Save this file as `jaeger-kusto-config.json` in the root of repository.


## Local runs
Plugin can be started as a standalone app (GRPC server):

* Standalone app (as grpc server). For this mode, use `docker compose --file build/server/docker-compose.yml up --build`
Once this is done, you can run the Jaeger UI on <http://localhost:16686> and see the traces in the UI.


# Deploying to Kubernetes

The plugin and Jaeger can be deployed to Kubernetes using the provided Helm chart. The Helm chart is available in the `build/server/helm` folder. The properties can be customized through values.yaml file.

The list of properties that can be customized are:

```yaml
baseConfig:
  logLevel: 
  logJson: 
  readNoTruncation: 
  readNoTimeout:
authConfig:
  clientId: 
  useManagedIdentity: 
  database: 
  clusterUrl: 
  tenantId: 
  traceTableName: 
```


table of yaml properties:
 
| Property | Description | Default |
| --- | --- | --- |
logLevel | Log level for the plugin | info |
logJson | Log format | false |
readNoTruncation | In case [KustoQueryLimits](aka.ms/kustoquerylimits) are hit, use this property to enable no-truncation | false |
readNoTimeout | The default query timeout is 10 minutes which should be sufficient for most cases. In case this needs to be extended to no-timeout | false |
clientId | Client ID for the plugin, represents the ClientId in case of ManagedIdentity. Set it to the AAD APP Id to use AAD Auth | "" |
clientSecret | If AAD Auth is used, set this to the AAD APP Secret for the APP Id| "" |
tenantId | The AAD tenant to use for authentication | "" |
useManagedIdentity | Use managed identity for authentication (Keyless , recommended) | false |
useWorkloadIdentity | Use Azure default credentials (uses workload identity in case it is defined) for authentication | false |
database | Database name to query the traces | "" |
clusterUrl | Cluster URL where the OTEL traces have been ingested | "" |
traceTableName | Trace table name to query | "OTELTraces" |
image.repository | The repository to pull the kusto-jaeger plugin | e.g. agramachandran/jaeger-kusto |
image.tag | The tag of kusto-jaeger-plugin to use  | e.g. "1.1.0-Preview" |
image.pullPolicy | Image pull policy | "IfNotPresent" |




## Known Limitations

The plugin is in early development stage (alpha) has the following known limitations:

* Currently search by tags is not implemented
* There are deprecated API's in use. These will be fixed in a newer version of the plugin.


## RED Metrics / Service Performance Monitoring (SPM)

The plugin supports [Jaeger's SPM (Service Performance Monitoring)](https://www.jaegertracing.io/docs/2.dev/architecture/spm/) feature, which surfaces RED metrics (Rate, Errors, Duration) in the Jaeger UI "Monitor" tab.

### Architecture

Instead of requiring Prometheus or a separate metrics pipeline, this plugin computes RED metrics directly from your OTELTraces data in Kusto using a **Materialized View** for pre-aggregation, and exposes them via a **PromQL-compatible HTTP API** (shim) that Jaeger V2's built-in prometheus metric backend can query.

```
OTELTraces table → SpanMetrics Materialized View → PromQL Shim (built-in) → Jaeger V2
```

### Setup

#### 1. Create the Kusto Materialized View

Run the KQL script in `config/kusto-materialized-view.kql` against your Kusto database:

```kql
.create-or-alter async materialized-view with (backfill=true) SpanMetrics on table OTELTraces
{
    OTELTraces
    | extend
        ServiceName = tostring(ResourceAttributes.['service.name']),
        Duration_ms = datetime_diff('millisecond', EndTime, StartTime),
        StatusCode = tostring(SpanStatus)
    | summarize
        call_count = count(),
        error_count = countif(StatusCode == 'STATUS_CODE_ERROR'),
        duration_sum_ms = sum(Duration_ms),
        p50_ms = percentile(Duration_ms, 50),
        p75_ms = percentile(Duration_ms, 75),
        p95_ms = percentile(Duration_ms, 95),
        p99_ms = percentile(Duration_ms, 99)
      by ServiceName, SpanName, SpanKind, bin(StartTime, 1m)
}
```

#### 2. Configure the Plugin

Add these fields to your plugin configuration JSON:

```json
{
    "metricsEnabled": true,
    "metricsListenAddress": ":9090"
}
```

And add the materialized view name to your Kusto configuration JSON:

```json
{
    "metricsViewName": "SpanMetrics"
}
```

If `metricsViewName` is empty, the plugin will query the raw `OTELTraces` table directly (slower for large datasets but requires no materialized view setup).

A full example is at `build/server/jaeger-kusto-plugin-config.json`.

#### 3. Configure Jaeger V2

Use the sample configuration at `config/jaeger-v2-config.yaml`:

```yaml
extensions:
  jaeger_storage:
    backends:
      kusto_traces:
        grpc:
          endpoint: "jaeger-kusto:8989"
    metric_backends:
      kusto_metrics:
        prometheus:
          endpoint: "http://jaeger-kusto:9090"
  jaeger_query:
    storage:
      traces: kusto_traces
      metrics: kusto_metrics
    ui:
      config_file: /etc/jaeger/jaeger-ui.json
```

Ensure `jaeger-ui.json` has the Monitor tab enabled:

```json
{
  "monitor": {
    "menuEnabled": true
  }
}
```

### Configuration Reference

| Property | Description | Default |
| --- | --- | --- |
| `metricsEnabled` | Enable the PromQL shim server for RED metrics | `false` |
| `metricsListenAddress` | Listen address for the PromQL shim HTTP server | `":9090"` |
| `metricsViewName` | Name of the Kusto materialized view for pre-computed metrics | `""` (uses raw trace table) |


## Reporting issues

The logging is controlled in the `jaeger-kusto-plugin-config.json` in the build/server folder. Please change the logLevel to `debug` to get more detailed logs. This should show the executed query, please execute this query in Kusto and provide the payload as well to debug issues in the applied transformation. Attach both the logs and the payload to troubleshoot the issue.