package config

import (
	"errors"
)

// KustoConfig contains AzureAD service principal and Kusto cluster configs
type KustoConfig struct {
	ClientID       string `json:"clientId"`
	ClientSecret   string `json:"clientSecret"`
	TenantID       string `json:"tenantId"`
	Endpoint       string `json:"endpoint"`
	Database       string `json:"database"`
	TraceTableName string `json:"traceTable"`
}

// ParseKustoConfig reads file at path and returns instance of KustoConfig or error
func ParseKustoConfig(path string) (*KustoConfig, error) {
	c := &KustoConfig{}

	if err := load(path, c); err != nil {
		return nil, err
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}

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
	if kc.ClientID == "" || kc.ClientSecret == "" || kc.TenantID == "" {
		return errors.New("missing client configuration (ClientId, ClientSecret, TenantId) for kusto")
	}
	return nil
}
