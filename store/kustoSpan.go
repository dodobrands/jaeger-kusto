package store

import (
	"encoding/json"
	"github.com/Azure/azure-kusto-go/kusto/data/value"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/plugin/storage/es/spanstore/dbmodel"
	"strconv"
	"time"
)

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
const (
	//TagDotReplacementCharacter state which character should replace the dot in es
	TagDotReplacementCharacter = "_"
)


func TransformKustoSpanToSpan(kustoSpan *KustoSpan) (*model.Span, error){


	var refs []dbmodel.Reference
	err := json.Unmarshal(kustoSpan.References.Value, &refs)
	if err != nil {
		return nil, err
	}

	var tags map[string]interface{}
	err = json.Unmarshal(kustoSpan.Tags.Value, &tags)
	if err != nil {
		return nil, err
	}

	var logs []dbmodel.Log
	err = json.Unmarshal(kustoSpan.Logs.Value, &logs)
	if err != nil {
		return nil, err
	}

	var process dbmodel.Process

	process = dbmodel.Process{
		ServiceName: kustoSpan.ProcessServiceName,
		Tags:        nil,
		Tag: nil,
	}

	err = json.Unmarshal(kustoSpan.ProcessTags.Value, &process.Tag)
	if err != nil {
		return nil, err
	}

	jsonSpan := &dbmodel.Span{
		TraceID:         dbmodel.TraceID(kustoSpan.TraceID),
		SpanID:          dbmodel.SpanID(kustoSpan.SpanID),
		Flags:           uint32(kustoSpan.Flags),
		OperationName:   "",
		References:      refs,
		StartTime:       0,
		StartTimeMillis: 0,
		Duration:        0,
		Tags:            nil,
		Tag:             tags,
		Logs:            logs,
		Process:         process,
	}

	spanConverter := dbmodel.NewToDomain(TagDotReplacementCharacter)
	convertedSpan, err := spanConverter.SpanToDomain(jsonSpan)
	if err != nil {
		return nil, err
	}

	span := &model.Span{
		TraceID:       convertedSpan.TraceID,
		SpanID:        convertedSpan.SpanID,
		OperationName: kustoSpan.OperationName,
		References:    convertedSpan.References,
		Flags:         convertedSpan.Flags,
		StartTime:     kustoSpan.StartTime,
		Duration:      kustoSpan.Duration,
		Tags:          convertedSpan.Tags,
		Logs:          convertedSpan.Logs,
		Process:       convertedSpan.Process,
	}

	return span, err
}


func getTagsValues(tags []model.KeyValue) []string {
	var values []string
	for i := range tags {
		values = append(values, tags[i].VStr)
	}
	return values
}

// Transforms Jaeger span to CSV
func TransformSpanToCSV(span *model.Span) ([]string, error) {

	spanConverter := dbmodel.NewFromDomain(true, getTagsValues(span.Tags), TagDotReplacementCharacter)
	jsonSpan := spanConverter.FromDomainEmbedProcess(span)

	references, err := json.Marshal(jsonSpan.References)
	if err != nil {
		return nil, err
	}
	tags, err := json.Marshal(jsonSpan.Tag)
	if err != nil {
		return nil, err
	}
	logs, err := json.Marshal(jsonSpan.Logs)
	if err != nil {
		return nil, err
	}
	processTags, err := json.Marshal(jsonSpan.Process.Tag)
	if err != nil {
		return nil, err
	}

	kustoStringSpan := []string{
		span.TraceID.String(),
		span.SpanID.String(),
		span.OperationName,
		string(references),
		strconv.FormatUint(uint64(span.Flags), 10),
		span.StartTime.Format(time.RFC3339Nano),
		value.Timespan{Value: span.Duration, Valid: true}.Marshal(),
		string(tags),
		string(logs),
		span.Process.ServiceName,
		string(processTags),
		span.ProcessID,
	}


	return kustoStringSpan, err
}
