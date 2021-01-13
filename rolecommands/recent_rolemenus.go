package rolecommands

import (
	"sync"
	"time"

	"github.com/jonas747/yagpdb/bot"
)

type RecentTrackedMenu struct {
	t     time.Time
	MsgID int64
}

// RecentMenusTracker is simply a way to reduce database queries
// We keep a small cache of message id's with menus on them created recently, that way
// we don't need to query the database on all messages with reactions on them if they're created within this tracked time interval
type RecentMenusTracker struct {
	RecentMenus []*RecentTrackedMenu
	Started     time.Time
	EvictionAge time.Duration

	mu sync.RWMutex
}

func NewRecentMenusTracker(evictionTreshold time.Duration) *RecentMenusTracker {
	tracker := &RecentMenusTracker{
		RecentMenus: make([]*RecentTrackedMenu, 0),
		Started:     time.Now(),
		EvictionAge: evictionTreshold,
	}

	go tracker.RunLoop()
	return tracker
}

func (r *RecentMenusTracker) AddMenu(msgID int64) {
	t := bot.SnowflakeToTime(msgID)
	if time.Since(t) > r.EvictionAge {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, v := range r.RecentMenus {
		if v.MsgID == msgID {
			return // Collision
		}
	}

	r.RecentMenus = append(r.RecentMenus, &RecentTrackedMenu{
		t:     t,
		MsgID: msgID,
	})
}

func (r *RecentMenusTracker) CheckRecentTrackedMenu(msgID int64) (outOfTimeRange bool, checkDB bool) {
	timestamp := bot.SnowflakeToTime(msgID)

	if timestamp.Before(r.Started) || time.Since(timestamp) > r.EvictionAge {
		return true, true // outside of tracked range, need to check DB
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, v := range r.RecentMenus {
		if v.MsgID == msgID {
			// there's a menu created on this message
			return false, true
		}
	}

	// no need to check db, no menu found here
	return false, false
}

func (r *RecentMenusTracker) RunLoop() {
	tickInterval := time.Minute

	ticker := time.NewTicker(tickInterval)
	for {
		<-ticker.C
		r.loopCheck(time.Now().Add(-(r.EvictionAge + tickInterval)))
	}
}

func (r *RecentMenusTracker) loopCheck(treshold time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	newList := make([]*RecentTrackedMenu, 0, len(r.RecentMenus))
	for _, v := range r.RecentMenus {
		if v.t.After(treshold) {
			newList = append(newList, v)
		}
	}

	r.RecentMenus = newList
	logger.Infof("new list len: %d", len(newList))
}
