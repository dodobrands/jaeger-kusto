package store

import (
	"errors"

	"github.com/jaegertracing/jaeger/storage/spanstore"
)

// taken from https://github.com/logzio/jaeger-logzio/blob/master/store/store.go
var (
	// ErrServiceNameNotSet occurs when attempting to query with an empty service name
	ErrServiceNameNotSet = errors.New("service Name must be set")

	// ErrStartTimeMinGreaterThanMax occurs when start time min is above start time max
	ErrStartTimeMinGreaterThanMax = errors.New("start Time Minimum is above Maximum")

	// ErrDurationMinGreaterThanMax occurs when duration min is above duration max
	ErrDurationMinGreaterThanMax = errors.New("duration Minimum is above Maximum")

	// ErrMalformedRequestObject occurs when a request object is nil
	ErrMalformedRequestObject = errors.New("malformed request object")

	// ErrStartAndEndTimeNotSet occurs when start time and end time are not set
	ErrStartAndEndTimeNotSet = errors.New("start and End Time must be set")
)

var (
	getTrace      = `getTrace`
	getTraceQuery = `%s | where TraceID == ParamTraceID | extend Duration=totimespan(datetime_diff('microsecond',EndTime,StartTime)) , ProcessServiceName=tostring(ResourceAttributes.['service.name']) | project-rename Tags=TraceAttributes,Logs=Events,ProcessTags=ResourceAttributes| extend References=iff(isempty(ParentID),todynamic("[]"),pack_array(bag_pack("refType","CHILD_OF","traceID",TraceID,"spanID",ParentID)))`

	getServices      = `getServices`
	getServicesQuery = `set query_results_cache_max_age = time(5m); %s 
	| extend ProcessServiceName=tostring(ResourceAttributes.['service.name']) 
	| summarize by ProcessServiceName 
	| sort by ProcessServiceName asc`

	getOpsWithNoParams      = `getOpsWithNoParams`
	getOpsWithNoParamsQuery = `%s
	| summarize count() by SpanName
	| sort by count_
	| project-away count_`

	getOpsWithParams      = `getOpsWithParams`
	getOpsWithParamsQuery = `%s | extend ProcessServiceName=tostring(ResourceAttributes.['service.name'])
	| where ProcessServiceName == ParamProcessServiceName
	| summarize count() by SpanName
	| sort by count_
	| project-away count_`

	getDependencies      = `getDependencies`
	getDependenciesQuery = `%s | extend ProcessServiceName=tostring(ResourceAttributes.['service.name'])
	| where StartTime < ParamEndTs and StartTime > (ParamEndTs-ParamLookBack)
	| project ProcessServiceName, SpanID, ChildOfSpanId = ParentID | join (%s | extend ProcessServiceName=tostring(ResourceAttributes.['service.name'])
	| project ChildOfSpanId=SpanID, ParentService=ProcessServiceName) on ChildOfSpanId | where ProcessServiceName != ParentService
	| extend Call=pack('Parent', ParentService, 'Child', ProcessServiceName) | summarize CallCount=count() by tostring(Call) | extend Call=parse_json(Call)
	| evaluate bag_unpack(Call)`

	getTraceIdBase      = `getTraceIdBase`
	getTraceIdBaseQuery = `%s | extend Duration=totimespan(datetime_diff('microsecond',EndTime,StartTime)) , ProcessServiceName=tostring(ResourceAttributes.['service.name'])`

	getTracesBase      = `getTracesBase`
	getTracesBaseQuery = `%s | extend ProcessServiceName=tostring(ResourceAttributes.['service.name']),Duration=totimespan(datetime_diff('millisecond',EndTime,StartTime))`
)

// taken from https://github.com/logzio/jaeger-logzio/blob/master/store/queryUtils.go
func validateQuery(p *spanstore.TraceQueryParameters) error {
	if p == nil {
		return ErrMalformedRequestObject
	}
	if p.ServiceName == "" && len(p.Tags) > 0 {
		return ErrServiceNameNotSet
	}
	if p.StartTimeMin.IsZero() || p.StartTimeMax.IsZero() {
		return ErrStartAndEndTimeNotSet
	}
	if p.StartTimeMax.Before(p.StartTimeMin) {
		return ErrStartTimeMinGreaterThanMax
	}
	if p.DurationMin != 0 && p.DurationMax != 0 && p.DurationMin > p.DurationMax {
		return ErrDurationMinGreaterThanMax
	}
	if p.NumTraces > 500 {
		p.NumTraces = 500
	}
	return nil
}
