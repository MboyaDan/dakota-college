package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/applicant"
)

// openProgramme makes a published programme with `open` intakes taking
// applications and one closed intake. It returns the slug and intake ids.
func (h *harness) openProgramme(open int) (slug string, intakes []string, closed string) {
	h.t.Helper()
	ctx := context.Background()
	var school uuid.UUID
	if err := h.pool.QueryRow(ctx, `INSERT INTO schools (slug, name, short_name, accent_key, status, published_at)
		VALUES ('s-'||substr(md5(random()::text),1,8), 'School', 'S', 'ict', 'published', now()) RETURNING id`).Scan(&school); err != nil {
		h.t.Fatal(err)
	}
	slug = "p-" + uuid.NewString()[:8]
	var programme uuid.UUID
	if err := h.pool.QueryRow(ctx, `INSERT INTO programmes (school_id, slug, title, level, status, published_at)
		VALUES ($1, $2, 'Diploma in Testing (Sample)', 'diploma', 'published', now()) RETURNING id`, school, slug).Scan(&programme); err != nil {
		h.t.Fatal(err)
	}
	add := func(months int, status string) string {
		starts := time.Now().AddDate(0, months, 0)
		var id string
		if err := h.pool.QueryRow(ctx, `INSERT INTO programme_intakes (programme_id, name, starts_on, application_deadline, status)
			VALUES ($1, $2, $3::date, ($3::date - 20), $4) RETURNING id`, programme, fmt.Sprintf("Intake %d (Sample)", months), starts, status).Scan(&id); err != nil {
			h.t.Fatal(err)
		}
		return id
	}
	for i := range open {
		intakes = append(intakes, add(3+i, "open"))
	}
	closed = add(12, "closed")
	return slug, intakes, closed
}

// newApplicant signs a new applicant in by link.
func (h *harness) newApplicant(email, name string) *applicantSession {
	h.t.Helper()
	n := h.mailsTo(email)
	h.requestLink(email, name)
	a, resp := h.applicantSignIn(h.linkFrom(email, n+1))
	if a == nil {
		h.t.Fatalf("sign-in %s: %+v", email, resp)
	}
	return a
}

type appDetail struct {
	ID            string `json:"id"`
	Reference     string `json:"reference"`
	Status        string `json:"status"`
	Editable      bool   `json:"editable"`
	IntakeID      string `json:"intake_id"`
	NeedsGuardian bool   `json:"needs_guardian"`
	Personal      map[string]string
	Guardian      struct {
		FullName     string `json:"full_name"`
		ConsentGiven bool   `json:"consent_given"`
	}
	Education []map[string]any
	Missing   []struct{ Step, Field, Message string }
}

func (d appDetail) missingSteps() []string {
	var out []string
	for _, m := range d.Missing {
		if !slices.Contains(out, m.Step) {
			out = append(out, m.Step)
		}
	}
	return out
}

func (h *harness) app(r req, wantStatus int) appDetail {
	h.t.Helper()
	w := h.do(r)
	if w.Code != wantStatus {
		h.t.Fatalf("%s %s: %d, want %d: %s", r.method, r.path, w.Code, wantStatus, w.Body.String())
	}
	var d appDetail
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		h.t.Fatal(err)
	}
	return d
}

func (h *harness) start(s *session, programme string, intake any) *httpResponse {
	h.t.Helper()
	w := h.do(req{method: http.MethodPost, path: "/api/v1/applicant/applications", body: map[string]any{"programme": programme, "intake_id": intake}, sess: s})
	return &httpResponse{w.Code, w.Body.String()}
}

func idOf(t *testing.T, r *httpResponse) string {
	t.Helper()
	var b struct{ ID string }
	if err := json.Unmarshal([]byte(r.body), &b); err != nil || b.ID == "" {
		t.Fatalf("no id in %s", r.body)
	}
	return b.ID
}

func (h *harness) put(s *session, id, step string, body any, wantStatus int) appDetail {
	h.t.Helper()
	return h.app(req{method: http.MethodPut, path: "/api/v1/applicant/applications/" + id + "/" + step, body: body, sess: s}, wantStatus)
}

