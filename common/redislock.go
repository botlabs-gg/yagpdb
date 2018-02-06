package common

import (
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/pkg/errors"
	"time"
)

// Locks the lock and if succeded sets it to expire after maxdur
// So that if someting went wrong its not locked forever
func TryLockRedisKey(client *redis.Client, key string, maxDur int) (bool, error) {
	reply := client.Cmd("SET", key, true, "NX", "EX", maxDur)
	if reply.IsType(redis.Nil) {
		return false, nil
	}

	return RedisBool(reply)
}

var (
	ErrMaxLockAttemptsExceeded = errors.New("Max lock attempts exceeded")
)

// BlockingLockRedisKey blocks until it suceeded to lock the key
func BlockingLockRedisKey(client *redis.Client, key string, maxTryDuration time.Duration, maxLockDur int) error {
	started := time.Now()
	sleepDur := time.Millisecond * 100
	maxSleep := time.Second
	for {
		if maxTryDuration != 0 && time.Since(started) > maxTryDuration {
			return ErrMaxLockAttemptsExceeded
		}

		locked, err := TryLockRedisKey(client, key, maxLockDur)
		if err != nil {
			return ErrWithCaller(err)
		}

		if locked {
			return nil
		}

		time.Sleep(sleepDur)
		sleepDur *= 2
		if sleepDur > maxSleep {
			sleepDur = maxSleep
		}
	}
}

func UnlockRedisKey(client *redis.Client, key string) {
	client.Cmd("DEL", key)
}
