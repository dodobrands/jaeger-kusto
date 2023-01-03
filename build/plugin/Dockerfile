FROM jaegertracing/all-in-one:1.40 as jaeger
FROM golang:1.19-alpine as build
RUN apk --no-cache add ca-certificates

WORKDIR /build/

COPY go.mod go.sum /build/
RUN go mod download
COPY . /build
RUN go build .

FROM alpine:latest as run
RUN apk add --update --no-cache ca-certificates

COPY --from=jaeger /go/bin/all-in-one-linux /go/bin/all-in-one-linux
COPY --from=build /build/jaeger-kusto /go/bin/jaeger-kusto

ENV SPAN_STORAGE_TYPE grpc-plugin
ENV GRPC_STORAGE_PLUGIN_BINARY "/go/bin/jaeger-kusto"

ENTRYPOINT [ "/go/bin/all-in-one-linux" ]
