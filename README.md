# Azure Data Explorer (Kusto) gRPC backend for Jaeger

![master](https://github.com/dodopizza/jaeger-kusto/workflows/master/badge.svg)
![Docker Pulls](https://img.shields.io/docker/pulls/dodopizza/jaeger-kusto-collector)

This is a storage grpc-plugin for [Jaeger end-to-end distributed tracing system](https://www.jaegertracing.io/).

Currently, supports version 1.31.0.

https://www.jaegertracing.io/

## Installation and testing

For local testing, you need Docker and docker-compose.

First, you have to have Azure Data Explorer cluster, here's a quickstart: <https://docs.microsoft.com/en-us/azure/data-explorer/create-cluster-database-portal>

Then create a table:

```kql
.create table Spans (
TraceID: string,
SpanID: string,
OperationName: string,
References: dynamic,
Flags: int,
StartTime: datetime,
Duration: timespan,
Tags: dynamic,
Logs: dynamic,
ProcessServiceName: string,
ProcessTags: dynamic,
ProcessID: string
)
```

Then, you should create json config file:

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

Plugin can be started in one of two modes:

* Standalone app (as grpc server). For this mode, use `docker compose --file build/server/docker-compose.yml up --build`
* Jaeger collector plugin. For this mode, use `docker compose --file build/plugin/docker-compose.yml up --build`

Any of docker-compose files will use Jaeger all-in-one container and start it together with Hotrod test app.

Jaeger UI will be at <http://localhost:16686/>, Hotrod test app will be at <http://localhost:8080>.

You can go to Hotrod test app and generate some spans. They will appear in Kusto approx. in 5 minutes (this can be controlled with IngestionBatching policy). After the spans have been ingested, you will see that UI works.

You can check that jaeger-kusto ingestion is working with this query:

```kql
.show commands
| where Database == '<yourdatabase>' and CommandType == 'DataIngestPull'
| top 10 by StartedOn
```

For production deployment we have these images: 

* [dodopizza/jaeger-kusto-query](https://hub.docker.com/r/dodopizza/jaeger-kusto-query) 
* [dodopizza/jaeger-kusto-collector](https://hub.docker.com/r/dodopizza/jaeger-kusto-collector)
* [dodopizza/jaeger-kusto-agent](https://hub.docker.com/r/dodopizza/jaeger-kusto-agent)
* [dodopizza/jaeger-kusto-plugin](https://hub.docker.com/r/dodopizza/jaeger-kusto-plugin)

You can view latest tag in docker hub
