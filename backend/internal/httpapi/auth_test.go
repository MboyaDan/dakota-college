package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestLoginSessionLogout(t *testing.T) {
	h := newHarness(t)
	h.createUser("editor@example.invalid", "content_editor")

	s, w := h.login("Editor@Example.invalid ", testPassword)
	if s == nil {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	c := s.cookie
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Domain != "" {
		t.Fatalf("cookie attributes %+v; want HttpOnly, Secure, SameSite=Lax, Path=/, no Domain", c)
	}
	if !strings.Contains(w.Body.String(), `"permissions":["content.read","content.write"]`) {
		t.Fatalf("login body: %s", w.Body.String())
	}

	w = h.do(req{method: http.MethodGet, path: "/api/v1/auth/session", sess: s})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "editor@example.invalid") || !strings.Contains(w.Body.String(), s.csrf) {
		t.Fatalf("session: %d %s", w.Code, w.Body.String())
	}

	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/logout", sess: s, noCSRF: true}), http.StatusForbidden, "csrf_failed")
	if w := h.do(req{method: http.MethodPost, path: "/api/v1/auth/logout", sess: s}); w.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", w.Code, w.Body.String())
	}
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/auth/session", sess: s}), http.StatusUnauthorized, "unauthenticated")

	if h.auditCount("auth.login_succeeded") != 1 || h.auditCount("auth.logout") != 1 {
		t.Fatal("login and logout were not audited")
	}
}

func TestLoginFailuresLookTheSame(t *testing.T) {
	h := newHarness(t)
	h.createUser("staff@example.invalid", "trainer")
	disabled := h.createUser("disabled@example.invalid", "trainer")
	h.exec(`UPDATE users SET status = 'disabled' WHERE id = $1`, disabled)

	_, wrongPassword := h.login("staff@example.invalid", "not the password at all")
	_, unknownEmail := h.login("nobody@example.invalid", testPassword)
	_, disabledUser := h.login("disabled@example.invalid", testPassword)
	message := func(w *httptest.ResponseRecorder) string {
		var b errorBody
		_ = json.Unmarshal(w.Body.Bytes(), &b)
		return fmt.Sprintf("%d %s %s", w.Code, b.Error.Code, b.Error.Message)
	}
	want := message(wrongPassword)
	if !strings.HasPrefix(want, "401 invalid_credentials") || message(unknownEmail) != want || message(disabledUser) != want {
		t.Fatalf("responses differ:\n%s\n%s\n%s", want, message(unknownEmail), message(disabledUser))
	}
	if n := h.auditCount("auth.login_failed"); n != 3 {
		t.Fatalf("%d failed logins audited, want 3", n)
	}
	var leaked int
	_ = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE details::text LIKE '%not the password%'`).Scan(&leaked)
	if leaked != 0 {
		t.Fatal("a password reached the audit log")
	}
}

func TestLockoutAfterFiveWrongPasswords(t *testing.T) {
	h := newHarness(t)
	id := h.createUser("staff@example.invalid", "trainer")

	for i := 1; i <= 4; i++ {
		_, w := h.login("staff@example.invalid", "wrong password number")
		expectStatus(t, w, http.StatusUnauthorized, "invalid_credentials")
	}
	_, w := h.login("staff@example.invalid", "wrong password number")
	expectStatus(t, w, http.StatusTooManyRequests, "too_many_attempts")

	// The correct password is refused while locked, even after an API restart
	// (the lock is in the database, not only in memory).
	h.rebuild()
	_, w = h.login("staff@example.invalid", testPassword)
	expectStatus(t, w, http.StatusTooManyRequests, "too_many_attempts")

	h.exec(`UPDATE users SET locked_until = now() - interval '1 second' WHERE id = $1`, id)
	h.rebuild()
	h.mustLogin("staff@example.invalid")
	var failures int
	_ = h.pool.QueryRow(context.Background(), `SELECT failed_login_count FROM users WHERE id = $1`, id).Scan(&failures)
	if failures != 0 {
		t.Fatalf("failed_login_count = %d after success, want 0", failures)
	}
}

func TestRateLimitByEmailIncludesUnknownAccounts(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 5; i++ {
		_, w := h.login("ghost@example.invalid", testPassword)
		expectStatus(t, w, http.StatusUnauthorized, "invalid_credentials")
	}
	_, w := h.login("ghost@example.invalid", testPassword)
	expectStatus(t, w, http.StatusTooManyRequests, "too_many_attempts")
}

func TestSessionIdleAndAbsoluteExpiry(t *testing.T) {
	h := newHarness(t)
	h.createUser("staff@example.invalid", "trainer")

	idle := h.mustLogin("staff@example.invalid")
	h.exec(`UPDATE sessions SET last_seen_at = now() - interval '31 minutes'`)
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/auth/session", sess: idle}), http.StatusUnauthorized, "unauthenticated")
	var revoked int
	_ = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE revoked_at IS NOT NULL`).Scan(&revoked)
	if revoked != 1 {
		t.Fatalf("idle session not revoked (%d revoked)", revoked)
	}

	old := h.mustLogin("staff@example.invalid")
	h.exec(`UPDATE sessions SET expires_at = now() - interval '1 second', created_at = now() - interval '13 hours' WHERE revoked_at IS NULL`)
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/auth/session", sess: old}), http.StatusUnauthorized, "unauthenticated")
}

