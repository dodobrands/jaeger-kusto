package store

import (
	"errors"
	"time"

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

// NewKustoClient creates a new Kusto client from the given configuration.
// This is exported for reuse by the metrics server.
func NewKustoClient(kc *config.KustoConfig, logger hclog.Logger) (*kusto.Client, error) {
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
			logger.Info("Authenticating using AppId / Secret / TenantId", "clientId", kc.ClientID, "tenantId", kc.TenantID)
			kcsb = kusto.NewConnectionStringBuilder(kc.Endpoint).WithAadAppKey(kc.ClientID, kc.ClientSecret, kc.TenantID)
		}
	}
	kcsb.SetConnectorDetails("Kusto Jaeger", "0.0.1", "plugin", "", false, "")
	return kusto.New(kcsb)
}

// NewStore creates new Kusto store for Jaeger span storage
func NewStore(kc *config.KustoConfig, pc *config.PluginConfig, logger hclog.Logger) (shared.StoragePlugin, error) {
	client, err := NewKustoClient(kc, logger)
	if err != nil {
		return nil, err
	}

	// create factory for trace table operations
	factory := newKustoFactory(client, kc.Database, kc.TraceTableName)

	var cache *discoveryCache
	var cacheTTL time.Duration
	if pc != nil && pc.CacheDiscoveryQueries {
		var parseErr error
		cacheTTL, parseErr = time.ParseDuration(pc.CacheDiscoveryTTL)
		if parseErr != nil {
			logger.Warn("Invalid cacheDiscoveryTTL, using default 6h", "value", pc.CacheDiscoveryTTL, "error", parseErr)
			cacheTTL = 6 * time.Hour
		}
		cache = newDiscoveryCache(cacheTTL)
		logger.Info("Discovery query caching enabled", "ttl", cacheTTL)
	}

	reader, err := newKustoSpanReader(factory, logger, kc.ClientRequestOptions, cache)
	if err != nil {
		return nil, err
	}

	// Start background dependency cache refresh when caching is enabled
	if cache != nil {
		refresher := newDependencyRefresher(reader, cache, cacheTTL, logger)
		reader.depRefresher = refresher
		refresher.start()
	}

	store := &store{
		dependencyStoreReader: reader,
		reader:                reader,
		writer:                &noopSpanWriter{},
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
