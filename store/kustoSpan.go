package store

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-kusto-go/kusto/data/value"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/plugin/storage/es/spanstore/dbmodel"
)

type kustoSpan struct {
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

type event struct {
	EventName       string                 `kusto:"EventName"`
	Timestamp       string                 `kusto:"Timestamp"`
	EventAttributes map[string]interface{} `kusto:"EventAttributes"`
}

const (
	// TagDotReplacementCharacter state which character should replace the dot in dynamic column
	TagDotReplacementCharacter = "_"
)

func transformKustoSpanToModelSpan(kustoSpan *kustoSpan, logger hclog.Logger) (*model.Span, error) {
	var refs []dbmodel.Reference
	err := json.Unmarshal(kustoSpan.References.Value, &refs)
	if err != nil {
		return nil, err
	}
	var tags map[string]interface{}
	err = json.Unmarshal(kustoSpan.Tags.Value, &tags)
	// Fix issues where there are JSON Array types in tags. On nested tag types convert arrays to string. Else this causes issues in span parsing in Jaeger span transformations
	for key, element := range tags {
		elementString := fmt.Sprint(element)
		isArray := len(elementString) > 0 && elementString[0] == '['
		if isArray {
			tags[key] = elementString
		}
	}
	if err != nil {
		return nil, err
	}

	var events []event
	err = json.Unmarshal(kustoSpan.Logs.Value, &events)
	if err != nil {
		logger.Warn(fmt.Sprintf("Error de-serializing data %s. The TraceId is %s and the SpanId is %s ", kustoSpan.Logs.String(), kustoSpan.TraceID, kustoSpan.SpanID))
		return nil, err
	}
	var logs []dbmodel.Log

	// Map event to logs that can be set. ref: https://opentelemetry.io/docs/reference/specification/trace/sdk_exporters/jaeger/#events
	// Set all the events' timestam and attibute, to log's timestamp and fields by iterating over span events
	for _, evt := range events {
		log := dbmodel.Log{}
		var kvs []dbmodel.KeyValue
		timestamp := evt.Timestamp
		if timestamp != "" {
			t, terr := time.Parse(time.RFC3339Nano, timestamp)
			if terr != nil {
				logger.Warn(fmt.Sprintf("Error parsing log timestamp. Error %s. The TraceId is %s and the SpanId is %s & timestamp is %s ", terr.Error(), kustoSpan.TraceID, kustoSpan.SpanID, timestamp))
			} else {
				log.Timestamp = uint64(t.UnixMicro())
			}
		}

		// EventName should be added as log's field.
		kvs = append(kvs, dbmodel.KeyValue{
			Key:   "event",
			Value: evt.EventName,
			Type:  dbmodel.StringType,
		})
		for ek, ev := range evt.EventAttributes {
			kv := dbmodel.KeyValue{
				Key:   ek,
				Value: fmt.Sprint(ev),
				Type:  dbmodel.ValueType(strings.ToLower(reflect.TypeOf(ev).String())),
			}
			kvs = append(kvs, kv)
		}
		log.Fields = kvs
		logs = append(logs, log)
	}

	process := dbmodel.Process{
		ServiceName: kustoSpan.ProcessServiceName,
		Tags:        nil,
		Tag:         nil,
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
		StartTime:       uint64(kustoSpan.StartTime.UnixMilli()),
		StartTimeMillis: uint64(kustoSpan.StartTime.UnixMilli()),
		Duration:        uint64(kustoSpan.Duration.Microseconds()),
		Tags:            nil,
		Tag:             tags,
		Logs:            logs,
		Process:         process,
	}
	spanConverter := dbmodel.NewToDomain(TagDotReplacementCharacter)
	convertedSpan, err := spanConverter.SpanToDomain(jsonSpan)
	if err != nil {
		logger.Error(fmt.Sprintf("Error parsing span to domain. Error %s. The TraceId is %s and the SpanId is %s ", err, kustoSpan.TraceID, kustoSpan.SpanID))
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

// TransformSpanToStringArray converts span to string ready for Kusto ingestion
func TransformSpanToStringArray(span *model.Span) ([]string, error) {
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
