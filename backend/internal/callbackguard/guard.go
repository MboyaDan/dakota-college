// Package callbackguard decides whether a payment provider's callback is
// accepted (D-170, D-173): the path must hold the configured secret
// (compared in constant time) and, if an allow list is set, the caller must
// be on it. An address with too many refusals in a short window is refused
// outright, so the secret cannot be guessed by trying.
package callbackguard

import (
	"crypto/subtle"
	"errors"
	"net/netip"
	"sync"
	"time"
)

var (
	// ErrUnauthorised: wrong secret or address.
	ErrUnauthorised = errors.New("callback not accepted")
	// ErrThrottled: too many refused attempts from this address lately.
	ErrThrottled = errors.New("too many refused callbacks")
)

const (
	MaxRefused    = 20
	RefusedWindow = 10 * time.Minute
)

type Guard struct {
	secret  string
	allowed []netip.Prefix
	now     func() time.Time

	mu      sync.Mutex
	refused map[string][]time.Time
}

// New returns a guard for secret and the allow list (IPs or CIDRs; invalid
// entries are ignored here, config refuses them at start-up).
func New(secret string, allowedIPs []string, now func() time.Time) *Guard {
	if now == nil {
		now = time.Now
	}
	g := &Guard{secret: secret, now: now, refused: map[string][]time.Time{}}
	for _, a := range allowedIPs {
		if p, err := netip.ParsePrefix(a); err == nil {
			g.allowed = append(g.allowed, p)
		} else if ip, err := netip.ParseAddr(a); err == nil {
			g.allowed = append(g.allowed, netip.PrefixFrom(ip, ip.BitLen()))
		}
	}
	return g
}

// HasAllowList reports whether callers are limited to listed addresses.
func (g *Guard) HasAllowList() bool { return len(g.allowed) > 0 }

// Check accepts or refuses one call. key is the address used for throttling.
func (g *Guard) Check(secret string, ip *netip.Addr) (key string, err error) {
	key = "unknown"
	if ip != nil {
		key = ip.Unmap().String()
	}
	now := g.now()
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.recent(key, now)) >= MaxRefused {
		return key, ErrThrottled
	}
	if subtle.ConstantTimeCompare([]byte(secret), []byte(g.secret)) == 1 && g.listed(ip) {
		return key, nil
	}
	g.refused[key] = append(g.recent(key, now), now)
	return key, ErrUnauthorised
}

func (g *Guard) listed(ip *netip.Addr) bool {
	if len(g.allowed) == 0 {
		return true
	}
	if ip == nil {
		return false
	}
	for _, p := range g.allowed {
		if p.Contains(ip.Unmap()) {
			return true
		}
	}
	return false
}

// recent keeps the refusals inside the window (caller holds the lock).
func (g *Guard) recent(key string, now time.Time) []time.Time {
	kept := g.refused[key][:0]
	for _, t := range g.refused[key] {
		if now.Sub(t) < RefusedWindow {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(g.refused, key)
		return nil
	}
	g.refused[key] = kept
	return kept
}
