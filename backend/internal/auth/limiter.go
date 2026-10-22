package auth

import (
	"sync"
	"time"
)

// Limiter allows at most Max events per key in a fixed Window. It is
// in-memory, so it limits per API instance; the database lockout is the
// durable control (one instance is planned for production, see D-46).
type Limiter struct {
	Max    int
	Window time.Duration
	now    func() time.Time

	mu      sync.Mutex
	windows map[string]*window
}

type window struct {
	count   int
	resetAt time.Time
}

func NewLimiter(max int, w time.Duration, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{Max: max, Window: w, now: now, windows: map[string]*window{}}
}

// Allow records one event for key and reports whether it is within the limit.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()

	if len(l.windows) > 10_000 {
		for k, w := range l.windows {
			if now.After(w.resetAt) {
				delete(l.windows, k)
			}
		}
	}

	w, ok := l.windows[key]
	if !ok || now.After(w.resetAt) {
		w = &window{resetAt: now.Add(l.Window)}
		l.windows[key] = w
	}
	w.count++
	return w.count <= l.Max
}

// Reset forgets key, e.g. after a successful login.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.windows, key)
}
