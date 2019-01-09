package reddit

import (
	"sync"
	"time"
)

type Ratelimiter struct {
	mu      sync.Mutex
	Windows []*RatelimitWindow
}

func NewRatelimiter() *Ratelimiter {
	return &Ratelimiter{}
}

type RatelimitWindow struct {
	T      time.Time
	ID     int64
	Guilds map[int64]int
}

func (r *Ratelimiter) CheckIncrement(t time.Time, guildID int64, limit int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	c := 0
	for _, v := range r.Windows {
		if t.Sub(v.T) > time.Hour {
			continue
		}

		c += v.Guilds[guildID]
	}

	if c >= limit {
		return false
	}

	window := r.findCurrentWindow(t)
	window.Guilds[guildID]++
	return true
}

func (r *Ratelimiter) findCurrentWindow(t time.Time) *RatelimitWindow {
	id := t.Unix() / 600

	// set t to when this window ends
	t = time.Unix((id*600)+600, 0)
	for _, v := range r.Windows {
		if v.ID == id {
			return v
		}
	}

	w := &RatelimitWindow{
		T:      t,
		ID:     id,
		Guilds: make(map[int64]int),
	}

	r.Windows = append(r.Windows, w)

	return w
}

func (r *Ratelimiter) GC(t time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, v := range r.Windows {
		if t.Sub(v.T) > time.Hour {
			r.Windows = append(r.Windows[:i], r.Windows[i+1:]...)
			return true
		}
	}

	return false
}

func (r *Ratelimiter) FullGC(t time.Time) {
	for r.GC(t) {
	}
}

func (r *Ratelimiter) RunGCLoop() {
	ticker := time.NewTicker(time.Minute)
	for {
		<-ticker.C

		r.FullGC(time.Now())
	}
}
