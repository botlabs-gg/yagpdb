package config

import (
	"os"
	"strings"
)

type EnvSource struct{}

func (e *EnvSource) GetValue(key string) interface{} {
	properKey := strings.ToUpper(key)
	properKey = strings.Replace(properKey, ".", "_", -1)
	v := os.Getenv(properKey)
	if v == "" {
		return nil
	}
	return v
}

func (e *EnvSource) Name() string {
	return "env"
}
