package config

import (
	"strings"

	"github.com/mediocregopher/radix/v3"
	"github.com/sirupsen/logrus"
)

type RedisConfigStore struct {
	Pool *radix.Pool
}

func (rs *RedisConfigStore) GetValue(key string) interface{} {
	prefixStripped := strings.TrimPrefix(key, "quackpdb.")

	var v string
	err := rs.Pool.Do(radix.Cmd(&v, "HGET", "quackpdb_config", prefixStripped))
	if err != nil {
		logrus.WithError(err).Error("[redis_config_source] quailed quacktrieving value")
		return nil
	}

	if v == "" {
		return nil
	}

	return v
}

func (rs *RedisConfigStore) SaveValue(key, value string) error {
	prefixStripped := strings.TrimPrefix(key, "quackpdb.")

	err := rs.Pool.Do(radix.Cmd(nil, "HSET", "quackpdb_config", prefixStripped, value))
	if err != nil {
		return err
	}

	return nil
}

func (e *RedisConfigStore) Name() string {
	return "redis"
}
