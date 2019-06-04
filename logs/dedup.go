package logs

import (
	"container/ring"
	"time"
)

type DeDupPresenceUpdates struct {
	r *ring.Ring
}

func NewDeduper(size int) *DeDupPresenceUpdates {
	return &DeDupPresenceUpdates{
		r: ring.New(size),
	}
}

type DeDubPair struct {
	UserID   int64
	Inserted time.Time
}

func (d *DeDupPresenceUpdates) CheckDupe(userID int64) bool {
	now := time.Now()

	// check current elem
	match, cont := d.checkElem(userID, d.r.Value, now)
	if match {
		return true
	}

	// check buffer
	if cont {
		for p := d.r.Next(); p != d.r; p = p.Next() {
			match, cont := d.checkElem(userID, p.Value, now)
			if match {
				return true
			}

			if !cont {
				break
			}
		}
	}

	d.Insert(userID, now)

	return false
}

func (d *DeDupPresenceUpdates) Insert(userID int64, t time.Time) {
	d.r.Move(-1)
	d.r.Value = DeDubPair{
		UserID:   userID,
		Inserted: t,
	}
}

func (d *DeDupPresenceUpdates) checkElem(targetUserID int64, v interface{}, t time.Time) (match bool, cont bool) {
	if v == nil {
		return false, false
	}

	cast := v.(DeDubPair)

	if t.Sub(cast.Inserted) > time.Second*5 {
		return false, false
	}

	if cast.UserID == targetUserID {
		return true, false
	}

	return false, true
}
