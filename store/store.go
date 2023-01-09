package store

import (
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/dodopizza/jaeger-kusto/config"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
	"github.com/jaegertracing/jaeger/storage/dependencystore"
	"github.com/jaegertracing/jaeger/storage/spanstore"
)

type store struct {
	dependencyStoreReader dependencystore.Reader
	reader                spanstore.Reader
	writer                spanstore.Writer
}

// NewStore creates new Kusto store for Jaeger span storage
func NewStore(pc *config.PluginConfig, kc *config.KustoConfig, logger hclog.Logger) (shared.StoragePlugin, error) {

	kcsb := kusto.NewConnectionStringBuilder(kc.Endpoint).WithAadAppKey(kc.ClientID, kc.ClientSecret, kc.TenantID)
	client, err := kusto.New(kcsb)
	if err != nil {
		return nil, err
	}

	factory := newKustoFactory(client, pc, kc.Database)

	reader, err := newKustoSpanReader(factory, logger)
	if err != nil {
		return nil, err
	}

	writer, err := newKustoSpanWriter(factory, logger)
	if err != nil {
		return nil, err
	}

	store := &store{
		dependencyStoreReader: reader,
		reader:                reader,
		writer:                writer,
	}

	return store, nil
}

// DependencyReader returns implementation of dependencystore.Reader interface
func (store *store) DependencyReader() dependencystore.Reader {
	return store.dependencyStoreReader
}

// SpanReader returns implementation of spanstore.Reader interface
func (store *store) SpanReader() spanstore.Reader {
	return store.reader
}

// SpanWriter returns implementation of spanstore.Writer interface
func (store *store) SpanWriter() spanstore.Writer {
	return store.writer
}
