package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

var signInLinkPattern = regexp.MustCompile(`/apply/continue\?token=([A-Za-z0-9_-]+)`)

func (h *harness) requestLink(email, name string) {
	h.t.Helper()
	w := h.do(req{method: http.MethodPost, path: "/api/v1/applicant/sign-in-link", body: map[string]string{"email": email, "full_name": name}})
	expectStatus(h.t, w, http.StatusAccepted, "")
}

// linkFrom returns the token in the n-th email (1-based) sent to `to`.
func (h *harness) linkFrom(to string, n int) string {
	h.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var found []string
		for _, m := range h.mailer.All() {
			if m.To == to {
				if mm := signInLinkPattern.FindStringSubmatch(m.Text); mm != nil {
					found = append(found, mm[1])
				}
			}
		}
		if len(found) >= n {
			return found[n-1]
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatalf("no sign-in link number %d to %s", n, to)
	return ""
}

func (h *harness) mailsTo(to string) int {
	n := 0
	for _, m := range h.mailer.All() {
		if m.To == to {
			n++
		}
	}
	return n
}

type applicantSession struct {
	*session
	userID        string
	fullName      string
	applicationID *string
}

func (h *harness) applicantSignIn(token string) (*applicantSession, *httpResponse) {
	h.t.Helper()
	w := h.do(req{method: http.MethodPost, path: "/api/v1/applicant/sign-in", body: map[string]string{"token": token}})
	if w.Code != http.StatusOK {
		return nil, &httpResponse{w.Code, w.Body.String()}
	}
	var body struct {
		User struct {
			ID       string `json:"id"`
			FullName string `json:"full_name"`
			Kind     string `json:"kind"`
		} `json:"user"`
		CSRFToken     string  `json:"csrf_token"`
		ApplicationID *string `json:"application_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		h.t.Fatal(err)
	}
	if body.User.Kind != "applicant" {
		h.t.Fatalf("kind %q, want applicant", body.User.Kind)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "__Host-dtc_session" && c.HttpOnly && c.Secure {
			return &applicantSession{session: &session{cookie: c, csrf: body.CSRFToken}, userID: body.User.ID,
				fullName: body.User.FullName, applicationID: body.ApplicationID}, nil
		}
	}
	h.t.Fatal("sign-in set no secure session cookie")
	return nil, nil
}

func (h *harness) myApplications(s *session) []map[string]any {
	h.t.Helper()
	w := h.do(req{method: http.MethodGet, path: "/api/v1/applicant/applications", sess: s})
	expectStatus(h.t, w, http.StatusOK, "")
	var body struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		h.t.Fatal(err)
	}
	return body.Items
}

// insertApplication gives the applicant account an application, as the
// Stage 9 form will.
func (h *harness) insertApplication(userID, email, reference string) uuid.UUID {
	h.t.Helper()
	ctx := context.Background()
	var school, programme, applicantID, id uuid.UUID
	q := func(sql string, dst *uuid.UUID, args ...any) {
		if err := h.pool.QueryRow(ctx, sql, args...).Scan(dst); err != nil {
			h.t.Fatalf("%s: %v", sql, err)
		}
	}
	q(`INSERT INTO schools (slug, name, short_name, accent_key) VALUES ('s-'||substr(md5(random()::text),1,8), 'School', 'S', 'ict') RETURNING id`, &school)
	q(`INSERT INTO programmes (school_id, slug, title, level) VALUES ($1, 'p-'||substr(md5(random()::text),1,8), 'Diploma in Testing (Sample)', 'diploma') RETURNING id`, &programme, school)
	q(`INSERT INTO applicants (user_id, email, first_name, last_name) VALUES ($1, $2, 'Test', 'Applicant 001') RETURNING id`, &applicantID, userID, email)
	q(`INSERT INTO applications (reference, applicant_id, programme_id) VALUES ($1, $2, $3) RETURNING id`, &id, reference, applicantID, programme)
	return id
}

func (h *harness) notificationStatus(to string) []string {
	h.t.Helper()
	rows, err := h.pool.Query(context.Background(), `SELECT status FROM notifications WHERE recipient = $1 ORDER BY created_at`, to)
	if err != nil {
		h.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			h.t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

// D-96, AC-04 (resume): a new applicant gets a link, the link signs them in
// once, and the account is created on first use.
func TestApplicantSignInByEmailLink(t *testing.T) {
	h := newHarness(t)
	const email = "applicant001@example.invalid"

	h.requestLink("  Applicant001@Example.invalid ", "Test Applicant 001")
	token := h.linkFrom(email, 1)
	m := h.mailer.All()[0]
	if m.Subject != "Your link to continue your application" || !strings.Contains(m.Text, "Hello Test Applicant 001") ||
		!strings.Contains(m.Text, testOrigin+"/apply/continue?token=") || !strings.Contains(m.Text, "30 minutes") {
		t.Fatalf("unexpected email: %+v", m)
	}

	// Nothing is created before the link is used.
	var users int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE email = $1`, email).Scan(&users); err != nil || users != 0 {
		t.Fatalf("users before sign-in = %d (%v)", users, err)
	}

	a, resp := h.applicantSignIn(token)
	if a == nil {
		t.Fatalf("sign-in: %+v", resp)
	}
	if a.fullName != "Test Applicant 001" || a.applicationID != nil {
		t.Fatalf("session = %+v", a)
	}
	if items := h.myApplications(a.session); len(items) != 0 {
		t.Fatalf("new applicant has applications: %v", items)
	}

	// The session works for /auth/session and has no staff permissions.
	w := h.do(req{method: http.MethodGet, path: "/api/v1/auth/session", sess: a.session})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"permissions":[]`) {
		t.Fatalf("session: %d %s", w.Code, w.Body.String())
	}

	// Single use.
	_, resp = h.applicantSignIn(token)
	resp.expect(t, http.StatusBadRequest, "invalid_sign_in_link")

	// A second link signs in to the same account and keeps the name; using
	// it spends every other outstanding link.
	h.requestLink(email, "Someone Else")
	h.requestLink(email, "")
	second, third := h.linkFrom(email, 2), h.linkFrom(email, 3)
	again, _ := h.applicantSignIn(third)
	if again == nil || again.userID != a.userID || again.fullName != "Test Applicant 001" {
		t.Fatalf("second sign-in = %+v, first user %s", again, a.userID)
	}
	_, resp = h.applicantSignIn(second)
	resp.expect(t, http.StatusBadRequest, "invalid_sign_in_link")

	if n := h.auditCount("applicant.signed_in"); n != 2 {
		t.Fatalf("applicant.signed_in audited %d times, want 2", n)
	}
	if n := h.auditCount("applicant.sign_in_link_requested"); n != 3 {
		t.Fatalf("link requests audited %d times, want 3", n)
	}
	// The log has a row per message and stores no body (links are secrets).
	time.Sleep(50 * time.Millisecond)
	if got := h.notificationStatus(email); len(got) != 3 || got[0] != "sent" {
		t.Fatalf("notification log = %v", got)
	}
	// Staff password sign-in does not work for an applicant account.
	_, lw := h.login(email, testPassword)
	expectStatus(t, lw, http.StatusUnauthorized, "invalid_credentials")
}

func TestApplicantSignInLinkRules(t *testing.T) {
	h := newHarness(t)

	// Invalid email is a fixable error.
	w := h.do(req{method: http.MethodPost, path: "/api/v1/applicant/sign-in-link", body: map[string]string{"email": "not-an-email"}})
	expectStatus(t, w, http.StatusUnprocessableEntity, "validation_failed")

	// Staff accounts never get an applicant link, but the answer is the same.
	h.createUser("staff@example.invalid", "registrar")
	h.requestLink("staff@example.invalid", "")
	time.Sleep(50 * time.Millisecond)
	if n := len(h.mailer.All()); n != 0 {
		t.Fatalf("staff account was emailed an applicant link (%d emails)", n)
	}

	// An unused link expires after 30 minutes.
	h.requestLink("late@example.invalid", "")
	token := h.linkFrom("late@example.invalid", 1)
	h.now = func() time.Time { return time.Now().Add(31 * time.Minute) }
	_, resp := h.applicantSignIn(token)
	resp.expect(t, http.StatusBadRequest, "invalid_sign_in_link")
	h.now = time.Now

	// A made-up token is refused.
	_, resp = h.applicantSignIn("not-a-real-token")
	resp.expect(t, http.StatusBadRequest, "invalid_sign_in_link")

	// Five links per address per hour; the sixth request gets the same answer
	// but no email.
	for range 6 {
		h.requestLink("busy@example.invalid", "")
	}
	h.linkFrom("busy@example.invalid", 5)
	time.Sleep(50 * time.Millisecond)
	if n := h.mailsTo("busy@example.invalid"); n != 5 {
		t.Fatalf("emails after six requests = %d, want 5", n)
	}

	// A disabled applicant gets no link.
	h.requestLink("gone@example.invalid", "")
	if a, _ := h.applicantSignIn(h.linkFrom("gone@example.invalid", 1)); a == nil {
		t.Fatal("first sign-in failed")
	}
	h.exec(`UPDATE users SET status = 'disabled' WHERE email = 'gone@example.invalid'`)
	h.requestLink("gone@example.invalid", "")
	time.Sleep(50 * time.Millisecond)
	if n := h.mailsTo("gone@example.invalid"); n != 1 {
		t.Fatalf("disabled applicant emails = %d, want 1", n)
	}
}

// AC-04: an application is found again by reference number plus the email on
// it; nobody else learns anything from trying.
func TestResumeByReference(t *testing.T) {
	h := newHarness(t)
	const email = "applicant002@example.invalid"
	h.requestLink(email, "Test Applicant 002")
	owner, _ := h.applicantSignIn(h.linkFrom(email, 1))
	appID := h.insertApplication(owner.userID, email, "DTC-APP-2026-000123")

	resume := func(reference, address string) {
		t.Helper()
		w := h.do(req{method: http.MethodPost, path: "/api/v1/applicant/resume", body: map[string]string{"reference": reference, "email": address}})
		expectStatus(t, w, http.StatusAccepted, "")
	}

	// Wrong email, unknown reference: same answer, no email.
	resume("DTC-APP-2026-000123", "someone@example.invalid")
	resume("DTC-APP-2026-999999", email)
	time.Sleep(50 * time.Millisecond)
	if n := len(h.mailer.All()); n != 1 {
		t.Fatalf("emails = %d, want only the first sign-in link", n)
	}

	// Right pair, typed loosely.
	resume(" dtc-app-2026-000123 ", "Applicant002@example.invalid")
	token := h.linkFrom(email, 2)
	m := h.mailer.All()[1]
	if m.Subject != "Continue application DTC-APP-2026-000123" || !strings.Contains(m.Text, "Hello Test Applicant 002") {
		t.Fatalf("unexpected resume email: %+v", m)
	}
	a, resp := h.applicantSignIn(token)
	if a == nil {
		t.Fatalf("resume sign-in: %+v", resp)
	}
	if a.applicationID == nil || *a.applicationID != appID.String() || a.userID != owner.userID {
		t.Fatalf("resume session = %+v", a)
	}
	items := h.myApplications(a.session)
	if len(items) != 1 || items[0]["reference"] != "DTC-APP-2026-000123" || items[0]["status"] != "draft" ||
		items[0]["programme_title"] != "Diploma in Testing (Sample)" {
		t.Fatalf("applications = %v", items)
	}

	// Another applicant sees only their own (none).
	h.requestLink("applicant003@example.invalid", "")
	other, _ := h.applicantSignIn(h.linkFrom("applicant003@example.invalid", 1))
	if items := h.myApplications(other.session); len(items) != 0 {
		t.Fatalf("other applicant sees %v", items)
	}

	// Validation.
	w := h.do(req{method: http.MethodPost, path: "/api/v1/applicant/resume", body: map[string]string{"reference": "", "email": email}})
	expectStatus(t, w, http.StatusUnprocessableEntity, "validation_failed")
}

// Applicant routes are closed to staff, and staff routes to applicants.
func TestApplicantAndStaffRoutesAreSeparate(t *testing.T) {
	h := newHarness(t)
	h.firstSuperAdmin("owner@example.invalid")
	staff := h.mustLogin("owner@example.invalid")
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/applicant/applications", sess: staff}), http.StatusForbidden, "forbidden")

	h.requestLink("applicant004@example.invalid", "")
	a, _ := h.applicantSignIn(h.linkFrom("applicant004@example.invalid", 1))
	for _, r := range h.router.Routes {
		if r.Permission == "" {
			continue
		}
		w := h.do(req{method: r.Method, path: concretePath(r.Path), body: map[string]any{}, sess: a.session})
		if w.Code != http.StatusForbidden {
			t.Errorf("applicant reached %s %s: %d", r.Method, r.Path, w.Code)
		}
	}
	// Without the CSRF token the applicant cannot sign out (or change anything).
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/logout", sess: a.session, noCSRF: true}), http.StatusForbidden, "csrf_failed")
}

func TestNotificationTemplates(t *testing.T) {
	h := newHarness(t)
	h.createUser("registrar@example.invalid", "registrar")
	h.createUser("editor@example.invalid", "content_editor")
	registrar := h.mustLogin("registrar@example.invalid")
	editor := h.mustLogin("editor@example.invalid")

	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/staff/notification-templates", sess: editor}), http.StatusForbidden, "forbidden")

	w := h.do(req{method: http.MethodGet, path: "/api/v1/staff/notification-templates", sess: registrar})
	expectStatus(t, w, http.StatusOK, "")
	var list struct {
		Items []struct {
			Key, Channel string
			Sending      bool
			MaxLength    int `json:"max_length"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	// 6 from Stage 8b; 8 messages and the offer letter from Stage 10a; 2 from 12a.
	if len(list.Items) != 17 {
		t.Fatalf("templates = %+v", list.Items)
	}
	for _, it := range list.Items {
		if it.Sending != (it.Channel == "email") || (it.Channel == "sms" && it.MaxLength != 320) {
			t.Fatalf("template %+v", it)
		}
	}

	put := func(key, channel string, body map[string]string) *httpResponse {
		w := h.do(req{method: http.MethodPut, path: "/api/v1/staff/notification-templates/" + key + "/" + channel, body: body, sess: registrar})
		return &httpResponse{w.Code, w.Body.String()}
	}
	put("applicant.sign_in_link", "email", map[string]string{"subject": "Hi {name}", "body": "Open {link} for {programme}"}).
		expect(t, http.StatusUnprocessableEntity, "validation_failed")
	put("applicant.sign_in_link", "email", map[string]string{"subject": "Hi", "body": "No link here"}).
		expect(t, http.StatusUnprocessableEntity, "validation_failed")
	put("applicant.sign_in_link", "email", map[string]string{"subject": "", "body": "{link}"}).
		expect(t, http.StatusUnprocessableEntity, "validation_failed")
	put("application.started", "sms", map[string]string{"body": strings.Repeat("a", 321)}).
		expect(t, http.StatusUnprocessableEntity, "validation_failed")
	put("no.such_template", "email", map[string]string{"subject": "x", "body": "y"}).
		expect(t, http.StatusNotFound, "not_found")
	put("applicant.sign_in_link", "fax", map[string]string{"subject": "x", "body": "{link}"}).
		expect(t, http.StatusNotFound, "not_found")
	// The offer letter is a letter, with its own placeholders and a heading.
	put("offer.letter", "letter", map[string]string{"subject": "", "body": "Dear {name}"}).
		expect(t, http.StatusUnprocessableEntity, "validation_failed")
	put("offer.letter", "letter", map[string]string{"subject": "Offer", "body": "Dear {name}, see {link}"}).
		expect(t, http.StatusUnprocessableEntity, "validation_failed")
	put("offer.letter", "email", map[string]string{"subject": "Offer", "body": "Dear {name}"}).
		expect(t, http.StatusNotFound, "not_found")

	put("applicant.sign_in_link", "email", map[string]string{"subject": "Sign in, {name}", "body": "Hello {name}, open {link} within {minutes} minutes."}).
		expect(t, http.StatusNoContent, "")
	put("application.started", "sms", map[string]string{"subject": "ignored", "body": "Ref {reference}"}).
		expect(t, http.StatusNoContent, "")
	if n := h.auditCount("notifications.template_updated"); n != 2 {
		t.Fatalf("template updates audited %d times, want 2", n)
	}
	var subject string
	if err := h.pool.QueryRow(context.Background(), `SELECT subject FROM notification_templates WHERE key = 'application.started' AND channel = 'sms'`).Scan(&subject); err != nil || subject != "" {
		t.Fatalf("sms subject = %q (%v)", subject, err)
	}

	// The next link uses the new text.
	h.requestLink("applicant005@example.invalid", "Test Applicant 005")
	h.linkFrom("applicant005@example.invalid", 1)
	m := h.mailer.All()[0]
	if m.Subject != "Sign in, Test Applicant 005" || !strings.Contains(m.Text, "within 30 minutes") {
		t.Fatalf("email after edit: %+v", m)
	}
}

// Two links for a new address used at the same moment make one account.
func TestConcurrentFirstSignInsMakeOneAccount(t *testing.T) {
	h := newHarness(t)
	const email = "applicant006@example.invalid"
	h.requestLink(email, "")
	h.requestLink(email, "")
	tokens := []string{h.linkFrom(email, 1), h.linkFrom(email, 2)}
	codes := make(chan int, 2)
	for _, tok := range tokens {
		go func() {
			w := h.do(req{method: http.MethodPost, path: "/api/v1/applicant/sign-in", body: map[string]string{"token": tok}})
			codes <- w.Code
		}()
	}
	got := []int{<-codes, <-codes}
	slices.Sort(got)
	if !slices.Equal(got, []int{http.StatusOK, http.StatusBadRequest}) {
		t.Fatalf("status codes %v, want one 200 and one 400", got)
	}
	var n int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE email = $1`, email).Scan(&n); err != nil || n != 1 {
		t.Fatalf("accounts = %d (%v)", n, err)
	}
}
