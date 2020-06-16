package store

import (
	"io/ioutil"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/viper"
)

// KustoConfig contains AzureAD service principal and Kusto cluster configs
type KustoConfig struct {
	ClientID     string
	ClientSecret string
	TenantID     string
	Endpoint     string
	Database     string
}

// InitConfig reads config from file
func InitConfig(configPath string, logger hclog.Logger) *KustoConfig {

	var kustoConfig *KustoConfig

	v := viper.New()

	if configPath != "" {
		logger.Debug("trying to read config file")
		logger.Debug("configPath is " + configPath)

		f, err := ioutil.ReadFile(configPath)

		if err != nil { // Handle errors reading the config file
			logger.Error("error reading config file", err.Error())
		}
		logger.Debug("file contents:" + string(f))

		logger.Debug("initializing Kusto storage")

		v.SetConfigFile(configPath)
		v.SetConfigType("json")
		err = v.ReadInConfig() // Find and read the config file
		if err != nil {        // Handle errors reading the config file
			logger.Error("error reading config file", err.Error())
		}
	}
	v.AutomaticEnv()

	kustoConfig = &KustoConfig{
		ClientID:     v.GetString("clientId"),
		ClientSecret: v.GetString("clientSecret"),
		TenantID:     v.GetString("tenantId"),
		Endpoint:     v.GetString("endpoint"),
		Database:     v.GetString("database"),
	}

	return kustoConfig
}
