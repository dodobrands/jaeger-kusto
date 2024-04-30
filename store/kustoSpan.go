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
	SpanName           string        `kusto:"SpanName"`
	References         value.Dynamic `kusto:"References"`
	Flags              int32         `kusto:"Flags"`
	StartTime          time.Time     `kusto:"StartTime"`
	Duration           int64         `kusto:"Duration"`
	Tags               value.Dynamic `kusto:"Tags"`
	Logs               value.Dynamic `kusto:"Logs"`
	Links              []link        `kusto:"Links"`
	ProcessServiceName string        `kusto:"ProcessServiceName"`
	ProcessTags        value.Dynamic `kusto:"ProcessTags"`
	ProcessID          string        `kusto:"ProcessID"`
	SpanKind           string        `kusto:"SpanKind"`
	SpanStatus         string        `kusto:"SpanStatus"`
}

type link struct {
	TraceID            dbmodel.TraceID `json:"TraceID"`
	SpanID             dbmodel.SpanID  `json:"SpanID"`
	RefType            string          `kusto:"RefType"`
	TraceState         string          `kusto:"TraceState,omitempty"`
	SpanLinkAttributes value.Dynamic   `kusto:"SpanLinkAttributes,omitempty"`
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
	// eMin":"datetime(2024-03-13T15:56:28.628Z)"}: EXTRA_VALUE_AT_END=<nil> @module=jaeger-kusto timestamp=2024-03-15T15:56:28.634Z
	//2024-03-15T15:56:29.206Z [ERROR] jaeger-kusto: Error parsing span to domain. Error not a valid SpanRefType string . The TraceId is d1b06c73d963045e657158dbd0ccf6d9 and the SpanId is cfb683d327e4dd90 : @module=jaeger-kusto timestamp=2024-03-15T15:56:29.205Z
	//
	spanReferences, err := transformReferencesToLinks(kustoSpan, logger)
	if err != nil {
		logger.Error(fmt.Sprintf("Error in Unmarshal Refs %s. TraceId: %s  SpanId: %s ", kustoSpan.Tags.String(), kustoSpan.TraceID, kustoSpan.SpanID), err)
		return nil, err
	}

	var tags map[string]interface{}
	err = json.Unmarshal(kustoSpan.Tags.Value, &tags)
	if err != nil {
		logger.Error(fmt.Sprintf("Error in Unmarshal tags %s. TraceId: %s  SpanId: %s ", kustoSpan.Tags.String(), kustoSpan.TraceID, kustoSpan.SpanID), err)
		return nil, err
	}
	// Fix issues where there are JSON Array types in tags. On nested tag types convert arrays to string. Else this causes issues in span parsing in Jaeger span transformations
	for key, element := range tags {
		elementString := fmt.Sprint(element)
		isArray := len(elementString) > 0 && elementString[0] == '['
		if isArray {
			tags[key] = elementString
		}
	}

	// https://opentelemetry.io/docs/specs/otel/trace/sdk_exporters/jaeger/#status
	switch kustoSpan.SpanStatus {
	case "STATUS_CODE_ERROR":
		tags["otel.status_code"] = "ERROR"
		tags["error"] = true
	case "STATUS_CODE_OK":
		tags["otel.status_code"] = "OK"
	default:
		break
	}

	// https://opentelemetry.io/docs/specs/otel/trace/sdk_exporters/jaeger/#spankind
	switch kustoSpan.SpanKind {
	case "SPAN_KIND_SERVER":
		tags["span.kind"] = "server"
	case "SPAN_KIND_CLIENT":
		tags["span.kind"] = "client"
	case "SPAN_KIND_CONSUMER":
		tags["span.kind"] = "consumer"
	case "SPAN_KIND_PRODUCER":
		tags["span.kind"] = "producer"
	default:
		break
	}

	logs, err := transformEventsToLogs(kustoSpan, logger)
	if err != nil {
		logger.Error(fmt.Sprintf("Error in transform (transformEventsToLogs) %s. TraceId: %s  SpanId: %s ", kustoSpan.Tags.String(), kustoSpan.TraceID, kustoSpan.SpanID), err)
		return nil, err
	}

	process := dbmodel.Process{
		ServiceName: kustoSpan.ProcessServiceName,
		Tags:        nil,
		Tag:         nil,
	}

	escapeProcessTags(kustoSpan.ProcessTags.Value)
	// Replace the special chars(including start and end []) for correct JSON parsing
	replacer := strings.NewReplacer(":[", ":\"[", "],", "]\",", "\\", "")
	processTag := []byte(replacer.Replace(string(kustoSpan.ProcessTags.Value)))
	err = json.Unmarshal(processTag, &process.Tag)
	// See if this parsing yielded an error ?
	if err != nil {
		logger.Error(fmt.Sprintf("ERROR in Unmarshal processTags %s. TraceId: %s SpanId: %s ", string(kustoSpan.ProcessTags.Value), kustoSpan.TraceID, kustoSpan.SpanID), err)
		return nil, err
	}

	jsonSpan := &dbmodel.Span{
		TraceID:         dbmodel.TraceID(kustoSpan.TraceID),
		SpanID:          dbmodel.SpanID(kustoSpan.SpanID),
		Flags:           uint32(kustoSpan.Flags),
		OperationName:   kustoSpan.SpanName,
		References:      spanReferences,
		StartTime:       uint64(kustoSpan.StartTime.UnixMicro()),
		StartTimeMillis: uint64(kustoSpan.StartTime.UnixMilli()),
		Duration:        uint64(kustoSpan.Duration),
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
		OperationName: kustoSpan.SpanName,
		References:    convertedSpan.References,
		Flags:         convertedSpan.Flags,
		StartTime:     kustoSpan.StartTime,
		Duration:      time.Duration(kustoSpan.Duration) * time.Microsecond,
		Tags:          convertedSpan.Tags,
		Logs:          convertedSpan.Logs,
		Process:       convertedSpan.Process,
	}
	return span, err
}

