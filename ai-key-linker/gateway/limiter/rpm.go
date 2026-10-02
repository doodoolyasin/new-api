package limiter

import (
	"sync"
	"time"
)

type bucket struct {
	tokens     float64
	capacity   float64
	fillRate   float64 // tokens per second
	lastUpdate time.Time
}

type RPMLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

func NewRPMLimiter() *RPMLimiter {
	limiter := &RPMLimiter{
		buckets: make(map[string]*bucket),
	}
	// Background cleanup of inactive buckets every 5 minutes
	go limiter.cleanupLoop(5*time.Minute, 15*time.Minute)
	return limiter
}

func (l *RPMLimiter) Allow(keyID string, rpm int) (bool, time.Duration) {
	if rpm <= 0 {
		return false, time.Minute
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, exists := l.buckets[keyID]
	rate := float64(rpm) / 60.0 // tokens per second
	cap := float64(rpm)

	if !exists {
		// New bucket starts with full capacity minus 1 for current request
		l.buckets[keyID] = &bucket{
			tokens:     cap - 1.0,
			capacity:   cap,
			fillRate:   rate,
			lastUpdate: now,
		}
		return true, 0
	}

	// Refill based on elapsed time
	elapsed := now.Sub(b.lastUpdate).Seconds()
	b.lastUpdate = now
	b.tokens += elapsed * b.fillRate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}

	// Dynamic capacity/rate change if rpm updated
	if b.capacity != cap {
		b.capacity = cap
		b.fillRate = rate
		if b.tokens > cap {
			b.tokens = cap
		}
	}

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true, 0
	}

	// Calculate wait time until 1 token is available
	missing := 1.0 - b.tokens
	waitSeconds := missing / b.fillRate
	if waitSeconds < 1.0 {
		waitSeconds = 1.0
	}
	return false, time.Duration(waitSeconds * float64(time.Second))
}

func (l *RPMLimiter) cleanupLoop(interval, maxIdle time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for k, b := range l.buckets {
			if now.Sub(b.lastUpdate) > maxIdle {
				delete(l.buckets, k)
			}
		}
		l.mu.Unlock()
	}
}
