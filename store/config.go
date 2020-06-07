package store

import (
	"fmt"
	"github.com/spf13/viper"
	"path"
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
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		err := viper.ReadInConfig() // Find and read the config file
		if err != nil { // Handle errors reading the config file
			panic(fmt.Errorf("Fatal error config file: %s \n", err))
		}
	}

	v := viper.New()
	v.AutomaticEnv()
	//v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	kustoConfig = &KustoConfig{
		ClientID:     v.GetString(clientId),
		ClientSecret: v.GetString(clientSecret),
		TenantID:     v.GetString(tenantId),
		Endpoint:     v.GetString(endpoint),
		Database:     v.GetString(database),
	}

	return kustoConfig
}
