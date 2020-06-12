package store

import (
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/storage/dependencystore"
	"github.com/jaegertracing/jaeger/storage/spanstore"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
)

type store struct {
	reader *KustoSpanReader
	writer *KustoSpanWriter
}

// NewStore creates new Kusto store for Jaeger span storage
func NewStore(config KustoConfig, logger hclog.Logger) shared.StoragePlugin {

	authorizer := kusto.Authorization{
		Config: auth.NewClientCredentialsConfig(config.ClientID, config.ClientSecret, config.TenantID),
	}

	client, err := kusto.New(config.Endpoint, authorizer)
	if err != nil {
		logger.Error("Error creating Kusto client", err.Error())
		panic("cant create Kusto client")
	}

	reader := NewKustoSpanReader(client, logger, config.Database)
	writer := NewKustoSpanWriter(client, logger, config.Database)
	store := &store{
		reader: reader,
		writer: writer,
	}

	return store
}

func (store *store) DependencyReader() dependencystore.Reader {
	return store.reader
}

func (store *store) SpanReader() spanstore.Reader {
	return store.reader
}

func (store *store) SpanWriter() spanstore.Writer {
	return store.writer
}
