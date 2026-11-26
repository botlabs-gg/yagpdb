package multiratelimit

import (
	"testing"
	"time"
)

func TestMultiRatelimiterPrunesIdleLimiters(t *testing.T) {
	limiter := NewMultiRatelimiter(1, 2)
	now := time.Now()

	if !limiter.AllowN("a", now, 2) {
		t.Fatal("expected the burst to be allowed")
	}
	if limiter.AllowN("a", now, 1) {
		t.Fatal("expected to be limited after the burst")
	}
	if !limiter.AllowN("b", now, 1) {
		t.Fatal("expected a different key to have its own limit")
	}

	later := now.Add(pruneInterval)
	if !limiter.AllowN("c", later, 1) {
		t.Fatal("expected a new key to be allowed")
	}
	if _, ok := limiter.limiters["a"]; ok {
		t.Error("expected a refilled limiter to be pruned")
	}
	if !limiter.AllowN("a", later, 2) {
		t.Error("expected a pruned key to get its full burst back")
	}

	soon := later.Add(time.Second)
	limiter.AllowN("c", soon, 0)
	if len(limiter.limiters) != 2 {
		t.Errorf("expected no pruning within the interval, have %d limiters", len(limiter.limiters))
	}
}
