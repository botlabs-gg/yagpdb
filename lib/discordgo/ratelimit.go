package discordgo

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// customRateLimit holds information for defining a custom rate limit
type customRateLimit struct {
	suffix   string
	requests int
	reset    time.Duration
}

// RateLimiter holds all ratelimit buckets
type RateLimiter struct {
	sync.Mutex
	global           *int64
	buckets          map[string]*Bucket
	customRateLimits []*customRateLimit

	MaxConcurrentRequests int
	numConcurrentLocks    *int32
}

// NewRatelimiter returns a new RateLimiter
func NewRatelimiter() *RateLimiter {

	return &RateLimiter{
		buckets:            make(map[string]*Bucket),
		global:             new(int64),
		numConcurrentLocks: new(int32),

		// with higher precision ratelimit headers enabled, this is no longer needed
		// customRateLimits: []*customRateLimit{
		// 	&customRateLimit{
		// 		suffix:   "/reactions//",
		// 		requests: 1,
		// 		reset:    250 * time.Millisecond,
		// 	},
		// },
	}
}

func (r *RateLimiter) CurrentConcurrentLocks() int {
	return int(atomic.LoadInt32(r.numConcurrentLocks))
}

// GetBucket retrieves or creates a bucket
func (r *RateLimiter) GetBucket(key string) *Bucket {
	r.Lock()
	defer r.Unlock()

	if bucket, ok := r.buckets[key]; ok {
		return bucket
	}

	b := &Bucket{
		Remaining: 1,
		Key:       key,
		global:    r.global,
		sem:       make(chan struct{}, 1), // Semaphore to serialize requests per bucket
	}

	if r.MaxConcurrentRequests > 0 {
		b.numConcurrentLocks = r.numConcurrentLocks
	}

	// Check if there is a custom ratelimit set for this bucket ID.
	for _, rl := range r.customRateLimits {
		if strings.HasSuffix(b.Key, rl.suffix) {
			b.customRateLimit = rl
			break
		}
	}

	r.buckets[key] = b
	return b
}

// GetWaitTime returns the duration you should wait for a Bucket
func (r *RateLimiter) GetWaitTime(b *Bucket, minRemaining int) time.Duration {
	// If we ran out of calls and the reset time is still ahead of us
	// then we need to take it easy and relax a little

	wait := time.Duration(0)
	if b.Remaining < minRemaining && b.reset.After(time.Now()) {
		wait = time.Until(b.reset)
	}

	// Check for global ratelimits
	sleepTo := time.Unix(0, atomic.LoadInt64(r.global))
	if now := time.Now(); now.Before(sleepTo) {

		// time until the global ratelimit is over
		globalWait := sleepTo.Sub(now)

		if globalWait < wait {
			// if the per route wait time is greater than the global wait time
			// return the per route wait time
			return wait
		}

		// otherwise return the global wait
		return globalWait
	}

	// either 0 or the per route wait time
	return wait
}

// LockBucket Locks until a request can be made
func (r *RateLimiter) LockBucket(bucketID string) *Bucket {
	bucket := r.GetBucket(bucketID)
	r.LockBucketObject(bucket)
	return bucket
}

// LockBucketObject waits for any rate limits on the bucket, acquires the per-bucket
// semaphore, decrements remaining, and returns. The semaphore serializes requests
// to prevent racing 429s, while a timeout prevents deadlock if a request hangs.
func (r *RateLimiter) LockBucketObject(b *Bucket) {
	// First, acquire the per-bucket semaphore to serialize requests.
	// Use a timeout to prevent deadlock if a request never completes.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	select {
	case b.sem <- struct{}{}:
		// Acquired semaphore, proceed
	case <-ctx.Done():
		// Timeout waiting for semaphore - proceed anyway to prevent deadlock
		// This indicates a previous request is hanging
		logrus.Warnf("WARNING: Semaphore timeout (30s) for bucket %s - previous request may be hanging", b.Key)
	}

	b.Lock()

	if wait := r.GetWaitTime(b, 1); wait > 0 {
		time.Sleep(wait)
	}

	didWaitForMaxCCR := false
	if r.MaxConcurrentRequests > 0 {
		// sleep until were below the maximum
		for {
			numNow := atomic.AddInt32(r.numConcurrentLocks, 1)
			if int(numNow) > r.MaxConcurrentRequests {
				atomic.AddInt32(r.numConcurrentLocks, -1)
				didWaitForMaxCCR = true
				time.Sleep(time.Millisecond * 25)
			} else {
				break
			}
		}
	}

	if didWaitForMaxCCR {
		// If things changed while waiting for max ccr (like a global ratelimit)
		if wait := r.GetWaitTime(b, 1); wait > 0 {
			time.Sleep(wait)
		}
	}

	b.Remaining--
	b.Unlock() // Release mutex before HTTP request (semaphore still held)
}

