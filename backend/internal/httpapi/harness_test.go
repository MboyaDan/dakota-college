package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/academics"
	"github.com/MboyaDan/dakota-college/backend/internal/access"
	"github.com/MboyaDan/dakota-college/backend/internal/admissions"
	"github.com/MboyaDan/dakota-college/backend/internal/applicant"
	"github.com/MboyaDan/dakota-college/backend/internal/application"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/config"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/enquiry"
	"github.com/MboyaDan/dakota-college/backend/internal/mail"
	"github.com/MboyaDan/dakota-college/backend/internal/media"
	"github.com/MboyaDan/dakota-college/backend/internal/notify"
	"github.com/MboyaDan/dakota-college/backend/internal/portal"
	"github.com/MboyaDan/dakota-college/backend/internal/testdb"
)

const (
	testOrigin   = "https://dakota.example"
	testPassword = "correct horse battery staple"
)

// harness is a full API over a fresh, migrated database.
type harness struct {
	t       *testing.T
	pool    *pgxpool.Pool
	router  *Router
	auth    *auth.Service
	access  *access.Service
	mailer  *mail.Recorder
	storage *media.Memory
	// scanner is the virus scanner documents go through (nil: none).
	scanner media.Scanner
	// now is the clock for applicant links; tests move it forward.
	now func() time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	pool, _ := testdb.Migrated(t)
	h := &harness{t: t, pool: pool, mailer: &mail.Recorder{}, storage: &media.Memory{}, now: time.Now}
	h.rebuild()
	return h
}

// rebuild creates new services and router over the same database, as an API
// restart would (in-memory rate limits are lost; the database is not).
func (h *harness) rebuild() {
	h.t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var err error
	h.auth, err = auth.NewService(auth.Options{
		Pool: h.pool, Mailer: h.mailer, Logger: logger, AppBaseURL: testOrigin,
		Session: config.SessionConfig{CookieSecure: true, TTL: 12 * time.Hour, IdleTimeout: 30 * time.Minute},
	})
	if err != nil {
		h.t.Fatal(err)
	}
	h.access = access.NewService(h.pool)
	notifier := notify.NewService(h.pool, h.mailer, logger)
	academicsSvc := academics.NewService(h.pool, func() time.Time { return h.now() })
	h.router, err = NewRouter(Deps{
		Logger: logger, DB: h.pool, AllowedOrigins: []string{testOrigin}, CookieSecure: true,
		Auth: h.auth, Access: h.access, Content: content.NewService(h.pool),
		Images:    &media.Images{Pool: h.pool, Storage: h.storage, Bucket: "dakota-public", PublicBaseURL: "https://cdn.example/dakota-public"},
		Enquiries: enquiry.NewService(h.pool, h.mailer, logger, testOrigin),
		Notify:    notifier,
		Applicants: applicant.NewService(applicant.Options{
			Pool: h.pool, Auth: h.auth, Notify: notifier, Logger: logger, AppBaseURL: testOrigin, Now: func() time.Time { return h.now() },
		}),
		Applications: application.NewService(application.Options{
			Pool: h.pool, Notify: notifier, Logger: logger, AppBaseURL: testOrigin, Now: func() time.Time { return h.now() },
			Storage: h.storage, PrivateBucket: "dakota-private", Scanner: h.scanner,
		}),
		Academics: academicsSvc,
		Portal: portal.NewService(portal.Options{
			Pool: h.pool, Academics: academicsSvc, Notify: notifier, Logger: logger, AppBaseURL: testOrigin, Now: func() time.Time { return h.now() },
		}),
		Admissions: admissions.NewService(admissions.Options{
			Pool: h.pool, Notify: notifier, Logger: logger, AppBaseURL: testOrigin, Now: func() time.Time { return h.now() },
			Storage: h.storage, PrivateBucket: "dakota-private",
		}),
	})
	if err != nil {
		h.t.Fatal(err)
	}
}

// createUser makes an active staff user with the given roles and password.
func (h *harness) createUser(email string, roleKeys ...string) uuid.UUID {
	h.t.Helper()
	ctx := context.Background()
	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		h.t.Fatal(err)
	}
	q := dbq.New(h.pool)
	id, err := q.UpsertUserByEmail(ctx, dbq.UpsertUserByEmailParams{Email: email, FullName: "Test User " + email, Kind: "staff", PasswordHash: &hash})
	if err != nil {
		h.t.Fatal(err)
	}
	for _, k := range roleKeys {
		role, err := q.GetRoleByKey(ctx, k)
		if err != nil {
			h.t.Fatalf("role %s: %v", k, err)
		}
		if err := q.AssignRole(ctx, dbq.AssignRoleParams{UserID: id, RoleID: role.ID}); err != nil {
			h.t.Fatal(err)
		}
	}
	return id
}

func (h *harness) roleID(key string) uuid.UUID {
	h.t.Helper()
	r, err := dbq.New(h.pool).GetRoleByKey(context.Background(), key)
	if err != nil {
		h.t.Fatal(err)
	}
	return r.ID
}

// session is a signed-in browser: cookie plus CSRF token.
type session struct {
	cookie *http.Cookie
	csrf   string
}

type req struct {
	method, path string
	body         any
	// raw and contentType send a non-JSON body (file uploads).
	raw         io.Reader
	contentType string
	sess        *session
	noCSRF      bool
	headers     map[string]string
}

func (h *harness) do(r req) *httptest.ResponseRecorder {
	h.t.Helper()
	var body io.Reader
	if r.body != nil {
		b, err := json.Marshal(r.body)
		if err != nil {
			h.t.Fatal(err)
		}
		body = bytes.NewReader(b)
	}
	if r.raw != nil {
		body = r.raw
	}
	hr := httptest.NewRequest(r.method, r.path, body)
	switch {
	case r.raw != nil:
		hr.Header.Set("Content-Type", r.contentType)
	case r.body != nil:
		hr.Header.Set("Content-Type", "application/json")
	}
	if r.sess != nil {
		hr.AddCookie(r.sess.cookie)
		if !r.noCSRF {
			hr.Header.Set(csrfHeader, r.sess.csrf)
		}
	}
	for k, v := range r.headers {
		hr.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, hr)
	return w
}

func (h *harness) login(email, password string) (*session, *httptest.ResponseRecorder) {
	h.t.Helper()
	w := h.do(req{method: http.MethodPost, path: "/api/v1/auth/login", body: map[string]string{"email": email, "password": password}})
	if w.Code != http.StatusOK {
		return nil, w
	}
	var body struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		h.t.Fatal(err)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "__Host-dtc_session" {
			return &session{cookie: c, csrf: body.CSRFToken}, w
		}
	}
	h.t.Fatal("login set no session cookie")
	return nil, w
}

func (h *harness) mustLogin(email string) *session {
	h.t.Helper()
	s, w := h.login(email, testPassword)
	if s == nil {
		h.t.Fatalf("login %s: %d %s", email, w.Code, w.Body.String())
	}
	return s
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var b errorBody
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("not an error body: %s", w.Body.String())
	}
	return b.Error.Code
}

func expectStatus(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status %d, want %d: %s", w.Code, status, w.Body.String())
	}
	if code != "" {
		if got := errorCode(t, w); got != code {
			t.Fatalf("error code %q, want %q", got, code)
		}
	}
}

func (h *harness) auditCount(action string) int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action = $1`, action).Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}

func (h *harness) exec(sql string, args ...any) {
	h.t.Helper()
	if _, err := h.pool.Exec(context.Background(), sql, args...); err != nil {
		h.t.Fatalf("%s: %v", sql, err)
	}
}
