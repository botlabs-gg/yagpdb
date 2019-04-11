package keylock

import (
	"sync"
	"time"
)

type bucket struct {
	expires time.Time
	handle  int64
}

// KeyLock is a simple implementation of key based locks with ttl's on them
type KeyLock struct {
	locks   map[interface{}]*bucket
	locksMU sync.Mutex
	c       int64
}

func NewKeyLock() *KeyLock {
	return &KeyLock{
		locks: make(map[interface{}]*bucket),
	}
}

// Lock attempts to lock the specified key for the specified duration, expiring after ttl
// if it fails to grab the key after timeout it will return -1,
// it will return a lock handle otherwise that you use to unlock it with.
// this is to protect against you unlocking it after the ttl expired when something else is holding it
func (kl *KeyLock) Lock(key interface{}, timeout time.Duration, ttl time.Duration) int64 {
	started := time.Now()

	for {
		if handle := kl.tryLock(key, ttl); handle != -1 {
			return handle
		}

		if time.Since(started) >= timeout {
			return -1
		}

		time.Sleep(time.Millisecond * 250)
	}
}

func (kl *KeyLock) tryLock(key interface{}, ttl time.Duration) int64 {
	kl.locksMU.Lock()
	now := time.Now()

	// if there is no lock, or were past the expiry of it
	if b, ok := kl.locks[key]; !ok || (b == nil || now.After(b.expires)) {
		// then we can sucessfully lock it
		kl.c++
		handle := kl.c
		kl.locks[key] = &bucket{
			handle:  handle,
			expires: now.Add(ttl),
		}

		kl.locksMU.Unlock()
		return handle
	}

	kl.locksMU.Unlock()
	return -1
}

func (kl *KeyLock) Unlock(key interface{}, handle int64) {
	kl.locksMU.Lock()
	if b, ok := kl.locks[key]; ok && b != nil && b.handle == handle {
		// only delete it if the caller is the one holding the lock
		delete(kl.locks, key)
	}
	kl.locksMU.Unlock()
}
