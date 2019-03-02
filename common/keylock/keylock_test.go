package keylock

import (
	"testing"
	"time"
)

func TestKeyLock(t *testing.T) {
	locker := NewKeyLock()

	h := locker.Lock(1, time.Second, time.Minute)

	startedWaiting := time.Now()
	go func(lh int64) {
		time.Sleep(time.Second)
		locker.Unlock(1, lh)
	}(h)

	h2 := locker.Lock(1, time.Minute, time.Minute)
	locker.Unlock(1, h2)

	if time.Since(startedWaiting) < time.Second {
		t.Error("Did not wait a second before locking key ", time.Since(startedWaiting))
	}
}

func BenchmarkKeyLock(b *testing.B) {
	locker := NewKeyLock()

	for i := 0; i < b.N; i++ {
		h := locker.Lock(1, time.Minute, time.Minute)
		locker.Unlock(1, h)
	}
}
