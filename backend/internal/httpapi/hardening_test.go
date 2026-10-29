package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

func TestSecurityHeaders(t *testing.T) {
	want := map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Content-Security-Policy":      "frame-ancestors 'none'",
		"Referrer-Policy":              "no-referrer",
		"Cross-Origin-Resource-Policy": "same-origin",
		"Cache-Control":                "no-store",
	}
	// Local http: everything but HSTS, on success, 404 and 405 alike.
	r := newTestRouter(t, fakePinger{}, nil)
	for _, w := range []*httptest.ResponseRecorder{
		do(r, http.MethodGet, "/api/v1/health", nil),
		do(r, http.MethodGet, "/api/v1/nope", nil),
		do(r, http.MethodPost, "/api/v1/health", nil),
	} {
		for k, v := range want {
			if got := w.Header().Get(k); got != v {
				t.Errorf("%d response: %s = %q, want %q", w.Code, k, got, v)
			}
		}
		if got := w.Header().Get("Strict-Transport-Security"); got != "" {
			t.Errorf("HSTS sent over local http: %q", got)
		}
	}

	// Behind HTTPS: HSTS too. Public data overrides Cache-Control.
	h := newHarness(t)
	w := h.do(req{method: http.MethodGet, path: "/api/v1/public/site"})
	if got := w.Header().Get("Strict-Transport-Security"); got != "max-age=31536000" {
		t.Errorf("HSTS = %q", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=60" {
		t.Errorf("public data Cache-Control = %q", got)
	}
	h.createUser("headers@example.invalid", "super_admin")
	s := h.mustLogin("headers@example.invalid")
	if got := h.do(req{method: http.MethodGet, path: "/api/v1/admin/users", sess: s}).Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("staff data Cache-Control = %q", got)
	}
}

func TestTrustedClientIP(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	e := gin.New()
	e.Use(trustedClientIP(secret))
	e.GET("/", func(c *gin.Context) { c.String(http.StatusOK, clientIP(c)) })

	cases := []struct {
		name, ip, key, want string
	}{
		{"verified visitor", "41.90.1.2", secret, "41.90.1.2"},
		{"verified IPv6 visitor", "2c0f:fe38::1", secret, "2c0f:fe38::1"},
		{"IPv4 written as IPv6", "::ffff:41.90.1.2", secret, "41.90.1.2"},
		{"no key: visitor-supplied, ignored", "41.90.1.2", "", "192.0.2.1"},
		{"wrong key", "41.90.1.2", secret[:31] + "x", "192.0.2.1"},
		{"not an address", "41.90.1.2, 10.0.0.1", secret, "192.0.2.1"},
	}
	for _, tc := range cases {
		hr := httptest.NewRequest(http.MethodGet, "/", nil) // peer 192.0.2.1
		hr.Header.Set(clientIPHeader, tc.ip)
		if tc.key != "" {
			hr.Header.Set(clientIPKeyHeader, tc.key)
		}
		w := httptest.NewRecorder()
		e.ServeHTTP(w, hr)
		if w.Body.String() != tc.want {
			t.Errorf("%s: client IP %q, want %q", tc.name, w.Body.String(), tc.want)
		}
	}

	// No secret configured: the header is never believed.
	off := gin.New()
	off.Use(trustedClientIP(""))
	off.GET("/", func(c *gin.Context) { c.String(http.StatusOK, clientIP(c)) })
	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	hr.Header.Set(clientIPHeader, "41.90.1.2")
	hr.Header.Set(clientIPKeyHeader, "")
	w := httptest.NewRecorder()
	off.ServeHTTP(w, hr)
	if w.Body.String() != "192.0.2.1" {
		t.Errorf("without a secret: %q", w.Body.String())
	}
}

