// Cache utilities
// TODO: Also use a local application cache to save redis rountrips

package common

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/karlseguin/ccache"
	"github.com/mediocregopher/radix/v3"
)

var (
	ErrNotFound    = errors.New("Not found")
	CacheKeyPrefix = "cache_"

	Cache *ccache.Cache
)

// Items in the cache expire after 1 min
func GetCacheData(key string) (data []byte, err error) {
	err = RedisPool.Do(radix.Cmd(&data, "GET", CacheKeyPrefix+key))
	return
}

// Stores an entry in the cache and sets it to expire after expire
func SetCacheData(key string, expire int, data []byte) error {
	err := RedisPool.Do(radix.Cmd(nil, "SET", CacheKeyPrefix+key, string(data), "EX", strconv.Itoa(expire)))
	return err
}

// Stores an entry in the cache and sets it to expire after a minute
func SetCacheDataSimple(key string, data []byte) error {
	return SetCacheData(key, 60, data)
}

// Helper methods
func SetCacheDataJson(key string, expire int, data interface{}) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return SetCacheData(key, expire, encoded)
}

func SetCacheDataJsonSimple(key string, data interface{}) error {
	return SetCacheDataJson(key, 60, data)
}

func GetCacheDataJson(key string, dest interface{}) error {
	data, err := GetCacheData(key)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, dest)
	return err
}
