package httpapi

import (
	"sync"
	"time"
)

const defaultMaxRateBuckets = 4096

type rateBucket struct {
	tokens float64
	last   time.Time
}

type rateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]rateBucket
	capacity float64
	refill   float64
	maxKeys  int
	now      func() time.Time
}

func newRateLimiter(capacity int, refillEvery time.Duration, now func() time.Time) *rateLimiter {
	if now == nil {
		now = time.Now
	}
	return &rateLimiter{
		buckets: make(map[string]rateBucket), capacity: float64(capacity),
		refill: 1 / refillEvery.Seconds(), maxKeys: defaultMaxRateBuckets, now: now,
	}
}

func (l *rateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	bucket, ok := l.buckets[key]
	if !ok {
		bucket = rateBucket{tokens: l.capacity, last: now}
	}
	bucket.tokens += now.Sub(bucket.last).Seconds() * l.refill
	if bucket.tokens > l.capacity {
		bucket.tokens = l.capacity
	}
	bucket.last = now
	allowed := bucket.tokens >= 1
	if allowed {
		bucket.tokens--
	}
	l.buckets[key] = bucket
	if len(l.buckets) > l.maxKeys {
		l.prune(now.Add(-time.Hour), l.maxKeys, key)
	}
	return allowed
}

func (l *rateLimiter) prune(before time.Time, limit int, preserve string) {
	for key, bucket := range l.buckets {
		if bucket.last.Before(before) {
			delete(l.buckets, key)
		}
	}
	for key := range l.buckets {
		if len(l.buckets) <= limit {
			break
		}
		if key != preserve {
			delete(l.buckets, key)
		}
	}
}