// The verified address is what audit entries and the login limits key on,
// so one visitor's failed sign-ins no longer count against everyone (R-23).
func TestVerifiedVisitorAddressReachesAuditAndLimits(t *testing.T) {
	h := newHarness(t)
	h.createUser("visitor@example.invalid")
	loginFrom := func(ip, key, email string) int {
		return h.do(req{method: http.MethodPost, path: "/api/v1/auth/login",
			body:    map[string]string{"email": email, "password": "wrong password"},
			headers: map[string]string{clientIPHeader: ip, clientIPKeyHeader: key}}).Code
	}
	auditIP := func() string {
		var ip string
		if err := h.pool.QueryRow(context.Background(),
			`SELECT host(ip) FROM audit_logs WHERE action = 'auth.login_failed' ORDER BY occurred_at DESC, id DESC LIMIT 1`).Scan(&ip); err != nil {
			t.Fatal(err)
		}
		return ip
	}

	loginFrom("41.90.1.2", testClientIPSecret, "visitor@example.invalid")
	if got := auditIP(); got != "41.90.1.2" {
		t.Errorf("verified visitor audited as %q", got)
	}
	loginFrom("41.90.9.9", "", "visitor@example.invalid")
	if got := auditIP(); got != "192.0.2.1" {
		t.Errorf("unverified header audited as %q, want the peer", got)
	}

	// One address uses up its sign-in allowance; another still gets a
	// normal answer (401, not 429).
	for i := 0; i < 100; i++ {
		loginFrom("41.90.1.3", testClientIPSecret, "nobody"+string(rune('a'+i%26))+string(rune('a'+i/26))+"@example.invalid")
	}
	if code := loginFrom("41.90.1.3", testClientIPSecret, "visitor2@example.invalid"); code != http.StatusTooManyRequests {
		t.Errorf("busy address after 100 attempts: %d, want 429", code)
	}
	if code := loginFrom("41.90.1.4", testClientIPSecret, "visitor2@example.invalid"); code != http.StatusUnauthorized {
		t.Errorf("another visitor: %d, want 401", code)
	}
}

// A student or applicant account never reaches a staff endpoint, even with a
// role attached directly in the database, and the API refuses to attach one.
func TestPermissionRoutesNeedAStaffAccount(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	for _, r := range h.router.Routes {
		if r.Permission != "" && r.Kind != "staff" {
			t.Errorf("%s %s names a permission but admits %q accounts", r.Method, r.Path, r.Kind)
		}
	}

	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	q := dbq.New(h.pool)
	studentID, err := q.UpsertUserByEmail(ctx, dbq.UpsertUserByEmailParams{
		Email: "student.with.role@example.invalid", FullName: "Test Student 900", Kind: "student", PasswordHash: &hash})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.AssignRole(ctx, dbq.AssignRoleParams{UserID: studentID, RoleID: h.roleID("super_admin")}); err != nil {
		t.Fatal(err)
	}
	student := h.mustLogin("student.with.role@example.invalid")
	for _, path := range []string{"/api/v1/admin/users", "/api/v1/staff/enquiries", "/api/v1/staff/finance/summary"} {
		expectStatus(t, h.do(req{method: http.MethodGet, path: path, sess: student}), http.StatusForbidden, "forbidden")
	}

	h.createUser("roles.admin@example.invalid", "super_admin")
	admin := h.mustLogin("roles.admin@example.invalid")
	setRoles(h, admin, studentID, "registrar").expect(t, http.StatusConflict, "not_staff_account")
	if n := h.auditCount("user.roles_changed"); n != 0 {
		t.Errorf("refused role change audited %d times", n)
	}
}

