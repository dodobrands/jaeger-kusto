package store

import (
	"fmt"
	"path"

	"github.com/spf13/viper"
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
	v := viper.New()
	if configPath != "" {
		v.SetConfigFile(path.Base(configPath))
		v.AddConfigPath(path.Dir(configPath))
		//viper.SetConfigName("config")
		v.SetConfigType("yaml")
		err := v.ReadInConfig() // Find and read the config file
		if err != nil {         // Handle errors reading the config file
			panic(fmt.Errorf("Fatal error config file: %s \n", err))
		}
	}
	v.AutomaticEnv()

	kustoConfig = &KustoConfig{
		ClientID:     v.GetString(clientId),
		ClientSecret: v.GetString(clientSecret),
		TenantID:     v.GetString(tenantId),
		Endpoint:     v.GetString(endpoint),
		Database:     v.GetString(database),
	}

	return kustoConfig
}
