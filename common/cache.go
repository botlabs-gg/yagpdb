package common

import (
	"encoding/json"
	"errors"
	"github.com/fzzy/radix/redis"
)

var (
	ErrNotFound = errors.New("Not found")
)

// Items in the cache expire after 1 min
func GetCacheData(client *redis.Client, key string) ([]byte, error) {
	client.Append("SELECT", "2")
	client.Append("GET", key)
	client.Append("SELECT", 0)

	replies := GetRedisReplies(client, 3)
	for _, r := range replies {
		if r.Err != nil {
			return nil, r.Err
		}
	}

	data, err := replies[1].Bytes()

	return data, err
}

// Stores an entry in the cache and sets it to expire after expire
func SetCacheData(client *redis.Client, key string, expire int, data []byte) error {
	cmds := []*RedisCmd{
		&RedisCmd{Name: "SELECT", Args: []interface{}{2}},
		&RedisCmd{Name: "SET", Args: []interface{}{key, data}},
		&RedisCmd{Name: "EXPIRE", Args: []interface{}{key, expire}},
		&RedisCmd{Name: "SELECT", Args: []interface{}{0}},
	}

	_, err := SafeRedisCommands(client, cmds)
	return err
}

// Stores an entry in the cache and sets it to expire after a minute
func SetCacheDataSimple(client *redis.Client, key string, data []byte) error {
	return SetCacheData(client, key, 60, data)
}

// Helper methods
func SetCacheDataJson(client *redis.Client, key string, expire int, data interface{}) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return SetCacheData(client, key, expire, encoded)
}

func SetCacheDataJsonSimple(client *redis.Client, key string, data interface{}) error {
	return SetCacheDataJson(client, key, 60, data)
}

func GetCacheDataJson(client *redis.Client, key string, dest interface{}) error {
	data, err := GetCacheData(client, key)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, dest)
	return err
}
