package limiter

import (
	"net"
	"net/http"
	"strings"
	"sync"
)

type ConcurrencyLimiter struct {
	mu     sync.Mutex
	counts map[string]int
}

func NewConcurrencyLimiter() *ConcurrencyLimiter {
	return &ConcurrencyLimiter{
		counts: make(map[string]int),
	}
}

func (c *ConcurrencyLimiter) Acquire(keyID string, maxConcurrency int) bool {
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	current := c.counts[keyID]
	if current >= maxConcurrency {
		return false
	}

	c.counts[keyID] = current + 1
	return true
}

func (c *ConcurrencyLimiter) Release(keyID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	current := c.counts[keyID]
	if current <= 1 {
		delete(c.counts, keyID)
	} else {
		c.counts[keyID] = current - 1
	}
}

func (c *ConcurrencyLimiter) Current(keyID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[keyID]
}

// IP Helper: normalizes client IP, handling proxies safely
func ExtractClientIP(r *http.Request, trustedProxies []string) string {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}

	// Check if direct caller is a trusted proxy
	isTrusted := false
	for _, tp := range trustedProxies {
		if remoteHost == tp || tp == "*" {
			isTrusted = true
			break
		}
	}

	if isTrusted {
		// Only trust X-Forwarded-For if caller is trusted
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				client := strings.TrimSpace(parts[0])
				if ip := net.ParseIP(client); ip != nil {
					return ip.String()
				}
			}
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			client := strings.TrimSpace(xri)
			if ip := net.ParseIP(client); ip != nil {
				return ip.String()
			}
		}
	}

	if ip := net.ParseIP(remoteHost); ip != nil {
		return ip.String()
	}
	return remoteHost
}
