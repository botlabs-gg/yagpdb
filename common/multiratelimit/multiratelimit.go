package multiratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type MultiRatelimiter struct {
	mu       sync.Mutex
	limiters map[interface{}]*rate.Limiter

	maxPerSecond float64
	maxBurst     int
}

func NewMultiRatelimiter(maxPerSecond float64, maxBurst int) *MultiRatelimiter {
	multiLimiter := &MultiRatelimiter{
		limiters: make(map[interface{}]*rate.Limiter),

		maxPerSecond: maxPerSecond,
		maxBurst:     maxBurst,
	}

	return multiLimiter
}

func (multi *MultiRatelimiter) findCreateLimiter(key interface{}) *rate.Limiter {
	multi.mu.Lock()
	defer multi.mu.Unlock()

	if current, ok := multi.limiters[key]; ok {
		return current
	}

	// not found, create it
	multi.limiters[key] = rate.NewLimiter(rate.Limit(multi.maxPerSecond), multi.maxBurst)
	return multi.limiters[key]
}

func (multi *MultiRatelimiter) AllowN(key interface{}, now time.Time, n int) bool {
	return multi.findCreateLimiter(key).AllowN(now, n)
}
