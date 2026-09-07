package reddit

import (
	"testing"
	"time"
)

func TestRatelimit(t *testing.T) {
	now := time.Now()
	// originalT := now

	rl := NewRatelimiter()

	for i := 0; i < 10; i++ {
		if !rl.CheckIncrement(now, 1, 7) {
			t.Error("premature false")
		}
		now = now.Add(time.Minute * 10)
	}

	if !rl.CheckIncrement(now, 1, 7) {
		t.Error("premature false")
	}
	if rl.CheckIncrement(now, 1, 7) {
		t.Error("should fail?")
	}

	if len(rl.Windows) != 11 {
		t.Error("num windows expected to be 11 but got ", len(rl.Windows))
	}

	rl.FullGC(now)

	if len(rl.Windows) != 7 {
		t.Error("num windows expected to be 7 but got ", len(rl.Windows))
	}
}

func TestRequestPacer(t *testing.T) {
	now := time.Now()
	pacer := &requestPacer{interval: time.Second}

	if !pacer.Allow(now) {
		t.Error("first request should be allowed")
	}

	if pacer.Allow(now.Add(time.Millisecond * 999)) {
		t.Error("request within the interval should be denied")
	}

	if !pacer.Allow(now.Add(time.Second)) {
		t.Error("request after the interval should be allowed")
	}

	// idling must not build up credit for a burst
	idled := now.Add(time.Hour)
	if !pacer.Allow(idled) {
		t.Error("request after idling should be allowed")
	}
	if pacer.Allow(idled) {
		t.Error("idling should not allow a second immediate request")
	}
}
