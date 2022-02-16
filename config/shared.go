package config

import (
	"errors"
	"github.com/spf13/viper"
	"os"
)

func load(path string, data interface{}) error {
	if path == "" {
		return errors.New("empty path to config")
	}

	_, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("json")

	if err := v.ReadInConfig(); err != nil {
		return err
	}

	return v.Unmarshal(data)
}