// Every applicant route that names an application refuses another
// applicant, and the owner's draft is unchanged afterwards (object-level
// authorisation, docs/security.md).
func TestApplicantRoutesRefuseOtherApplicants(t *testing.T) {
	h := newHarness(t)
	owner, id := h.readyApplication("owner.applicant@example.invalid")
	h.uploadAll(owner.session, id)
	var doc string
	if err := h.pool.QueryRow(context.Background(),
		`SELECT id::text FROM application_documents WHERE application_id = $1 LIMIT 1`, id).Scan(&doc); err != nil {
		t.Fatal(err)
	}
	before := h.do(req{method: http.MethodGet, path: "/api/v1/applicant/applications/" + id, sess: owner.session}).Body.String()

	other := h.newApplicant("other.applicant@example.invalid", "Test Applicant 021")
	n := 0
	for _, r := range h.router.Routes {
		if r.Kind != "applicant" || !strings.Contains(r.Path, ":id") {
			continue
		}
		n++
		path := strings.NewReplacer(":id", id, ":doc", doc).Replace(r.Path)
		w := h.do(req{method: r.Method, path: path, body: map[string]any{}, sess: other.session,
			headers: map[string]string{"Idempotency-Key": "other-" + r.Method + path}})
		if w.Code < 400 || strings.Contains(w.Body.String(), "Applicant 020") {
			t.Errorf("%s %s by another applicant: %d %s", r.Method, r.Path, w.Code, w.Body.String())
		}
	}
	if n < 10 {
		t.Fatalf("only %d applicant routes with an id were checked", n)
	}
	if after := h.do(req{method: http.MethodGet, path: "/api/v1/applicant/applications/" + id, sess: owner.session}).Body.String(); after != before {
		t.Errorf("the owner's application changed:\nbefore %s\nafter  %s", before, after)
	}
}

// docs/endpoints.md lists every route and who may call it, generated from the
// route table so the reviewed list cannot drift from the code. Regenerate
// with UPDATE_ENDPOINTS=1 after adding or changing a route.
func TestEndpointInventoryIsCurrent(t *testing.T) {
	h := newHarness(t)
	var b strings.Builder
	b.WriteString("# API endpoints\n\nGenerated from the route table by `TestEndpointInventoryIsCurrent` " +
		"(backend/internal/httpapi/hardening_test.go). Do not edit by hand: run\n" +
		"`UPDATE_ENDPOINTS=1 go test ./internal/httpapi -run TestEndpointInventoryIsCurrent` after changing a route.\n\n" +
		"Every route needs a session unless marked public. Staff routes also need the named permission and a staff account; " +
		"applicant and student routes act only on the caller's own records. Changes (POST, PUT, DELETE) on signed-in routes also need the CSRF token. " +
		"The review is in [security.md](security.md).\n\n")
	counts := map[string]int{}
	b.WriteString("| Method | Path | Who may call it |\n|---|---|---|\n")
	for _, r := range h.router.Routes {
		who := ""
		switch {
		case r.Public && strings.Contains(r.Path, ":secret"):
			who, counts["public"] = "Public, on a secret path (payment provider)", counts["public"]+1
		case r.Public:
			who, counts["public"] = "Public", counts["public"]+1
		case r.Permission != "":
			who, counts["staff"] = "Staff with `"+r.Permission+"`", counts["staff"]+1
		case r.Kind == "staff":
			who, counts["staff"] = "Any staff account", counts["staff"]+1
		case r.Kind != "":
			who, counts[r.Kind] = "The "+r.Kind+", own records only", counts[r.Kind]+1
		default:
			who, counts["session"] = "Any signed-in account", counts["session"]+1
		}
		fmt.Fprintf(&b, "| %s | `%s` | %s |\n", r.Method, r.Path, who)
	}
	fmt.Fprintf(&b, "\n%d routes: %d public, %d staff, %d applicant, %d student, %d for any signed-in account.\n",
		len(h.router.Routes), counts["public"], counts["staff"], counts["applicant"], counts["student"], counts["session"])

	const path = "../../../docs/endpoints.md"
	if os.Getenv("UPDATE_ENDPOINTS") == "1" {
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with UPDATE_ENDPOINTS=1)", err)
	}
	if string(got) != b.String() {
		t.Error("docs/endpoints.md is out of date: run UPDATE_ENDPOINTS=1 go test ./internal/httpapi -run TestEndpointInventoryIsCurrent and review the change")
	}
}