func (r *RateLimiter) SetGlobalTriggered(to time.Time) {
	atomic.StoreInt64(r.global, to.UnixNano())
}

// Bucket represents a ratelimit bucket, each bucket gets ratelimited individually (-global ratelimits)
type Bucket struct {
	sync.Mutex
	Key                string
	Remaining          int
	reset              time.Time
	global             *int64
	numConcurrentLocks *int32
	sem                chan struct{} // Semaphore to serialize requests per bucket

	lastReset       time.Time
	customRateLimit *customRateLimit
	Userdata        interface{}
}

// Release updates the bucket's ratelimit info from response headers.
// This acquires the lock, updates state, then releases the lock and semaphore.
func (b *Bucket) Release(headers http.Header) error {
	b.Lock()
	defer b.Unlock()

	// Release the per-bucket semaphore to allow next request
	select {
	case <-b.sem:
		// Released semaphore
	default:
		// Semaphore wasn't held (timeout case in LockBucketObject)
	}

	if b.numConcurrentLocks != nil {
		atomic.AddInt32(b.numConcurrentLocks, -1)
	}

	// Check if the bucket uses a custom ratelimiter
	if rl := b.customRateLimit; rl != nil {
		if time.Since(b.lastReset) >= rl.reset {
			b.Remaining = rl.requests - 1
			b.lastReset = time.Now()
		}
		if b.Remaining < 1 {
			b.reset = time.Now().Add(rl.reset)
		}
		return nil
	}

	if headers == nil {
		return nil
	}

	// X-RateLimit-Reset is a fixed point in time, unix time in seconds (represented as a float with millisecond precision if that's enabled)
	// while X-RateLimit-Reset-After is a duration after the current time the ratelimit resets in seconds (represented as a float with millisecond precision if that's enabled)
	// reset-after is a newer addition, as well as millisecond precision
	//
	// The original implementation used the fixed time "reset" header and the date header to avoid any need to synchronize the system time, but since the date header
	// does not provide millisecond precision, it's somewhat inaccurate for fast ratelimits (such as the react ones with 250ms/1)
	// The reset-after was found to be more reliable, even my dev case from europe over wifi, so that's why i switched to that.
	//
	// You could argue that syncronizing the system time and using the fixed time "reset" header instead is more reliable
	// but if your time becomes out of sync by just 1 second you will be hit with a wave of 429's and with discords new strict limits on those,
	// you could very well get temp banned easily for a hour if you're a big bot, making thousands of requests every minute.
	//
	// So reset-after is the best and most reliable choice here, at-least as a default.
	resetAfter := headers.Get("X-RateLimit-Reset-After")

	// Update global and per bucket reset time if the proper headers are available
	// If global is set, then it will block all buckets until after Retry-After
	// If Retry-After without global is provided it will use that for the new reset
	// time since it's more accurate than X-RateLimit-Reset.
	retryAfter := headers.Get("Retry-After")
	if retryAfter != "" {

		dur, err := parseResetAfterDur(retryAfter)
		if err != nil {
			return err
		}

		resetAt := time.Now().Add(dur)

		// Lock either this single bucket or all buckets
		global := headers.Get("X-RateLimit-Global")
		if global != "" {
			atomic.StoreInt64(b.global, resetAt.UnixNano())
		} else {
			b.reset = resetAt
		}
	} else if resetAfter != "" {
		dur, err := parseResetAfterDur(resetAfter)
		if err != nil {
			return err
		}

		b.reset = time.Now().Add(dur)
	}

	// Udpate remaining if header is present
	remaining := headers.Get("X-RateLimit-Remaining")
	if remaining != "" {
		parsedRemaining, err := strconv.ParseInt(remaining, 10, 32)
		if err != nil {
			return err
		}
		b.Remaining = int(parsedRemaining)
	}

	return nil
}

func parseResetAfterDur(in string) (time.Duration, error) {
	resetAfterParsed, err := strconv.ParseFloat(in, 64)
	if err != nil {
		return 0, err
	}

	return time.Millisecond * time.Duration(resetAfterParsed*1000), nil
}
