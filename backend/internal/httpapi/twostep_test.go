package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MboyaDan/dakota-college/backend/internal/auth"
)

const stepCookieName = "__Host-dtc_signin_step"

// awayFromStepEdge waits while the clock is within two seconds of a new
// 30-second code, so a code made here is still current when the API checks it.
func awayFromStepEdge() {
	for s := time.Now().Unix() % 30; s < 2 || s > 27; s = time.Now().Unix() % 30 {
		time.Sleep(250 * time.Millisecond)
	}
}

func codeAt(t *testing.T, secret string, offset time.Duration) string {
	t.Helper()
	c, err := auth.CodeFor(secret, time.Now().Add(offset))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func decodeAs[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	return v
}

type twoStepStatusBody struct {
	Enabled           bool `json:"enabled"`
	RecoveryCodesLeft int  `json:"recovery_codes_left"`
	Recommended       bool `json:"recommended"`
}

// enableTwoStep turns two-step sign-in on for a signed-in person, using the
// code of the previous 30-second step, and returns the secret and codes.
func (h *harness) enableTwoStep(s *session) (secret string, codes []string) {
	h.t.Helper()
	w := h.do(req{method: http.MethodPost, path: "/api/v1/auth/two-step/setup", sess: s, body: map[string]string{"password": testPassword}})
	if w.Code != http.StatusOK {
		h.t.Fatalf("setup: %d %s", w.Code, w.Body.String())
	}
	setup := decodeAs[struct {
		Secret string `json:"secret"`
		URI    string `json:"otpauth_uri"`
	}](h.t, w)
	awayFromStepEdge()
	w = h.do(req{method: http.MethodPost, path: "/api/v1/auth/two-step/confirm", sess: s, body: map[string]string{"code": codeAt(h.t, setup.Secret, -30*time.Second)}})
	if w.Code != http.StatusOK {
		h.t.Fatalf("confirm: %d %s", w.Code, w.Body.String())
	}
	return setup.Secret, decodeAs[struct {
		Codes []string `json:"recovery_codes"`
	}](h.t, w).Codes
}

// passwordStep signs in with the password and returns the challenge cookie.
func (h *harness) passwordStep(email string) *http.Cookie {
	h.t.Helper()
	w := h.do(req{method: http.MethodPost, path: "/api/v1/auth/login", body: map[string]string{"email": email, "password": testPassword}})
	if w.Code != http.StatusAccepted {
		h.t.Fatalf("password step: %d %s", w.Code, w.Body.String())
	}
	var step *http.Cookie
	for _, c := range w.Result().Cookies() {
		switch c.Name {
		case "__Host-dtc_session":
			h.t.Fatal("a session cookie was set before the code")
		case stepCookieName:
			step = c
		}
	}
	if step == nil || !step.HttpOnly || !step.Secure || step.SameSite != http.SameSiteStrictMode {
		h.t.Fatalf("challenge cookie missing or weak: %+v", step)
	}
	return step
}

func (h *harness) codeStep(step *http.Cookie, code string) *httptest.ResponseRecorder {
	h.t.Helper()
	hr := map[string]string{}
	if step != nil {
		hr["Cookie"] = step.Name + "=" + step.Value
	}
	return h.do(req{method: http.MethodPost, path: "/api/v1/auth/login/code", body: map[string]string{"code": code}, headers: hr})
}

func sessionFrom(t *testing.T, w *httptest.ResponseRecorder) *session {
	t.Helper()
	body := decodeAs[struct {
		CSRFToken string `json:"csrf_token"`
	}](t, w)
	for _, c := range w.Result().Cookies() {
		if c.Name == "__Host-dtc_session" {
			return &session{cookie: c, csrf: body.CSRFToken}
		}
	}
	t.Fatal("no session cookie after the code")
	return nil
}

func TestTwoStepSignIn(t *testing.T) {
	h := newHarness(t)
	const email = "finance@example.invalid"
	h.createUser(email, "finance_officer")
	h.createUser("trainer@example.invalid", "trainer")
	s := h.mustLogin(email)
	other := h.mustLogin(email)

	st := decodeAs[twoStepStatusBody](t, h.do(req{method: http.MethodGet, path: "/api/v1/auth/two-step", sess: s}))
	if st.Enabled || !st.Recommended {
		t.Fatalf("finance officer status = %+v, want off and recommended", st)
	}
	trainer := h.mustLogin("trainer@example.invalid")
	if st := decodeAs[twoStepStatusBody](t, h.do(req{method: http.MethodGet, path: "/api/v1/auth/two-step", sess: trainer})); st.Recommended {
		t.Fatal("two-step recommended for a trainer")
	}

	// Changes need the password again, and the app must prove it has the secret.
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/two-step/setup", sess: s, body: map[string]string{"password": "not my password"}}), http.StatusUnprocessableEntity, "wrong_password")
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/two-step/confirm", sess: s, body: map[string]string{"code": "123456"}}), http.StatusConflict, "two_step_not_set_up")
	w := h.do(req{method: http.MethodPost, path: "/api/v1/auth/two-step/setup", sess: s, body: map[string]string{"password": testPassword}})
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/two-step/confirm", sess: s, body: map[string]string{"code": "000000"}}), http.StatusUnprocessableEntity, "invalid_code")
	if w.Code != http.StatusOK {
		t.Fatalf("setup: %d", w.Code)
	}
	// Not on until confirmed: the password alone still signs in.
	if s, _ := h.login(email, testPassword); s == nil {
		t.Fatal("an unconfirmed setup changed sign-in")
	}

	secret, codes := h.enableTwoStep(s)
	if len(codes) != auth.RecoveryCodeCount {
		t.Fatalf("%d recovery codes", len(codes))
	}
	var sealed []byte
	_ = h.pool.QueryRow(t.Context(), `SELECT secret_sealed FROM user_totp`).Scan(&sealed)
	if len(sealed) == 0 || string(sealed) == secret {
		t.Fatal("secret not stored sealed")
	}
	// Turning it on signs out every other session, not this one.
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/auth/session", sess: other}), http.StatusUnauthorized, "unauthenticated")
	if st := decodeAs[twoStepStatusBody](t, h.do(req{method: http.MethodGet, path: "/api/v1/auth/two-step", sess: s})); !st.Enabled || st.RecoveryCodesLeft != 10 {
		t.Fatalf("status after confirm = %+v", st)
	}
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/two-step/setup", sess: s, body: map[string]string{"password": testPassword}}), http.StatusConflict, "two_step_already_on")

	// The password alone now earns only a challenge.
	step := h.passwordStep(email)
	expectStatus(t, h.codeStep(nil, "123456"), http.StatusUnauthorized, "sign_in_again")
	expectStatus(t, h.codeStep(step, "000000"), http.StatusUnauthorized, "invalid_code")
	awayFromStepEdge()
	current := codeAt(t, secret, 0)
	w = h.codeStep(step, current)
	if w.Code != http.StatusOK {
		t.Fatalf("right code: %d %s", w.Code, w.Body.String())
	}
	signedIn := sessionFrom(t, w)
	if w := h.do(req{method: http.MethodGet, path: "/api/v1/auth/session", sess: signedIn}); w.Code != http.StatusOK {
		t.Fatalf("session after the code: %d", w.Code)
	}
	// A challenge is spent by its first success, and a code works once.
	expectStatus(t, h.codeStep(step, codeAt(t, secret, 30*time.Second)), http.StatusUnauthorized, "sign_in_again")
	expectStatus(t, h.codeStep(h.passwordStep(email), current), http.StatusUnauthorized, "invalid_code")

	// A recovery code works once, in any case and with or without dashes.
	upper := []byte(codes[0])
	for i, c := range upper {
		if c >= 'a' && c <= 'z' {
			upper[i] = c - 32
		}
	}
	if w := h.codeStep(h.passwordStep(email), string(upper)); w.Code != http.StatusOK {
		t.Fatalf("recovery code: %d %s", w.Code, w.Body.String())
	}
	expectStatus(t, h.codeStep(h.passwordStep(email), codes[0]), http.StatusUnauthorized, "invalid_code")
	if st := decodeAs[twoStepStatusBody](t, h.do(req{method: http.MethodGet, path: "/api/v1/auth/two-step", sess: s})); st.RecoveryCodesLeft != 9 {
		t.Fatalf("%d recovery codes left, want 9", st.RecoveryCodesLeft)
	}

	// An old challenge dies.
	old := h.passwordStep(email)
	h.exec(`UPDATE login_challenges SET expires_at = now() - interval '1 second', created_at = now() - interval '10 minutes' WHERE used_at IS NULL`)
	expectStatus(t, h.codeStep(old, codes[1]), http.StatusUnauthorized, "sign_in_again")

	// New recovery codes replace the old ones.
	w = h.do(req{method: http.MethodPost, path: "/api/v1/auth/two-step/recovery-codes", sess: s, body: map[string]string{"password": testPassword}})
	if w.Code != http.StatusOK {
		t.Fatalf("new codes: %d %s", w.Code, w.Body.String())
	}
	fresh := decodeAs[struct {
		Codes []string `json:"recovery_codes"`
	}](t, w).Codes
	expectStatus(t, h.codeStep(h.passwordStep(email), codes[2]), http.StatusUnauthorized, "invalid_code")

	// Turning it off needs the password; then the password alone signs in.
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/two-step/disable", sess: s, body: map[string]string{"password": "wrong password here"}}), http.StatusUnprocessableEntity, "wrong_password")
	// Failures above count towards the lockout; a success clears them.
	if w := h.codeStep(h.passwordStep(email), fresh[0]); w.Code != http.StatusOK {
		t.Fatalf("fresh recovery code: %d %s", w.Code, w.Body.String())
	}
	if w := h.do(req{method: http.MethodPost, path: "/api/v1/auth/two-step/disable", sess: s, body: map[string]string{"password": testPassword}}); w.Code != http.StatusNoContent {
		t.Fatalf("disable: %d %s", w.Code, w.Body.String())
	}
	if s, w := h.login(email, testPassword); s == nil {
		t.Fatalf("password alone refused after turning off: %d %s", w.Code, w.Body.String())
	}

	for action, want := range map[string]int{
		"auth.two_step_setup_started": 2, "auth.two_step_enabled": 1, "auth.recovery_codes_replaced": 1,
		"auth.two_step_disabled": 1, "auth.password_accepted": 7,
	} {
		if n := h.auditCount(action); n != want {
			t.Errorf("%s audited %d times, want %d", action, n, want)
		}
	}
}

// Wrong codes count towards the account lockout, so a known password does
// not give unlimited guesses at the code.
func TestTwoStepWrongCodesLockTheAccount(t *testing.T) {
	h := newHarness(t)
	const email = "registrar@example.invalid"
	h.createUser(email, "registrar")
	secret, _ := h.enableTwoStep(h.mustLogin(email))

	for range auth.MaxFailedLogins - 1 {
		expectStatus(t, h.codeStep(h.passwordStep(email), "000000"), http.StatusUnauthorized, "invalid_code")
	}
	expectStatus(t, h.codeStep(h.passwordStep(email), "000000"), http.StatusTooManyRequests, "too_many_attempts")
	// Locked: the password is refused and the right code would be too.
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/login", body: map[string]string{"email": email, "password": testPassword}}), http.StatusTooManyRequests, "too_many_attempts")

	// A single challenge also dies after MaxChallengeAttempts wrong codes.
	h.exec(`UPDATE users SET failed_login_count = 0, locked_until = NULL`)
	h.rebuild() // fresh in-memory limiters
	step := h.passwordStep(email)
	h.exec(`UPDATE login_challenges SET attempts = $1 WHERE used_at IS NULL`, auth.MaxChallengeAttempts)
	awayFromStepEdge()
	expectStatus(t, h.codeStep(step, codeAt(t, secret, 0)), http.StatusUnauthorized, "sign_in_again")
}

func TestTwoStepAdminReset(t *testing.T) {
	h := newHarness(t)
	h.createUser("ict@example.invalid", "ict_admin")
	h.createUser("owner@example.invalid", "super_admin")
	finance := h.createUser("finance@example.invalid", "finance_officer")
	owner := h.createUser("owner2@example.invalid", "super_admin")
	plain := h.createUser("trainer@example.invalid", "trainer")

	financeSession := h.mustLogin("finance@example.invalid")
	h.enableTwoStep(financeSession)
	ownerSession := h.mustLogin("owner2@example.invalid")
	h.enableTwoStep(ownerSession)
	ict := h.mustLogin("ict@example.invalid")
	ictID := decodeAs[struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}](t, h.do(req{method: http.MethodGet, path: "/api/v1/auth/session", sess: ict})).User.ID

	reset := func(s *session, id string) *httptest.ResponseRecorder {
		return h.do(req{method: http.MethodPost, path: "/api/v1/admin/users/" + id + "/two-step/reset", sess: s})
	}

	// The staff list shows who uses it.
	users := decodeAs[struct {
		Users []struct {
			Email   string `json:"email"`
			TwoStep bool   `json:"two_step_enabled"`
		} `json:"users"`
	}](t, h.do(req{method: http.MethodGet, path: "/api/v1/admin/users", sess: ict})).Users
	on := map[string]bool{}
	for _, u := range users {
		on[u.Email] = u.TwoStep
	}
	if !on["finance@example.invalid"] || on["trainer@example.invalid"] {
		t.Fatalf("two_step_enabled in the staff list = %v", on)
	}

	expectStatus(t, reset(h.mustLogin("trainer@example.invalid"), finance.String()), http.StatusForbidden, "forbidden")
	expectStatus(t, reset(ict, plain.String()), http.StatusConflict, "two_step_not_on")
	expectStatus(t, reset(ict, ictID), http.StatusConflict, "cannot_reset_self")
	// Only a Super Admin resets a Super Admin.
	expectStatus(t, reset(ict, owner.String()), http.StatusForbidden, "needs_super_admin")
	if w := reset(h.mustLogin("owner@example.invalid"), owner.String()); w.Code != http.StatusNoContent {
		t.Fatalf("super admin resets super admin: %d %s", w.Code, w.Body.String())
	}

	if w := reset(ict, finance.String()); w.Code != http.StatusNoContent {
		t.Fatalf("reset: %d %s", w.Code, w.Body.String())
	}
	// Signed out everywhere, then the password alone works again.
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/auth/session", sess: financeSession}), http.StatusUnauthorized, "unauthenticated")
	if s, w := h.login("finance@example.invalid", testPassword); s == nil {
		t.Fatalf("sign-in after reset: %d %s", w.Code, w.Body.String())
	}
	if n := h.auditCount("user.two_step_reset"); n != 2 {
		t.Fatalf("%d resets audited, want 2", n)
	}
}
