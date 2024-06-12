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
type KeyLock[K comparable] struct {
	locks map[K]*bucket
	mu    sync.Mutex
	c     int64
}

func NewKeyLock[K comparable]() *KeyLock[K] {
	return &KeyLock[K]{
		locks: make(map[K]*bucket),
	}
}

// Lock attempts to lock the key for the given duration ttl, blocking until it succeeds
// or the timeout passes.
//
// Specifically, if the key is currently locked by another caller and cannot be
// obtained within timeout, Lock returns -1. Otherwise, Lock returns a
// non-negative handle that can be passed to KeyLock.Lock to unlock the key
// before the ttl expires. (The lock handle guards against the case where
// callers attempt to unlock keys that have already expired and have since been
// re-locked by a different caller.)
func (kl *KeyLock[K]) Lock(key K, timeout time.Duration, ttl time.Duration) (handle int64) {
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

func (kl *KeyLock[K]) tryLock(key K, ttl time.Duration) int64 {
	kl.mu.Lock()
	now := time.Now()

	// if there is no lock, or the bucket has expired
	if b, ok := kl.locks[key]; !ok || (b == nil || now.After(b.expires)) {
		// then we can sucessfully lock it
		kl.c++
		handle := kl.c
		kl.locks[key] = &bucket{
			handle:  handle,
			expires: now.Add(ttl),
		}

		kl.mu.Unlock()
		return handle
	}

	kl.mu.Unlock()
	return -1
}

func (kl *KeyLock[K]) Unlock(key K, handle int64) {
	kl.mu.Lock()
	if b, ok := kl.locks[key]; ok && b != nil && b.handle == handle {
		// only delete if the caller is the one holding the lock
		delete(kl.locks, key)
	}
	kl.mu.Unlock()
}
