package store

import (
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/storage/dependencystore"
	"github.com/jaegertracing/jaeger/storage/spanstore"
)

type Store struct {
	config KustoConfig
	reader *KustoSpanReader
	writer *KustoSpanWriter
}

func NewStore(config KustoConfig, logger hclog.Logger) *Store {

	authorizer := kusto.Authorization{
		Config: auth.NewClientCredentialsConfig(config.ClientID, config.ClientSecret, config.TenantID),
	}

	client, err := kusto.New(config.Endpoint, authorizer)
	if err != nil {
		panic("add error handling")
	}

	reader := NewKustoSpanReader(client, logger)
	writer := NewKustoSpanWriter(client, logger)
	store := &Store{
		reader: reader,
		writer: writer,
	}

	return store
}

func (store *Store) DependencyReader() dependencystore.Reader {
	return store.reader
}

func (store *Store) SpanReader() spanstore.Reader {
	return store.reader
}

func (store *Store) SpanWriter() spanstore.Writer {
	return store.writer
}
