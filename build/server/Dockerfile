FROM golang:1.20.1-alpine as build
RUN apk --no-cache add ca-certificates

WORKDIR /build/

COPY go.mod go.sum /build/
RUN go mod download
COPY . /build
RUN go build .

FROM alpine:latest as run
RUN apk add --update --no-cache ca-certificates

COPY --from=build /build/jaeger-kusto /go/bin/jaeger-kusto

ENTRYPOINT [ "/go/bin/jaeger-kusto" ]
