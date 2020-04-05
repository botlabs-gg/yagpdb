package common

import (
	"strconv"
	"time"

	"emperror.dev/errors"
	"github.com/mediocregopher/radix/v3"
)

// Locks the lock and if succeded sets it to expire after maxdur
// So that if someting went wrong its not locked forever
func TryLockRedisKey(key string, maxDur int) (bool, error) {
	resp := ""
	err := RedisPool.Do(radix.Cmd(&resp, "SET", key, "1", "NX", "EX", strconv.Itoa(maxDur)))
	if err != nil {
		return false, err
	}

	if resp == "OK" {
		return true, nil
	}

	return false, nil
}

var (
	ErrMaxLockAttemptsExceeded = errors.New("Max lock attempts exceeded")
)

// BlockingLockRedisKey blocks until it suceeded to lock the key
func BlockingLockRedisKey(key string, maxTryDuration time.Duration, maxLockDur int) error {
	started := time.Now()
	sleepDur := time.Millisecond * 100
	maxSleep := time.Second
	for {
		if maxTryDuration != 0 && time.Since(started) > maxTryDuration {
			return ErrMaxLockAttemptsExceeded
		}

		locked, err := TryLockRedisKey(key, maxLockDur)
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

func UnlockRedisKey(key string) {
	RedisPool.Do(radix.Cmd(nil, "DEL", key))
}
