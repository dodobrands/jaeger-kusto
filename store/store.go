package store

import (
	"errors"

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
	var kcsb *kusto.ConnectionStringBuilder
	if kc.UseManagedIdentity {
		if kc.ClientID == "" {
			logger.Info("Using system managed identity")
			kcsb = kusto.NewConnectionStringBuilder(kc.Endpoint).WithSystemManagedIdentity()
		} else {
			logger.Info("Using user managed identity")
			kcsb = kusto.NewConnectionStringBuilder(kc.Endpoint).WithUserManagedIdentity(kc.ClientID)
		}
	} else {
		if kc.UseWorkloadIdentity {
			logger.Info("Using workload identity for authentication")
			kcsb = kusto.NewConnectionStringBuilder(kc.Endpoint).WithDefaultAzureCredential()
		} else {
			if kc.ClientID == "" || kc.ClientSecret == "" || kc.TenantID == "" {
				return nil, errors.New("missing client configuration (ClientId, ClientSecret, TenantId) for kusto")
			}
			logger.Info("Authenticating using AppId [%s] / Secret / TenantId [%s]", kc.ClientID, kc.TenantID)
			kcsb = kusto.NewConnectionStringBuilder(kc.Endpoint).WithAadAppKey(kc.ClientID, kc.ClientSecret, kc.TenantID)
		}
	}
	client, err := kusto.New(kcsb)
	if err != nil {
		return nil, err
	}

	// create factory for trace table opertations
	factory := newKustoFactory(client, pc, kc.Database, kc.TraceTableName)

	reader, err := newKustoSpanReader(factory, logger, kc.ClientRequestOptions)
	if err != nil {
		return nil, err
	}

	writer, err := newKustoSpanWriter(factory, logger, pc)
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
