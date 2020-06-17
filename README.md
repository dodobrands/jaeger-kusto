# Azure Data Explorer (Kusto) gRPC backend for Jaeger

![master](https://github.com/dodopizza/jaeger-kusto/workflows/master/badge.svg)
![Docker Pulls](https://img.shields.io/docker/pulls/dodopizza/jaeger-kusto-collector)

This is a storage grpc-plugin for [Jaeger end-to-end distributed tracing system](https://www.jaegertracing.io/).

Currently supports version 1.18.

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
  "tenantId": ""
}
```

Save this file as `jaeger-kusto-config.json` in the root of repository.

`docker-compose up --build` will build the all-in-one Jaeger container and start it together with Hotrod test app.

Jaeger UI will be at <http://localhost:16686/>, Hotrod test app will be at <http://localhost:8080>.

You can go to Hotrod test app and generate some spans. They will appear in Kusto approx. in 5 minutes (this can be controlled with IngestionBatching policy). After the spans have been ingested, you will see that UI works.

You can check that jaeger-kusto ingestion is working with this query:

```kql
.show commands
| where Database == '<yourdatabase>' and CommandType == 'DataIngestPull'
| top 10 by StartedOn
```

For production deployment we have these charts: `dodopizza/jaeger-kusto-query` `dodopizza/jaeger-kusto-collector` `dodopizza/jaeger-kusto-agent`. You can view latest tag in docker hub - <https://hub.docker.com/r/dodopizza/jaeger-kusto/tags>
