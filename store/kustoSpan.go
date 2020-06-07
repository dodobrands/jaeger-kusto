package store

import (
	"bytes"
	"encoding/json"
	"github.com/Azure/azure-kusto-go/kusto/data/value"
	"github.com/jaegertracing/jaeger/model"
	"github.com/tushar2708/altcsv"
	"io"
	"strconv"
	"strings"
	"time"
)

type ToDomain struct {
	tagDotReplacement string
}

// ReplaceDot replaces dot with dotReplacement
func (td ToDomain) ReplaceDot(k string) string {
	return strings.Replace(k, ".", td.tagDotReplacement, -1)
}

// ReplaceDotReplacement replaces dotReplacement with dot
func (td ToDomain) ReplaceDotReplacement(k string) string {
	return strings.Replace(k, td.tagDotReplacement, ".", -1)
}

type KustoSpan struct {
	TraceID            string        `kusto:"TraceID"`
	SpanID             string        `kusto:"SpanID"`
	OperationName      string        `kusto:"OperationName"`
	References         value.Dynamic `kusto:"References"`
	Flags              int32         `kusto:"Flags"`
	StartTime          time.Time     `kusto:"StartTime"`
	Duration           time.Duration `kusto:"Duration"`
	Tags               value.Dynamic `kusto:"Tags"`
	Logs               value.Dynamic `kusto:"Logs"`
	ProcessServiceName string        `kusto:"ProcessServiceName"`
	ProcessTags        value.Dynamic `kusto:"ProcessTags"`
	ProcessID          string        `kusto:"ProcessID"`
}


func TransformKustoSpanToSpan(kustoSpan *KustoSpan) (*model.Span, error){

	refs := []model.SpanRef{}
	err := json.Unmarshal(kustoSpan.References.Value, &refs)
	if err != nil {
		return nil, err
	}

	tags := []model.KeyValue{}
	err = json.Unmarshal(kustoSpan.Tags.Value, &tags)
	if err != nil {
		return nil, err
	}

	logs := []model.Log{}
	err = json.Unmarshal(kustoSpan.Logs.Value, &logs)
	if err != nil {
		return nil, err
	}

	processTags := []model.KeyValue{}
	err = json.Unmarshal(kustoSpan.ProcessTags.Value, &processTags)
	if err != nil {
		return nil, err
	}

	process := &model.Process{
		ServiceName: kustoSpan.ProcessServiceName,
		Tags: processTags,
	}

	traceID, err := model.TraceIDFromString(string(kustoSpan.TraceID))
	if err != nil {
		return nil, err
	}

	spanIDInt, err := model.SpanIDFromString(string(kustoSpan.SpanID))
	if err != nil {
		return nil, err
	}

	span := &model.Span{
		TraceID:       traceID,
		SpanID:        model.NewSpanID(uint64(spanIDInt)),
		OperationName: kustoSpan.OperationName,
		References:    refs,
		Flags:         model.Flags(uint32(kustoSpan.Flags)),
		StartTime:     kustoSpan.StartTime,
		Duration:      kustoSpan.Duration,
		Tags:          tags,
		Logs:          logs,
		Process:       process,
	}

	return span, err
}

// Transforms Jaeger span to CSV
func TransformSpanToCSV(span *model.Span) (io.Reader, error) {

	references, err := json.Marshal(span.References)
	if err != nil {
		return nil, err
	}
	tags, err := json.Marshal(span.Tags)
	if err != nil {
		return nil, err
	}
	logs, err := json.Marshal(span.Logs)
	if err != nil {
		return nil, err
	}
	processTags, err := json.Marshal(span.Process.Tags)
	if err != nil {
		return nil, err
	}

	kustoStringSpan := []string{
		span.TraceID.String(),
		span.SpanID.String(),
		span.OperationName,
		string(references),
		strconv.FormatUint(uint64(span.Flags), 10),
		span.StartTime.Format(time.RFC3339),
		value.Timespan{Value: span.Duration, Valid: true}.Marshal(),
		string(tags),
		string(logs),
		span.Process.ServiceName,
		string(processTags),
		span.ProcessID,
	}

	b := &bytes.Buffer{}
	writer := altcsv.NewWriter(b)
	writer.AllQuotes = true
	err = writer.Write(kustoStringSpan)
	writer.Flush()
	err = writer.Error()

	return b, err
}
