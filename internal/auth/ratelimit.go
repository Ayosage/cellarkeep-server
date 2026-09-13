package auth

import (
	"sync"
	"time"
)

// Limiter is a fixed-window in-memory counter. One process, one cellar: this
// is enough for a self-hosted instance and needs no extra service.
type Limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string]entry
	now    func() time.Time
}

type entry struct {
	count int
	reset time.Time
}

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, hits: map[string]entry{}, now: time.Now}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.hits[key]
	if !ok || now.After(e.reset) {
		l.hits[key] = entry{count: 1, reset: now.Add(l.window)}
		return true
	}
	if e.count >= l.max {
		return false
	}
	e.count++
	l.hits[key] = e
	return true
}
