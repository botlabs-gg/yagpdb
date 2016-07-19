package common

import (
	"encoding/json"
	"github.com/fzzy/radix/redis"
)

// Items in the cache expire after 1 min
func GetCacheData(client *redis.Client, key string) ([]byte, error) {
	client.Append("SELECT", "2")
	client.Append("GET", key)
	client.Append("SELECT", "0")

	// Select reply
	if reply := client.GetReply(); reply.Err != nil {
		return nil, reply.Err
	}

	// GET reply
	reply := client.GetReply()
	data, err := reply.Bytes()
	if err != nil {
		return nil, err
	}

	selectReply := client.GetReply()
	return data, selectReply.Err
}

// Stores an entry in the cache and sets it to expire after a minute
func SetCacheData(client *redis.Client, key string, data []byte) error {
	client.Append("SELECT", "2")
	client.Append("SET", key, data)
	client.Append("EXPIRE", key, 60)
	client.Append("SELECT", 0)

	for i := 0; i < 4; i++ {
		reply := client.GetReply()
		if reply.Err != nil {
			return reply.Err
		}
	}
	return nil
}

// Helper methods
func SetCacheDataJson(client *redis.Client, key string, data interface{}) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return SetCacheData(client, key, encoded)
}

func GetCacheDataJson(client *redis.Client, key string, dest interface{}) error {
	data, err := GetCacheData(client, key)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, dest)
	return err
}