var referencePattern = regexp.MustCompile(`^DTC-APP-\d{4}-\d{6}$`)

// AC-04 (start): one draft per programme and intake, with a reference
// number that is emailed.
func TestStartApplication(t *testing.T) {
	h := newHarness(t)
	slug, intakes, closed := h.openProgramme(1)
	a := h.newApplicant("applicant010@example.invalid", "Test Applicant 010")

	r := h.start(a.session, slug, nil)
	r.expect(t, http.StatusCreated, "")
	id := idOf(t, r)
	d := h.app(req{method: http.MethodGet, path: "/api/v1/applicant/applications/" + id, sess: a.session}, http.StatusOK)
	if !referencePattern.MatchString(d.Reference) || !strings.HasSuffix(d.Reference, "-000001") || d.Status != "draft" || !d.Editable {
		t.Fatalf("detail = %+v", d)
	}
	// One open intake is chosen for the applicant; the name typed at sign-in
	// fills the name fields.
	if d.IntakeID != intakes[0] || d.Personal["first_name"] != "Test" || d.Personal["last_name"] != "Applicant 010" {
		t.Fatalf("detail = %+v", d)
	}
	if got := d.missingSteps(); !slices.Equal(got, []string{"personal", "education", "sponsor"}) {
		t.Fatalf("missing steps = %v", got)
	}

	// The reference is emailed.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && h.mailsTo("applicant010@example.invalid") < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	m := h.mailer.All()[len(h.mailer.All())-1]
	if m.Subject != "Your application reference: "+d.Reference || !strings.Contains(m.Text, "Diploma in Testing (Sample)") {
		t.Fatalf("started email = %+v", m)
	}

	// Starting again returns the same draft, and no second email.
	again := h.start(a.session, slug, intakes[0])
	again.expect(t, http.StatusOK, "")
	if idOf(t, again) != id {
		t.Fatal("second start made another application")
	}
	time.Sleep(50 * time.Millisecond)
	if n := h.mailsTo("applicant010@example.invalid"); n != 2 {
		t.Fatalf("emails = %d, want sign-in and one reference", n)
	}

	// Refusals.
	h.start(a.session, slug, closed).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	h.start(a.session, "no-such-programme", nil).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	h.exec(`UPDATE programme_intakes SET status = 'closed' WHERE id = $1`, intakes[0])
	h.start(a.session, slug, nil).expect(t, http.StatusConflict, "applications_closed")

	// Two programmes with two open intakes: the intake must be chosen.
	slug2, intakes2, _ := h.openProgramme(2)
	h.start(a.session, slug2, nil).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	second := h.start(a.session, slug2, intakes2[1])
	second.expect(t, http.StatusCreated, "")
	d2 := h.app(req{method: http.MethodGet, path: "/api/v1/applicant/applications/" + idOf(t, second), sess: a.session}, http.StatusOK)
	if !strings.HasSuffix(d2.Reference, "-000002") {
		t.Fatalf("second reference = %s", d2.Reference)
	}
	if n := h.auditCount("application.started"); n != 2 {
		t.Fatalf("starts audited %d times, want 2", n)
	}
	var history int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM application_status_history WHERE to_status = 'draft' AND changed_by IS NULL`).Scan(&history); err != nil || history != 2 {
		t.Fatalf("history rows = %d (%v)", history, err)
	}

	// Moving to the other open intake of the second programme works; to the
	// first programme's intake does not.
	h.put(a.session, d2.ID, "intake", map[string]any{"intake_id": intakes2[0]}, http.StatusOK)
	h.put(a.session, d2.ID, "intake", map[string]any{"intake_id": intakes[0]}, http.StatusUnprocessableEntity)
}

// A double tap on "Start" makes one application.
func TestConcurrentStartsMakeOneApplication(t *testing.T) {
	h := newHarness(t)
	slug, _, _ := h.openProgramme(1)
	a := h.newApplicant("applicant011@example.invalid", "")
	codes := make(chan int, 3)
	for range 3 {
		go func() { codes <- h.start(a.session, slug, nil).code }()
	}
	got := []int{<-codes, <-codes, <-codes}
	slices.Sort(got)
	if !slices.Equal(got, []int{http.StatusOK, http.StatusOK, http.StatusCreated}) {
		t.Fatalf("status codes %v, want one 201 and two 200", got)
	}
	var n int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM applications`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("applications = %d (%v)", n, err)
	}
}

