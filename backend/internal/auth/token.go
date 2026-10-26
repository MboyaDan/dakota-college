package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// NewToken returns a random 256-bit token (URL-safe) and its SHA-256 hash.
// Only the hash is stored, so a database leak does not reveal live tokens.
func NewToken() (token string, hash []byte) {
	var b [32]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error
	token = base64.RawURLEncoding.EncodeToString(b[:])
	return token, HashToken(token)
}

func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// SetupLinkTTL is how long the link in an enrolment email works (D-141).
const SetupLinkTTL = 7 * 24 * time.Hour

// ResetPath is the site path of a password link. Students are sent back to
// the student portal once they have chosen a password.
func ResetPath(token, kind string) string {
	p := "/reset-password?token=" + token
	if kind == "student" {
		p += "&portal=student"
	}
	return p
}

// IssueSetupLink records a one-time link for userID to choose a first
// password, inside the caller's transaction, and returns its site path. It
// is an ordinary password reset that lasts SetupLinkTTL.
func IssueSetupLink(ctx context.Context, q *dbq.Queries, userID uuid.UUID, now time.Time) (string, error) {
	token, hash := NewToken()
	if err := q.CreatePasswordReset(ctx, dbq.CreatePasswordResetParams{UserID: userID, TokenHash: hash, ExpiresAt: now.Add(SetupLinkTTL)}); err != nil {
		return "", err
	}
	return ResetPath(token, "student"), nil
}
