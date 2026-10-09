package common

import (
	"crypto/rand"
	"encoding/hex"
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
	for {
		err := RedisPool.Do(radix.Cmd(nil, "DEL", key))
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		break
	}
}

// LeasedRedisLock is held for a short lease that is renewed in the background
// while the holder runs, so a holder that dies frees it within one lease rather
// than after a fixed expiry long enough to cover the slowest holder.
type LeasedRedisLock struct {
	key   string
	token string
	stop  chan struct{}
	done  chan struct{}
}

// The token check keeps a holder whose lease ran out from renewing or deleting
// a lock that has since been taken by someone else.
var (
	renewLeasedLockScript   = radix.NewEvalScript(1, `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("PEXPIRE", KEYS[1], ARGV[2]) end return 0`)
	releaseLeasedLockScript = radix.NewEvalScript(1, `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) end return 0`)
)

// BlockingLeasedLockRedisKey blocks until it locks the key, or maxTryDuration
// passes if it is not 0.
func BlockingLeasedLockRedisKey(key string, maxTryDuration, lease time.Duration) (*LeasedRedisLock, error) {
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)
	leaseMS := strconv.FormatInt(lease.Milliseconds(), 10)

	started := time.Now()
	sleepDur := time.Millisecond * 100
	maxSleep := time.Second
	for {
		if maxTryDuration != 0 && time.Since(started) > maxTryDuration {
			return nil, ErrMaxLockAttemptsExceeded
		}

		resp := ""
		err := RedisPool.Do(radix.Cmd(&resp, "SET", key, token, "NX", "PX", leaseMS))
		if err != nil {
			return nil, ErrWithCaller(err)
		}

		if resp == "OK" {
			break
		}

		time.Sleep(sleepDur)
		sleepDur = min(sleepDur*2, maxSleep)
	}

	lock := &LeasedRedisLock{
		key:   key,
		token: token,
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	go lock.renew(lease, leaseMS)
	return lock, nil
}

func (l *LeasedRedisLock) renew(lease time.Duration, leaseMS string) {
	defer close(l.done)

	ticker := time.NewTicker(lease / 3)
	defer ticker.Stop()

	for {
		select {
		case <-l.stop:
			return
		case <-ticker.C:
			var renewed int
			err := RedisPool.Do(renewLeasedLockScript.Cmd(&renewed, l.key, l.token, leaseMS))
			if err != nil {
				logger.WithError(err).WithField("key", l.key).Error("failed renewing redis lock lease")
			} else if renewed == 0 {
				logger.WithField("key", l.key).Error("redis lock lease ran out before it could be renewed")
			}
		}
	}
}

// Unlock stops renewing the lease and releases the lock if it is still ours.
func (l *LeasedRedisLock) Unlock() {
	select {
	case <-l.stop:
		return
	default:
		close(l.stop)
	}
	<-l.done

	for {
		err := RedisPool.Do(releaseLeasedLockScript.Cmd(nil, l.key, l.token))
		if err == nil {
			return
		}

		logger.WithError(err).WithField("key", l.key).Error("failed releasing redis lock, retrying")
		time.Sleep(time.Second)
	}
}