// Every step saves on its own; missing items follow the answers, and an
// under-18 applicant needs a guardian who has agreed (AC-13).
func TestApplicationSteps(t *testing.T) {
	h := newHarness(t)
	slug, _, _ := h.openProgramme(1)
	a := h.newApplicant("applicant012@example.invalid", "")
	id := idOf(t, h.start(a.session, slug, nil))

	// Badly formed values are refused with a message per field.
	w := h.do(req{method: http.MethodPut, path: "/api/v1/applicant/applications/" + id + "/personal", sess: a.session, body: map[string]any{
		"date_of_birth": "21/03/2009", "county": "Atlantis", "identity_document_type": "library_card", "phone": "call me",
	}})
	expectStatus(t, w, http.StatusUnprocessableEntity, "validation_failed")
	for _, f := range []string{"date_of_birth", "county", "identity_document_type", "phone"} {
		if !strings.Contains(w.Body.String(), `"field":"`+f+`"`) {
			t.Errorf("no problem reported for %s: %s", f, w.Body.String())
		}
	}

	// A half-filled step saves.
	d := h.put(a.session, id, "personal", map[string]any{"first_name": " Test ", "last_name": "Applicant 012"}, http.StatusOK)
	if d.Personal["first_name"] != "Test" || d.NeedsGuardian {
		t.Fatalf("after partial save: %+v", d)
	}

	// Under 18 today: the guardian step appears.
	dob := time.Now().AddDate(-16, 0, 0).Format(time.DateOnly)
	personal := map[string]any{
		"first_name": "Test", "last_name": "Applicant 012", "date_of_birth": dob, "identity_document_type": "birth_certificate",
		"identity_document": "bc 000 0012", "nationality": "Kenyan", "county": "Nairobi", "phone": "+254 700 000 012",
	}
	d = h.put(a.session, id, "personal", personal, http.StatusOK)
	if !d.NeedsGuardian || d.Personal["identity_document"] != "BC 000 0012" || !slices.Contains(d.missingSteps(), "guardian") || slices.Contains(d.missingSteps(), "personal") {
		t.Fatalf("minor: %+v", d)
	}

	// Consent needs a name first.
	h.put(a.session, id, "guardian", map[string]any{"consent_given": true}, http.StatusUnprocessableEntity)
	guardian := map[string]any{"full_name": "Test Guardian 012", "relationship": "mother", "phone": "+254 700 100 012", "email": "", "consent_given": true}
	d = h.put(a.session, id, "guardian", guardian, http.StatusOK)
	if !d.Guardian.ConsentGiven || slices.Contains(d.missingSteps(), "guardian") {
		t.Fatalf("guardian: %+v", d)
	}
	// Saving the same again records nothing new; a new name needs consent again.
	h.put(a.session, id, "guardian", guardian, http.StatusOK)
	guardian["full_name"], guardian["consent_given"] = "Test Guardian 013", false
	d = h.put(a.session, id, "guardian", guardian, http.StatusOK)
	if d.Guardian.ConsentGiven {
		t.Fatal("consent carried over to a different guardian name")
	}
	guardian["consent_given"] = true
	h.put(a.session, id, "guardian", guardian, http.StatusOK)
	guardian["consent_given"] = false
	d = h.put(a.session, id, "guardian", guardian, http.StatusOK)
	if d.Guardian.ConsentGiven {
		t.Fatal("withdrawn consent still counts")
	}
	rows, err := h.pool.Query(context.Background(), `SELECT given, given_by_name, notice_version FROM consents WHERE purpose = 'guardian_consent' ORDER BY captured_at, id`)
	if err != nil {
		t.Fatal(err)
	}
	var trail []string
	for rows.Next() {
		var given bool
		var name, version string
		if err := rows.Scan(&given, &name, &version); err != nil {
			t.Fatal(err)
		}
		trail = append(trail, fmt.Sprintf("%v %s %s", given, name, version))
	}
	rows.Close()
	want := []string{"true Test Guardian 012 draft-2026-10", "true Test Guardian 013 draft-2026-10", "false Test Guardian 013 draft-2026-10"}
	if !slices.Equal(trail, want) {
		t.Fatalf("consent trail = %v, want %v", trail, want)
	}
	guardian["consent_given"] = true
	h.put(a.session, id, "guardian", guardian, http.StatusOK)

	// Education: KCSE grades are checked; blank subject rows are dropped.
	h.put(a.session, id, "education", map[string]any{"items": []map[string]any{{"level": "kcse", "institution": "Test School", "mean_grade": "Z"}}}, http.StatusUnprocessableEntity)
	h.put(a.session, id, "education", map[string]any{"items": []map[string]any{{"level": "kcse", "institution": "Test School", "year_completed": 1900}}}, http.StatusUnprocessableEntity)
	d = h.put(a.session, id, "education", map[string]any{"items": []map[string]any{{
		"level": "kcse", "institution": "Test Secondary School", "year_completed": time.Now().Year() - 1, "mean_grade": "c+",
		"subjects": []map[string]string{{"subject": "English", "grade": "b-"}, {"subject": "", "grade": ""}, {"subject": "Mathematics", "grade": "C"}},
	}}}, http.StatusOK)
	subjects, _ := d.Education[0]["subjects"].([]any)
	if d.Education[0]["mean_grade"] != "C+" || len(subjects) != 2 || slices.Contains(d.missingSteps(), "education") {
		t.Fatalf("education: %+v", d.Education)
	}

	// Sponsor: someone else paying needs a name and phone.
	d = h.put(a.session, id, "sponsor", map[string]any{"type": "parent_guardian"}, http.StatusOK)
	if !slices.Contains(d.missingSteps(), "sponsor") {
		t.Fatal("sponsor without a name is not reported missing")
	}
	d = h.put(a.session, id, "sponsor", map[string]any{"type": "self", "name": "ignored", "phone": ""}, http.StatusOK)
	if len(d.Missing) != 0 {
		t.Fatalf("still missing: %+v", d.Missing)
	}

	// Turning 18 by a corrected date removes the guardian requirement.
	personal["date_of_birth"] = time.Now().AddDate(-19, 0, 0).Format(time.DateOnly)
	if d = h.put(a.session, id, "personal", personal, http.StatusOK); d.NeedsGuardian {
		t.Fatal("adult still needs a guardian")
	}
	personal["date_of_birth"] = time.Now().AddDate(1, 0, 0).Format(time.DateOnly)
	h.put(a.session, id, "personal", personal, http.StatusUnprocessableEntity)
}

