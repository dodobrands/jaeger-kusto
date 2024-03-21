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
  "traceTableName":"<trace_table>" // defaults to `OTELTraces` if not provided
}
```

Save this file as `jaeger-kusto-config.json` in the root of repository.


## Local runs
Plugin can be started as a standalone app (GRPC server):

* Standalone app (as grpc server). For this mode, use `docker compose --file build/server/docker-compose.yml up --build`
Once this is done, you can run the Jaeger UI on <http://localhost:16686> and see the traces in the UI.

## Known Limitations

The plugin is in early development stage (alpha) has the following known limitations:

* Currently search by tags is not implemented
* There are deprecated API's in use. These will be fixed in a newer version of the plugin.

## Reporting issues

The logging is controlled in the `jaeger-kusto-plugin-config.json` in the build/server folder. Please change the logLevel to `debug` to get more detailed logs. This should show the executed query, please execute this query in Kusto and provide the payload as well to debug issues in the applied transformation. Attach both the logs and the payload to troubleshoot the issue.