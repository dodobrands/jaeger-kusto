# Azure Data Explorer (Kusto) gRPC backend for Jaeger

![master](https://github.com/dodopizza/jaeger-kusto/workflows/master/badge.svg)

This is a storage grpc-plugin for [Jaeger end-to-end distributed tracing system](https://www.jaegertracing.io/).

Currently supports version 1.18.

https://www.jaegertracing.io/

## Installation and testing

For local testing, you need Docker and docker-compose.

First, you have to create a table:

```
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

Then, you should create json file with config settings:

```json
{
  "clientId": "",
  "clientSecret": "",
  "database": "jaeger",
  "endpoint": "https://<cluster>.<region>.kusto.windows.net",
  "tenantId": ""
}
```

Save this file as `jaeger-kusto-config.json` in the root of repository.

`docker-compose up --build` will build the all-in-one Jaeger container and start it together with Hotrod test app.

Jaeger UI will be at http://localhost:16686/, Hotrod test app will be at http://localhost:8080.

You can go to Hotrod test app and generate some spans. They will appear in Kusto approx. in 5 minutes (this can be controlled with IngestionBatching policy)

You can check that jaeger-kusto ingestion is working with this query:

```kql
.show commands
| where Database == '<yourdatabase>' and CommandType == 'DataIngestPull'
| top 10 by StartedOn
```