// AC-08 for applicants: nobody else can read or change an application.
func TestApplicationsAreOwnersOnly(t *testing.T) {
	h := newHarness(t)
	slug, _, _ := h.openProgramme(1)
	owner := h.newApplicant("applicant013@example.invalid", "")
	other := h.newApplicant("applicant014@example.invalid", "")
	id := idOf(t, h.start(owner.session, slug, nil))

	path := "/api/v1/applicant/applications/" + id
	expectStatus(t, h.do(req{method: http.MethodGet, path: path, sess: other.session}), http.StatusNotFound, "not_found")
	expectStatus(t, h.do(req{method: http.MethodPut, path: path + "/personal", body: map[string]any{"first_name": "X"}, sess: other.session}), http.StatusNotFound, "not_found")
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/applicant/applications/not-a-uuid", sess: owner.session}), http.StatusNotFound, "not_found")

	h.firstSuperAdmin("owner@example.invalid")
	staff := h.mustLogin("owner@example.invalid")
	expectStatus(t, h.do(req{method: http.MethodGet, path: path, sess: staff}), http.StatusForbidden, "forbidden")
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/applicant/applications", body: map[string]any{"programme": slug}, sess: staff}), http.StatusForbidden, "forbidden")

	// Without the CSRF token nothing changes.
	expectStatus(t, h.do(req{method: http.MethodPut, path: path + "/personal", body: map[string]any{"first_name": "X"}, sess: owner.session, noCSRF: true}), http.StatusForbidden, "csrf_failed")
}

