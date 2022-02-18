package config

import (
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"os"
	"reflect"
	"unicode"
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

func override(prefix string, data interface{}) error {
	v := viper.New()
	v.SetEnvPrefix(prefix)

	pointerType := reflect.TypeOf(data)
	if pointerType.Kind() != reflect.Ptr {
		return errors.New("data not a pointer to struct")
	}
	dataType := pointerType.Elem()
	if dataType.Kind() != reflect.Struct {
		return errors.New("data not a pointer to struct")
	}

	keys := make([]string, dataType.NumField())
	for i := 0; i < dataType.NumField(); i++ {
		keys = append(keys, dataType.Field(i).Name)
	}

	for _, key := range keys {
		if key == "" {
			continue
		}

		variable := fmt.Sprintf("%s_%s", prefix, toEnvironmentVariable(key))
		if err := v.BindEnv(key, variable); err != nil {
			return err
		}
	}

	return v.Unmarshal(data)
}

func toEnvironmentVariable(name string) string {
	output := ""
	runes := []rune(name)

	var c, p rune
	for i := 0; i < len(runes); i++ {
		c = runes[i]
		if unicode.IsUpper(c) && unicode.IsLower(p) {
			output += "_"
		}
		output += string(unicode.ToUpper(c))
		p = c
	}

	return output
}
