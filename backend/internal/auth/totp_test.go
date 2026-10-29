package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// RFC 6238 appendix B, SHA-1 vectors, truncated to six digits.
func TestTOTPCodeRFC6238(t *testing.T) {
	secret := []byte("12345678901234567890")
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	}
	for _, c := range cases {
		if got := totpCode(secret, totpStep(time.Unix(c.unix, 0))); got != c.want {
			t.Errorf("code at %d = %s, want %s", c.unix, got, c.want)
		}
	}
}

func TestMatchTOTP(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Unix(1111111111, 0)
	cur := totpStep(now)

	if s, ok := matchTOTP(secret, "050 471", now, 0); !ok || s != cur {
		t.Fatalf("current code with a space refused (%d, %v)", s, ok)
	}
	if _, ok := matchTOTP(secret, totpCode(secret, cur-1), now, 0); !ok {
		t.Fatal("previous step refused: a phone 30 seconds slow must still work")
	}
	if _, ok := matchTOTP(secret, totpCode(secret, cur+2), now, 0); ok {
		t.Fatal("code two steps ahead accepted")
	}
	if _, ok := matchTOTP(secret, "050471", now, cur); ok {
		t.Fatal("a code was accepted twice")
	}
	if _, ok := matchTOTP(secret, "12345", now, 0); ok {
		t.Fatal("five digits accepted")
	}
}

func TestSecretBoxBindsToUser(t *testing.T) {
	box, err := newSecretBox(make([]byte, TOTPKeySize))
	if err != nil {
		t.Fatal(err)
	}
	alice, bob := uuid.New(), uuid.New()
	sealed := box.seal(alice, []byte("secret"))
	if got, err := box.open(alice, sealed); err != nil || string(got) != "secret" {
		t.Fatalf("open = %q, %v", got, err)
	}
	if _, err := box.open(bob, sealed); err == nil {
		t.Fatal("a secret sealed for one user opened for another")
	}
	if _, err := newSecretBox(make([]byte, 16)); err == nil {
		t.Fatal("short key accepted")
	}
}

func TestRecoveryCodes(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		c := newRecoveryCode()
		if len(c) != 14 || c[4] != '-' || c[9] != '-' {
			t.Fatalf("bad format %q", c)
		}
		if strings.ContainsAny(c, "il01oO") {
			t.Fatalf("look-alike character in %q", c)
		}
		if seen[c] {
			t.Fatalf("repeated code %q", c)
		}
		seen[c] = true
		if !looksLikeRecoveryCode(c) || looksLikeRecoveryCode("123456") {
			t.Fatal("recovery code not told apart from a six-digit code")
		}
	}
	if string(recoveryCodeHash("ABCD-efgh-2345")) != string(recoveryCodeHash(" abcdefgh2345 ")) {
		t.Fatal("case, dashes or spaces change the hash")
	}
}

func TestOTPAuthURI(t *testing.T) {
	// The RFC 6238 example key. Its base32 form is checked by formatSecret
	// below, so it is not written out here as one long string.
	rfcKey := []byte("12345678901234567890")
	u := otpauthURI(rfcKey, "a.person@example.invalid")
	if !strings.HasPrefix(u, "otpauth://totp/Dakota%20Training%20College:a.person@example.invalid?") ||
		!strings.Contains(u, "secret="+strings.ReplaceAll(formatSecret(rfcKey), " ", "")) || !strings.Contains(u, "issuer=Dakota+Training+College") {
		t.Fatalf("uri = %s", u)
	}
	if got := formatSecret([]byte("12345678901234567890")); got != "GEZD GNBV GY3T QOJQ GEZD GNBV GY3T QOJQ" {
		t.Fatalf("formatted = %q", got)
	}
}
