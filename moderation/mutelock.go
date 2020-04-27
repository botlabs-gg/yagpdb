package moderation

import (
	"sync"
	"time"
)

var (
	muteLocks   = make(map[int64]bool)
	muteLocksmu sync.Mutex
)

func LockMute(uID int64) {
	for {
		muteLocksmu.Lock()
		if l, ok := muteLocks[uID]; !ok || !l {
			muteLocks[uID] = true
			muteLocksmu.Unlock()
			return
		}
		muteLocksmu.Unlock()

		time.Sleep(time.Millisecond * 250)
	}
}

func UnlockMute(uID int64) {
	muteLocksmu.Lock()
	delete(muteLocks, uID)
	muteLocksmu.Unlock()
}
