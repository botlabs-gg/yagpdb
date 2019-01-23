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
