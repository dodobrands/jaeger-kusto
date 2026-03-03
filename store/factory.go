package store

import (
	"github.com/Azure/azure-kusto-go/azkustodata"
)

type kustoFactory struct {
	Database string
	Table    string
	client   *azkustodata.Client
}

func newKustoFactory(client *azkustodata.Client, database string, table string) *kustoFactory {
	return &kustoFactory{
		client:   client,
		Database: database,
		Table:    table,
	}
}

func (f *kustoFactory) Reader() kustoReaderClient {
	return f.client
}
