package moderation

import (
	"sync"
	"time"
)

var (
	lockdownLocks   = make(map[int64]bool)
	lockdownLocksmu sync.Mutex
)

func LockLockdown(rID int64) {
	for {
		lockdownLocksmu.Lock()
		if l, ok := lockdownLocks[rID]; !ok || !l {
			lockdownLocks[rID] = true
			lockdownLocksmu.Unlock()
			return
		}
		lockdownLocksmu.Unlock()

		time.Sleep(time.Millisecond * 250)
	}
}

func UnlockLockdown(rID int64) {
	lockdownLocksmu.Lock()
	delete(lockdownLocks, rID)
	lockdownLocksmu.Unlock()
}
