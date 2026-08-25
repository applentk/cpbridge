package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type entry struct {
	count   int
	started time.Time
}

// Limiter is a bounded process-local fixed-window limiter. Multi-instance
// deployments should also enforce an equivalent limit at the edge.
type Limiter struct {
	mu      sync.Mutex
	entries map[string]entry
	maxKeys int
}

func New(maxKeys int) *Limiter {
	if maxKeys < 1 {
		maxKeys = 10000
	}
	return &Limiter{entries: make(map[string]entry), maxKeys: maxKeys}
}

func (l *Limiter) Allow(key string, limit int, window time.Duration, now time.Time) (bool, time.Duration) {
	if l == nil || limit < 1 || window <= 0 {
		return true, 0
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unknown"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.entries) >= l.maxKeys {
		for k, current := range l.entries {
			if now.Sub(current.started) >= window {
				delete(l.entries, k)
			}
		}
		if _, exists := l.entries[key]; !exists && len(l.entries) >= l.maxKeys {
			return false, window
		}
	}

	current, ok := l.entries[key]
	if !ok || now.Sub(current.started) >= window {
		l.entries[key] = entry{count: 1, started: now}
		return true, 0
	}
	if current.count >= limit {
		return false, window - now.Sub(current.started)
	}
	current.count++
	l.entries[key] = current
	return true, 0
}

func ClientIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if strings.TrimSpace(r.RemoteAddr) != "" {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return "unknown"
}
