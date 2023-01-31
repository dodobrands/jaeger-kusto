package store

import (
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/dodopizza/jaeger-kusto/config"
)

type kustoFactory struct {
	PluginConfig *config.PluginConfig
	Database     string
	Table        string
	client       *kusto.Client
}

func newKustoFactory(client *kusto.Client, pc *config.PluginConfig, database string, table string) *kustoFactory {
	return &kustoFactory{
		client:       client,
		Database:     database,
		Table:        table,
		PluginConfig: pc,
	}
}

func (f *kustoFactory) Reader() kustoReaderClient {
	return f.client
}

func (f *kustoFactory) Ingest() (kustoIngest, error) {
	return ingest.New(f.client, f.Database, f.Table)
}
