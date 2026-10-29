package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 authenticator apps use HMAC-SHA1; it is not used as a plain hash.
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Time-based one-time codes (RFC 6238) as every common authenticator app
// makes them: HMAC-SHA1, six digits, a new code every 30 seconds.
const (
	totpPeriod = 30
	totpDigits = 6
	// totpSkew accepts the code before and after the current one, for a
	// phone clock up to 30 seconds out.
	totpSkew = 1
	// TOTPKeySize is the length of TOTP_KEY in bytes.
	TOTPKeySize    = 32
	totpSecretSize = 20
	totpIssuer     = "Dakota Training College"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// totpStep is the 30-second step that t falls in.
func totpStep(t time.Time) int64 { return t.Unix() / totpPeriod }

// totpCode is the code for one step (RFC 4226 dynamic truncation).
func totpCode(secret []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step)) //nolint:gosec // steps are positive
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", totpDigits, n%1_000_000)
}

// CodeFor is the authenticator code at t for a secret as shown at setup
// (base32, spaces allowed). Tests and support tools use it.
func CodeFor(secret string, t time.Time) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil {
		return "", err
	}
	return totpCode(key, totpStep(t)), nil
}

// matchTOTP returns the step a code belongs to, if it is the code for now
// or the step either side and newer than lastStep (so a code that has been
// used cannot be used again).
func matchTOTP(secret []byte, code string, now time.Time, lastStep int64) (int64, bool) {
	code = normaliseCode(code)
	if len(code) != totpDigits {
		return 0, false
	}
	cur := totpStep(now)
	for s := cur - totpSkew; s <= cur+totpSkew; s++ {
		if s <= lastStep {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(totpCode(secret, s)), []byte(code)) == 1 {
			return s, true
		}
	}
	return 0, false
}

// normaliseCode drops the spaces and dashes people type or paste.
func normaliseCode(code string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' {
			return -1
		}
		return r
	}, strings.TrimSpace(code))
}

// otpauthURI is what the QR code holds, so an authenticator app can add the
// account in one scan.
func otpauthURI(secret []byte, account string) string {
	label := url.PathEscape(totpIssuer + ":" + account)
	q := url.Values{}
	q.Set("secret", b32.EncodeToString(secret))
	q.Set("issuer", totpIssuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(totpDigits))
	q.Set("period", fmt.Sprint(totpPeriod))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// formatSecret groups the secret in fours, for typing into an app by hand.
func formatSecret(secret []byte) string {
	s := b32.EncodeToString(secret)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && i%4 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// secretBox seals authenticator secrets with TOTP_KEY. The sealed value is
// bound to the user, so it cannot be copied onto another account.
type secretBox struct{ aead cipher.AEAD }

func newSecretBox(key []byte) (*secretBox, error) {
	if len(key) != TOTPKeySize {
		return nil, fmt.Errorf("two-step key must be %d bytes", TOTPKeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &secretBox{aead: aead}, nil
}

func (b *secretBox) seal(userID uuid.UUID, secret []byte) []byte {
	nonce := make([]byte, b.aead.NonceSize(), b.aead.NonceSize()+len(secret)+b.aead.Overhead())
	_, _ = rand.Read(nonce) // crypto/rand.Read never returns an error
	return b.aead.Seal(nonce, nonce, secret, userID[:])
}

func (b *secretBox) open(userID uuid.UUID, sealed []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("stored two-step secret too short")
	}
	return b.aead.Open(nil, sealed[:n], sealed[n:], userID[:])
}

// Recovery codes: ten per person, each 12 characters from an alphabet
// without look-alike letters (60 bits), shown as xxxx-xxxx-xxxx.
const (
	RecoveryCodeCount = 10
	recoveryAlphabet  = "abcdefghjkmnpqrstuvwxyz23456789" // 31 characters: no i, l, o, 0, 1
	recoveryLength    = 12
)

func newRecoveryCode() string {
	var b [recoveryLength]byte
	_, _ = rand.Read(b[:])
	out := make([]byte, 0, recoveryLength+2)
	for i, v := range b {
		if i > 0 && i%4 == 0 {
			out = append(out, '-')
		}
		// 256 % 31 leaves a bias of under 1 in 8 per character: negligible
		// against 60 bits and a rate-limited sign-in.
		out = append(out, recoveryAlphabet[int(v)%len(recoveryAlphabet)])
	}
	return string(out)
}

// recoveryCodeHash is what is stored: the code without dashes, lower case.
func recoveryCodeHash(code string) []byte {
	return HashToken(strings.ToLower(normaliseCode(code)))
}

// looksLikeRecoveryCode tells a recovery code from a six-digit code.
func looksLikeRecoveryCode(code string) bool {
	return len(normaliseCode(code)) == recoveryLength
}
