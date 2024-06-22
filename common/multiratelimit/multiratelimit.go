package multiratelimit

import (
	"sync"

	"golang.org/x/time/rate"
)

type MultiRatelimiter[T comparable] struct {
	mu       sync.Mutex
	limiters map[T]*rate.Limiter

	maxPerSecond float64
	maxBurst     int
}

func NewMultiRatelimiter[T comparable](maxPerSecond float64, maxBurst int) *MultiRatelimiter[T] {
	multiLimiter := &MultiRatelimiter[T]{
		limiters: make(map[T]*rate.Limiter),

		maxPerSecond: maxPerSecond,
		maxBurst:     maxBurst,
	}

	return multiLimiter
}

func (multi *MultiRatelimiter[T]) findCreateLimiter(key T) *rate.Limiter {
	multi.mu.Lock()
	defer multi.mu.Unlock()

	if current, ok := multi.limiters[key]; ok {
		return current
	}

	// not found, create it
	multi.limiters[key] = rate.NewLimiter(rate.Limit(multi.maxPerSecond), multi.maxBurst)
	return multi.limiters[key]
}

func (multi *MultiRatelimiter[T]) Allow(key T) bool {
	return multi.findCreateLimiter(key).Allow()
}

func (multi *MultiRatelimiter[T]) Reserve(key T) *rate.Reservation {
	return multi.findCreateLimiter(key).Reserve()
}
