package httpapi

import (
	"fmt"
	"testing"
	"time"
)

func TestRateLimiterBoundsUniqueClientState(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	limiter := newRateLimiter(2, time.Minute, func() time.Time { return now })
	limiter.maxKeys = 4
	for index := range 10 {
		now = now.Add(time.Second)
		limiter.Allow(fmt.Sprintf("client-%d", index))
	}
	if len(limiter.buckets) != limiter.maxKeys {
		t.Fatalf("rate limiter retained %d buckets, want %d", len(limiter.buckets), limiter.maxKeys)
	}
	if _, found := limiter.buckets["client-9"]; !found {
		t.Fatal("rate limiter pruned the newest client bucket")
	}
}
