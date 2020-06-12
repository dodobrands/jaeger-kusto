# Azure Data Explorer (Kusto) gRPC backend for Jaeger

![master](https://github.com/dodopizza/jaeger-kusto/workflows/master/badge.svg)

This is a storage grpc-plugin for [Jaeger end-to-end distributed tracing system](https://www.jaegertracing.io/).

Currently supports version 1.18.

https://www.jaegertracing.io/

## Installation

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

