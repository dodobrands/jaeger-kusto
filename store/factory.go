package store

import (
	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
)

type kustoFactory struct {
	PluginConfig *PluginConfig
	Database     string
	Table        string
	client       *kusto.Client
}

func newKustoFactory(client *kusto.Client, pc *PluginConfig, database string) *kustoFactory {
	return &kustoFactory{
		client:       client,
		Database:     database,
		Table:        "Spans",
		PluginConfig: pc,
	}
}

func (f *kustoFactory) Reader() kustoReaderClient {
	return f.client
}

func (f *kustoFactory) Ingest() (kustoIngest, error) {
	return ingest.New(f.client, f.Database, f.Table)
}
