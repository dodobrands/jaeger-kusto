FROM golang:1.25-alpine AS build
RUN apk --no-cache add ca-certificates

WORKDIR /build/

COPY go.mod go.sum /build/
RUN go mod download
COPY . /build/
RUN go build -o jaeger-kusto .

FROM alpine:3 AS run
RUN apk add --no-cache ca-certificates

COPY --from=build /build/jaeger-kusto /go/bin/jaeger-kusto

ENTRYPOINT [ "/go/bin/jaeger-kusto" ]
