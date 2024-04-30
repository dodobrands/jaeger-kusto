package config

import (
	"errors"

	"github.com/Azure/azure-kusto-go/kusto"
)

// KustoConfig contains AzureAD service principal and Kusto cluster configs
type KustoConfig struct {
	ClientID             string              `json:"clientId"`
	ClientSecret         string              `json:"clientSecret"`
	TenantID             string              `json:"tenantId"`
	UseManagedIdentity   bool                `json:"useManagedIdentity,omitempty"`
	UseWorkloadIdentity  bool                `json:"useWorkloadIdentity,omitempty"`
	Endpoint             string              `json:"endpoint"`
	Database             string              `json:"database"`
	TraceTableName       string              `json:"traceTableName"`
	ClientRequestOptions []kusto.QueryOption `json:"clientRequestOptions,omitempty"`
}

// ParseKustoConfig reads file at path and returns instance of KustoConfig or error
func ParseKustoConfig(path string, requestNoTruncation bool, requestNoTimeout bool) (*KustoConfig, error) {
	c := &KustoConfig{}
	queryOptions := make([]kusto.QueryOption, 0)

	if err := load(path, c); err != nil {
		return nil, err
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}

	if requestNoTruncation {
		queryOptions = append(queryOptions, kusto.NoTruncation())

	}
	if requestNoTimeout {
		queryOptions = append(queryOptions, kusto.NoRequestTimeout())
	}

	queryOptions = append(queryOptions, kusto.Application("azure-kusto-jaeger-plugin"))
	c.ClientRequestOptions = queryOptions
	return c, nil
}

// Validate returns error if any of required fields missing
func (kc *KustoConfig) Validate() error {
	if kc.Database == "" {
		return errors.New("missing database in kusto configuration")
	}
	if kc.Endpoint == "" {
		return errors.New("missing endpoint in kusto configuration")
	}
	// If the config indicates a non ManagedIdentity or WorkloadIdentity, then the ClientID, ClientSecret, and TenantID must be provided.
	if !kc.UseManagedIdentity && !kc.UseWorkloadIdentity {
		if kc.ClientID == "" || kc.ClientSecret == "" || kc.TenantID == "" {
			return errors.New("missing client configuration (ClientId, ClientSecret, TenantId) & ManagedIdentity is missing for kusto")
		}
	}
	//if no Tracetable name provided, default to OTELTraces.
	if kc.TraceTableName == "" {
		kc.TraceTableName = "OTELTraces"
	}
	return nil
}
