package store

import (
	"github.com/Azure/azure-kusto-go/kusto"
)

type kustoFactory struct {
	Database string
	Table    string
	client   *kusto.Client
}

func newKustoFactory(client *kusto.Client, database string, table string) *kustoFactory {
	return &kustoFactory{
		client:   client,
		Database: database,
		Table:    table,
	}
}

func (f *kustoFactory) Reader() kustoReaderClient {
	return f.client
}
