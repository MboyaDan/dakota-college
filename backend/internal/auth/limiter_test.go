package auth

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	l := NewLimiter(3, time.Minute, func() time.Time { return now })

	for i := 0; i < 3; i++ {
		if !l.Allow("a") {
			t.Fatalf("event %d refused", i+1)
		}
	}
	if l.Allow("a") {
		t.Fatal("fourth event allowed")
	}
	if !l.Allow("b") {
		t.Fatal("other key affected")
	}

	now = now.Add(61 * time.Second)
	if !l.Allow("a") {
		t.Fatal("not allowed after the window passed")
	}

	l.Allow("a")
	l.Allow("a")
	l.Reset("a")
	if !l.Allow("a") {
		t.Fatal("not allowed after Reset")
	}
}

func TestNewTokenIsRandomAndHashed(t *testing.T) {
	a, ha := NewToken()
	b, hb := NewToken()
	if a == b || string(ha) == string(hb) {
		t.Fatal("tokens repeat")
	}
	if len(a) < 43 || len(ha) != 32 {
		t.Fatalf("token %d chars, hash %d bytes", len(a), len(ha))
	}
	if string(HashToken(a)) != string(ha) {
		t.Fatal("HashToken does not match NewToken's hash")
	}
}
