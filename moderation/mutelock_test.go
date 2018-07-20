package moderation

import (
	"testing"
	"time"
)

func TestMuteLock(t *testing.T) {
	LockMute(1)

	startedWaiting := time.Now()
	go func() {
		time.Sleep(time.Second)
		UnlockMute(1)
	}()

	LockMute(1)
	UnlockMute(1)

	if time.Since(startedWaiting) < time.Second {
		t.Error("Did not wait a second before locking key ", time.Since(startedWaiting))
	}
}

func BenchmarkMuteLock(b *testing.B) {
	for i := 0; i < b.N; i++ {
		LockMute(1)
		UnlockMute(1)
	}
}
