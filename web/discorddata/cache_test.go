package discorddata

import (
	"testing"
	"time"
)

func TestFetchEvictedWhileInFlightIsNotCached(t *testing.T) {
	const key = "test:evicted-in-flight"

	value, err := fetchCached(key, time.Minute, func() (any, error) {
		evictCached(key)
		return "stale", nil
	})
	if err != nil || value != "stale" {
		t.Fatalf("expected the fetched value to be returned, got %v %v", value, err)
	}

	value, _ = fetchCached(key, time.Minute, func() (any, error) {
		return "fresh", nil
	})
	if value != "fresh" {
		t.Fatalf("value fetched before the eviction should not have been cached, got %v", value)
	}

	value, _ = fetchCached(key, time.Minute, func() (any, error) {
		return "refetched", nil
	})
	if value != "fresh" {
		t.Fatalf("value fetched after the eviction should be cached, got %v", value)
	}
}
