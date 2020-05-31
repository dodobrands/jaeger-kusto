package store

import (
	"github.com/spf13/viper"
	"path"
	"strings"
)

const (
	clientId     = "CLIENT_ID"
	clientSecret = "CLIENT_SECRET"
	tenantId     = "TENANT_ID"
	endpoint     = "ENDPOINT"
	database     = "DATABASE"
)

type KustoConfig struct {
	ClientID     string
	ClientSecret string
	TenantID     string
	Endpoint     string
	Database     string
}

func InitConfig(configPath string) *KustoConfig {
	var kustoConfig *KustoConfig
	if configPath != "" {
		viper.SetConfigFile(path.Base(configPath))
		viper.AddConfigPath(path.Dir(configPath))
	}

	v := viper.New()
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	kustoConfig = &KustoConfig{
		ClientID:     v.GetString(clientId),
		ClientSecret: v.GetString(clientSecret),
		TenantID:     v.GetString(tenantId),
		Endpoint:     v.GetString(endpoint),
		Database:     v.GetString(database),
	}

	return kustoConfig
}
