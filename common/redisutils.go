package common

import (
	"encoding/json"
	"fmt"
	"github.com/mediocregopher/radix.v2/redis"
	"strings"
	"time"
)

// GetRedisReplies is a helper func when using redis pipelines
// It retrieves n amount of replies and returns the first error it finds (but still continues to retrieve replies after that)
func GetRedisReplies(client *redis.Client, n int) ([]*redis.Resp, error) {
	var err error
	out := make([]*redis.Resp, n)
	for i := 0; i < n; i++ {
		reply := client.PipeResp()
		out[i] = reply
		if reply.Err != nil && err == nil {
			err = reply.Err
		}
	}
	return out, err
}

type RedisCmd struct {
	Name string
	Args []interface{}
}

// SafeRedisCommands Will do the following commands and stop if an error occurs
func SafeRedisCommands(client *redis.Client, cmds []*RedisCmd) ([]*redis.Resp, error) {
	out := make([]*redis.Resp, 0)
	for _, cmd := range cmds {
		reply := client.Cmd(cmd.Name, cmd.Args...)
		out = append(out, reply)
		if reply.Err != nil {
			return out, reply.Err
		}
	}
	return out, nil
}

func RedisDialFunc(network, addr string) (client *redis.Client, err error) {
	for {
		client, err = redis.DialTimeout(network, addr, time.Second*10)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "socket: too many open files") ||
				strings.Contains(errStr, "cannot assign requested address") {
				// Sleep for 100 milliseconds and try again
				time.Sleep(time.Millisecond * 100)
				continue
			} else {
				return
			}
		} else {
			break
		}
	}

	return
}

func GenID(client *redis.Client, key string) string {
	idInt, err := client.Cmd("INCR", key).Int64()
	if err != nil {
		panic(err)
	}

	return fmt.Sprintf("r%d", idInt)
}

// GetRedisJson executes a get redis command and unmarshals the value into out
func GetRedisJson(client *redis.Client, key string, out interface{}) error {
	reply := client.Cmd("GET", key)
	if reply.IsType(redis.Nil) {
		return nil
	}

	raw, err := reply.Bytes()
	if err != nil {
		return err
	}

	err = json.Unmarshal(raw, out)
	return err
}

// GetRedisJson executes a get redis command and unmarshals the value into out
func GetRedisJsonDefault(client *redis.Client, key string, out interface{}) error {
	reply := client.Cmd("GET", key)
	if reply.IsType(redis.Nil) {
		return nil
	}

	raw, err := reply.Bytes()
	if err != nil {
		return err
	}

	err = json.Unmarshal(raw, out)
	return err
}

// SetRedisJson marshals the value and runs a set redis command for key
func SetRedisJson(client *redis.Client, key string, value interface{}) error {
	serialized, err := json.Marshal(value)
	if err != nil {
		return err
	}

	err = client.Cmd("SET", key, serialized).Err
	return err
}

func MustGetRedisClient() *redis.Client {
	client, err := RedisPool.Get()
	if err != nil {
		panic("Failed retrieving redis client from pool: " + err.Error())
	}
	return client
}

// Locks the lock and if succeded sets it to expire after maxdur
// So that if someting went wrong its not locked forever
func TryLockRedisKey(client *redis.Client, key string, maxDur int) (bool, error) {
	didSet := !client.Cmd("SET", key, true, "NX", "EX", maxDur).IsType(redis.Nil)
	return didSet, nil
}

// BlockingLockRedisKey blocks until it suceeded to lock the key
func BlockingLockRedisKey(client *redis.Client, key string, maxDur int) error {
	for {
		locked, err := TryLockRedisKey(client, key, maxDur)
		if err != nil {
			return ErrWithCaller(err)
		}

		if locked {
			return nil
		}

		time.Sleep(time.Millisecond)
	}
}

func UnlockRedisKey(client *redis.Client, key string) {
	client.Cmd("DEL", key)
}

func RedisBool(resp *redis.Resp) (b bool, err error) {
	if resp.Err != nil {
		return false, resp.Err
	}

	if resp.IsType(redis.Nil) {
		return false, nil
	}

	if resp.IsType(redis.Int) {
		i, err := resp.Int()
		return i > 0, err
	}

	if resp.IsType(redis.Str) {
		s, err := resp.Str()
		return (s != "" && s != "false"), err
	}

	panic("Unknown redis reply type")
}