func transformReferencesToLinks(kustoSpan *kustoSpan, logger hclog.Logger) ([]dbmodel.Reference, error) {
	// There are 2 parts in the links. The first one is the CHILD_OF hierarchy and the second one is the FOLLOWS_FROM hierarchy
	// Ref : https://opentelemetry.io/docs/specs/otel/trace/sdk_exporters/jaeger/#links
	// Note that we can convert SpanLinkAttributes to logs too. But this is not added at the moment
	var childOfRefs []dbmodel.Reference
	referenceValue := kustoSpan.References.Value
	if len(referenceValue) > 0 {
		err := json.Unmarshal(referenceValue, &childOfRefs)
		if err != nil {
			logger.Error(fmt.Sprintf("Error in Unmarshal CO refs %s. TraceId: %s SpanId: %s. References: %s",
				kustoSpan.References.String(), kustoSpan.TraceID, kustoSpan.SpanID, kustoSpan.References.Value), err)
			return nil, err
		}
	}

	var followsFromRefs []dbmodel.Reference
	for _, ref := range kustoSpan.Links {
		if ref.TraceID == "" || ref.SpanID == "" { // Skip the empty references
			logger.Warn(fmt.Sprintf("Empty link TraceID or SpanID for RefType %s . TraceId: %s SpanId: %s",
				ref.RefType, kustoSpan.TraceID, kustoSpan.SpanID))
		} else {
			followsFromRefs = append(followsFromRefs, dbmodel.Reference{
				RefType: dbmodel.FollowsFrom,
				TraceID: ref.TraceID,
				SpanID:  ref.SpanID,
			})
		}
	}
	// Combine the childOfRefs and followsFromRefs
	spanRefs := append(childOfRefs, followsFromRefs...)
	return spanRefs, nil
}

// Ref : https://opentelemetry.io/docs/specs/otel/trace/sdk_exporters/jaeger/#events
func transformEventsToLogs(kustoSpan *kustoSpan, logger hclog.Logger) ([]dbmodel.Log, error) {
	var events []event
	err := json.Unmarshal(kustoSpan.Logs.Value, &events)
	if err != nil {
		return nil, err
	}
	// Get the events field from events and convert it to logs
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
				logger.Warn(fmt.Sprintf("Error parsing log timestamp. Error %s. TraceId: %s SpanId: %s & timestamp: %s ", terr.Error(), kustoSpan.TraceID, kustoSpan.SpanID, timestamp))
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
	return logs, nil
}

// escapeProcessTags replaces the double quotes with single quotes in the process tags list
func escapeProcessTags(processTagsString []byte) {
	var insideSquareBrackets bool
	for i := 0; i < len(processTagsString); i++ {
		if processTagsString[i] == '[' {
			insideSquareBrackets = true
		} else if processTagsString[i] == ']' {
			insideSquareBrackets = false
		} else if insideSquareBrackets && processTagsString[i] == '"' {
			processTagsString[i] = '\''
		}
	}
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
