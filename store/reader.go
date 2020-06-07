package store

import (
	"context"
	"errors"
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/data/table"
	"github.com/Azure/azure-kusto-go/kusto/data/types"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/spanstore"
	"time"
)

type KustoSpanReader struct {
	client *kusto.Client
}

func NewKustoSpanReader(client *kusto.Client, logger hclog.Logger) *KustoSpanReader {
	reader := &KustoSpanReader{client}
	return reader
}

func (r *KustoSpanReader) GetTrace(ctx context.Context, traceID model.TraceID) (*model.Trace, error) {

	kustoStmt := kusto.NewStmt("Spans | where TraceID == ParamTraceID").MustDefinitions(
		kusto.NewDefinitions().Must(
			kusto.ParamTypes{
				"ParamTraceID": kusto.ParamType{Type: types.String},
			},
		),).MustParameters(kusto.NewParameters().Must(kusto.QueryValues{"ParamTraceID": traceID.String()}))

	iter, err := r.client.Query(ctx, "jaeger", kustoStmt)
	if err != nil {
		panic("add error handling")
	}
	defer iter.Stop()



	var spans []*model.Span
	err = iter.Do(
		func(row *table.Row) error {
			rec := KustoSpan{}
			if err := row.ToStruct(&rec); err != nil {
				return err
			}
			var span *model.Span
			span, err = TransformKustoSpanToSpan(&rec)
			if err != nil {
				return err
			}
			spans = append(spans, span)
			return nil
		},
	)

	trace := model.Trace{Spans: spans}

	return &trace, err
}

func (r *KustoSpanReader) FindTraceIDs(ctx context.Context, query *spanstore.TraceQueryParameters) ([]model.TraceID, error) {
	return nil, errors.New("not implemented")
}

func (r *KustoSpanReader) GetServices(ctx context.Context) ([]string, error) {
	panic("implement me")
}

func (r *KustoSpanReader) GetOperations(ctx context.Context, query spanstore.OperationQueryParameters) ([]spanstore.Operation, error) {
	panic("implement me")
}

func (r *KustoSpanReader) FindTraces(ctx context.Context, query *spanstore.TraceQueryParameters) ([]*model.Trace, error) {
	panic("implement me")
}

func (r *KustoSpanReader) GetDependencies(endTs time.Time, lookback time.Duration) ([]model.DependencyLink, error) {
	panic("implement me")
}