// Brief 8: a draft expires after the configured period without a save, and
// is then read-only, with the change in its history.
func TestDraftExpiry(t *testing.T) {
	h := newHarness(t)
	h.exec(`UPDATE site_settings SET value = '2' WHERE key = 'applications.draft_expiry_days'`)
	slug, _, _ := h.openProgramme(1)
	a := h.newApplicant("applicant015@example.invalid", "")
	id := idOf(t, h.start(a.session, slug, nil))
	path := "/api/v1/applicant/applications/" + id

	// A save a day later moves the expiry on.
	h.now = func() time.Time { return time.Now().Add(36 * time.Hour) }
	h.put(a.session, id, "sponsor", map[string]any{"type": "self"}, http.StatusOK)
	h.now = func() time.Time { return time.Now().Add(60 * time.Hour) }
	if d := h.app(req{method: http.MethodGet, path: path, sess: a.session}, http.StatusOK); d.Status != "draft" {
		t.Fatalf("expired although saved within the period: %s", d.Status)
	}

	h.now = func() time.Time { return time.Now().Add(90 * time.Hour) }
	d := h.app(req{method: http.MethodGet, path: path, sess: a.session}, http.StatusOK)
	if d.Status != "expired" || d.Editable {
		t.Fatalf("after expiry: %+v", d)
	}
	expectStatus(t, h.do(req{method: http.MethodPut, path: path + "/sponsor", body: map[string]any{"type": "self"}, sess: a.session}), http.StatusConflict, "not_editable")
	var reason string
	if err := h.pool.QueryRow(context.Background(), `SELECT reason FROM application_status_history WHERE application_id = $1 AND to_status = 'expired'`, id).Scan(&reason); err != nil || reason == "" {
		t.Fatalf("expiry history: %q (%v)", reason, err)
	}
	// An expired draft no longer blocks a fresh start for the same intake.
	h.start(a.session, slug, nil).expect(t, http.StatusCreated, "")
	h.now = time.Now
}

// After signing in, the applicant lands where they were going.
func TestSignInLinkCarriesWhereToGo(t *testing.T) {
	h := newHarness(t)
	h.do(req{method: http.MethodPost, path: "/api/v1/applicant/sign-in-link", body: map[string]string{
		"email": "applicant016@example.invalid", "next": "/apply/new?programme=diploma-in-testing",
	}})
	w := h.do(req{method: http.MethodPost, path: "/api/v1/applicant/sign-in", body: map[string]string{"token": h.linkFrom("applicant016@example.invalid", 1)}})
	if !strings.Contains(w.Body.String(), `"next":"/apply/new?programme=diploma-in-testing"`) {
		t.Fatalf("sign-in: %s", w.Body.String())
	}
	h.do(req{method: http.MethodPost, path: "/api/v1/applicant/sign-in-link", body: map[string]string{
		"email": "applicant016@example.invalid", "next": "https://evil.example/apply/",
	}})
	w = h.do(req{method: http.MethodPost, path: "/api/v1/applicant/sign-in", body: map[string]string{"token": h.linkFrom("applicant016@example.invalid", 2)}})
	if !strings.Contains(w.Body.String(), `"next":""`) {
		t.Fatalf("sign-in kept an outside address: %s", w.Body.String())
	}
}

func TestCleanNext(t *testing.T) {
	for in, want := range map[string]string{
		"/apply/new?programme=a&intake=b": "/apply/new?programme=a&intake=b",
		"/apply/applications/123":         "/apply/applications/123",
		"/staff":                          "",
		"//evil.example/apply/":           "",
		"/apply/../staff":                 "",
		"/apply//evil.example":            "",
		"/apply/x\\y":                     "",
		"/apply/x y":                      "",
		"https://evil.example/apply/":     "",
	} {
		if got := applicant.CleanNext(in); got != want {
			t.Errorf("CleanNext(%q) = %q, want %q", in, got, want)
		}
	}
}
