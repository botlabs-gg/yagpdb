package multiratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type MultiRatelimiter struct {
	mu       sync.Mutex
	limiters map[any]*rate.Limiter

	maxPerSecond float64
	maxBurst     int

	lastPrune time.Time
}

const pruneInterval = time.Minute

func NewMultiRatelimiter(maxPerSecond float64, maxBurst int) *MultiRatelimiter {
	multiLimiter := &MultiRatelimiter{
		limiters: make(map[any]*rate.Limiter),

		maxPerSecond: maxPerSecond,
		maxBurst:     maxBurst,
	}

	return multiLimiter
}

func (multi *MultiRatelimiter) findCreateLimiter(key any, now time.Time) *rate.Limiter {
	multi.mu.Lock()
	defer multi.mu.Unlock()

	if now.Sub(multi.lastPrune) >= pruneInterval {
		multi.prune(now)
	}

	if current, ok := multi.limiters[key]; ok {
		return current
	}

	// not found, create it
	multi.limiters[key] = rate.NewLimiter(rate.Limit(multi.maxPerSecond), multi.maxBurst)
	return multi.limiters[key]
}

func (multi *MultiRatelimiter) AllowN(key any, now time.Time, n int) bool {
	return multi.findCreateLimiter(key, now).AllowN(now, n)
}

// A limiter with a full bucket behaves exactly like a new one, so dropping it only frees memory.
func (multi *MultiRatelimiter) prune(now time.Time) {
	multi.lastPrune = now
	for key, limiter := range multi.limiters {
		if limiter.TokensAt(now) >= float64(multi.maxBurst) {
			delete(multi.limiters, key)
		}
	}
}