func TestCSRFAndOriginChecks(t *testing.T) {
	h := newHarness(t)
	h.createUser("staff@example.invalid", "trainer")
	s := h.mustLogin("staff@example.invalid")

	wrong := &session{cookie: s.cookie, csrf: "not-the-token"}
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/logout", sess: wrong}), http.StatusForbidden, "csrf_failed")

	// A token from another session does not work either.
	other := h.mustLogin("staff@example.invalid")
	stolen := &session{cookie: s.cookie, csrf: other.csrf}
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/logout", sess: stolen}), http.StatusForbidden, "csrf_failed")

	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/logout", sess: s, headers: map[string]string{"Origin": "https://evil.example"}}), http.StatusForbidden, "cross_site_request")
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/logout", sess: s, headers: map[string]string{"Sec-Fetch-Site": "cross-site"}}), http.StatusForbidden, "cross_site_request")
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/login", body: map[string]string{"email": "staff@example.invalid", "password": testPassword}, headers: map[string]string{"Origin": "https://evil.example"}}), http.StatusForbidden, "cross_site_request")

	if w := h.do(req{method: http.MethodPost, path: "/api/v1/auth/logout", sess: s, headers: map[string]string{"Origin": testOrigin, "Sec-Fetch-Site": "same-origin"}}); w.Code != http.StatusNoContent {
		t.Fatalf("same-origin logout: %d %s", w.Code, w.Body.String())
	}
}

var resetLink = regexp.MustCompile(`/reset-password\?token=([A-Za-z0-9_-]+)`)

func (h *harness) waitForResetToken(to string, count int) string {
	h.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var found []string
		for _, m := range h.mailer.All() {
			if m.To == to {
				found = append(found, m.Text)
			}
		}
		if len(found) >= count {
			match := resetLink.FindStringSubmatch(found[count-1])
			if match == nil {
				h.t.Fatalf("no reset link in email: %s", found[count-1])
			}
			return match[1]
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("no reset email %d to %s", count, to)
	return ""
}

func TestPasswordReset(t *testing.T) {
	h := newHarness(t)
	h.createUser("staff@example.invalid", "trainer")
	existing := h.mustLogin("staff@example.invalid")

	known := h.do(req{method: http.MethodPost, path: "/api/v1/auth/password-reset", body: map[string]string{"email": "staff@example.invalid"}})
	unknown := h.do(req{method: http.MethodPost, path: "/api/v1/auth/password-reset", body: map[string]string{"email": "nobody@example.invalid"}})
	if known.Code != http.StatusAccepted || unknown.Code != http.StatusAccepted || known.Body.String() != unknown.Body.String() {
		t.Fatalf("reset responses differ: %d %s / %d %s", known.Code, known.Body, unknown.Code, unknown.Body)
	}
	token := h.waitForResetToken("staff@example.invalid", 1)
	time.Sleep(100 * time.Millisecond)
	for _, m := range h.mailer.All() {
		if m.To == "nobody@example.invalid" {
			t.Fatal("an email went to an address with no account")
		}
	}

	confirm := func(tok, pw string) int {
		return h.do(req{method: http.MethodPost, path: "/api/v1/auth/password-reset/confirm", body: map[string]string{"token": tok, "password": pw}}).Code
	}
	if c := confirm(token, "short"); c != http.StatusUnprocessableEntity {
		t.Fatalf("short password: %d", c)
	}
	const newPassword = "a brand new long passphrase"
	if c := confirm(token, newPassword); c != http.StatusNoContent {
		t.Fatalf("confirm: %d", c)
	}
	// Reset signs the user out everywhere, spends the link, and changes the password.
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/auth/session", sess: existing}), http.StatusUnauthorized, "unauthenticated")
	if c := confirm(token, "another long passphrase"); c != http.StatusBadRequest {
		t.Fatalf("reused link: %d", c)
	}
	if s, _ := h.login("staff@example.invalid", testPassword); s != nil {
		t.Fatal("old password still works")
	}
	if s, w := h.login("staff@example.invalid", newPassword); s == nil {
		t.Fatalf("new password refused: %s", w.Body.String())
	}

	h.do(req{method: http.MethodPost, path: "/api/v1/auth/password-reset", body: map[string]string{"email": "staff@example.invalid"}})
	expired := h.waitForResetToken("staff@example.invalid", 2)
	h.exec(`UPDATE password_resets SET expires_at = now() - interval '1 second', created_at = now() - interval '2 hours' WHERE used_at IS NULL`)
	if c := confirm(expired, "yet another long passphrase"); c != http.StatusBadRequest {
		t.Fatalf("expired link: %d", c)
	}
	if h.auditCount("auth.password_reset_completed") != 1 || h.auditCount("auth.password_reset_requested") != 2 {
		t.Fatal("password reset was not audited")
	}
}
