package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MboyaDan/dakota-college/backend/internal/admissions"
	"github.com/MboyaDan/dakota-college/backend/internal/application"
)

// staffDetail is the part of the staff view the tests read.
type staffDetail struct {
	ID        string   `json:"id"`
	Reference string   `json:"reference"`
	Status    string   `json:"status"`
	Actions   []string `json:"actions"`
	Applicant struct {
		Name, Email string
	}
	AssigneeName *string `json:"assignee_name"`
	Submitted    *struct {
		Reference string
		Personal  map[string]string
	} `json:"submitted"`
	Documents []struct {
		Kind            string
		AfterSubmission bool `json:"after_submission"`
	}
	History []struct {
		From          *string
		To            string
		Reason        string
		ChangedByName *string `json:"changed_by_name"`
	}
	Notes        []struct{ Body string }
	InfoRequests []struct {
		Message, Response string
		ClosedAt          *time.Time `json:"closed_at"`
		ClosedByName      *string    `json:"closed_by_name"`
	} `json:"info_requests"`
	Offer *struct {
		RespondBy  string     `json:"respond_by"`
		Conditions string     `json:"conditions"`
		AcceptedAt *time.Time `json:"accepted_at"`
		DeclinedAt *time.Time `json:"declined_at"`
		HasLetter  bool       `json:"has_letter"`
	}
	Student *struct {
		ID            string `json:"id"`
		StudentNumber string `json:"student_number"`
		InvoiceID     string `json:"invoice_id"`
		InvoiceStatus string `json:"invoice_status"`
	}
}

func (h *harness) staffApp(s *session, id string, want int) staffDetail {
	h.t.Helper()
	w := h.do(req{method: http.MethodGet, path: "/api/v1/staff/applications/" + id, sess: s})
	expectStatus(h.t, w, want, "")
	var d staffDetail
	if want == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
			h.t.Fatal(err)
		}
	}
	return d
}

// staffAct posts one workflow action and returns the response.
func (h *harness) staffAct(s *session, id, action string, body any) *httpResponse {
	h.t.Helper()
	if body == nil {
		body = map[string]any{}
	}
	w := h.do(req{method: http.MethodPost, path: "/api/v1/staff/applications/" + id + "/" + action, body: body, sess: s})
	return &httpResponse{w.Code, w.Body.String()}
}

// applicantAct posts one of the applicant's own actions.
func (h *harness) applicantAct(s *session, id, action string, body any) *httpResponse {
	h.t.Helper()
	w := h.do(req{method: http.MethodPost, path: "/api/v1/applicant/applications/" + id + "/" + action, body: body, sess: s})
	return &httpResponse{w.Code, w.Body.String()}
}

// submitted is a submitted application with its three documents.
func (h *harness) submitted(email string) (*applicantSession, string) {
	h.t.Helper()
	a, id := h.readyApplication(email)
	h.uploadAll(a.session, id)
	h.submit(a.session, id, "key-"+uuid.NewString(), true).expect(h.t, http.StatusOK, "")
	return a, id
}

// waitMail waits for an email to `to` whose subject contains `subject`.
func (h *harness) waitMail(to, subject string) string {
	h.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, m := range h.mailer.All() {
			if m.To == to && strings.Contains(m.Subject, subject) {
				return m.Text
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatalf("no email to %s with subject containing %q", to, subject)
	return ""
}

func (h *harness) historyCount(id string) int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM application_status_history WHERE application_id = $1`, id).Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}

func (h *harness) statusOf(id string) string {
	h.t.Helper()
	var s string
	if err := h.pool.QueryRow(context.Background(), `SELECT status FROM applications WHERE id = $1`, id).Scan(&s); err != nil {
		h.t.Fatal(err)
	}
	return s
}

// The Go state machine and the database's table are the same list.
func TestTransitionTableMatchesDatabase(t *testing.T) {
	h := newHarness(t)
	rows, err := h.pool.Query(context.Background(), `SELECT from_status, to_status FROM application_transitions`)
	if err != nil {
		t.Fatal(err)
	}
	db := map[string][]string{}
	for rows.Next() {
		var from, to string
		if err := rows.Scan(&from, &to); err != nil {
			t.Fatal(err)
		}
		db[from] = append(db[from], to)
	}
	rows.Close()
	if len(db) != len(application.Transitions) {
		t.Fatalf("database has moves from %d statuses, Go from %d", len(db), len(application.Transitions))
	}
	for from, tos := range application.Transitions {
		got := slices.Clone(db[from])
		want := slices.Clone(tos)
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("from %s: database %v, Go %v", from, got, want)
		}
	}
}

// Brief 8: every move between statuses that is not in the state machine is
// refused by the database itself, whatever code path tries it.
func TestDatabaseRefusesEveryIllegalTransition(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	a := h.newApplicant("applicant100@example.invalid", "Test Applicant 100")
	slug, _, _ := h.openProgramme(1)
	id := idOf(t, h.start(a.session, slug, nil))

	tried, refused := 0, 0
	for _, from := range application.Statuses {
		for _, to := range application.Statuses {
			if from == to {
				continue
			}
			tried++
			err := pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
				// Put the row in `from` without the trigger, then try the move.
				if _, err := tx.Exec(ctx, `SET LOCAL session_replication_role = replica`); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `UPDATE applications SET status = $2, submitted_at = now(), declaration_accepted_at = now(),
					submitted_snapshot = '{}' WHERE id = $1`, id, from); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `SET LOCAL session_replication_role = origin`); err != nil {
					return err
				}
				_, moveErr := tx.Exec(ctx, `UPDATE applications SET status = $2 WHERE id = $1`, id, to)
				var pg *pgconn.PgError
				allowed := application.CanMove(from, to)
				switch {
				case allowed && moveErr != nil:
					t.Errorf("%s -> %s is allowed but was refused: %v", from, to, moveErr)
				case !allowed && moveErr == nil:
					t.Errorf("%s -> %s is not allowed but the database accepted it", from, to)
				case !allowed && (!errors.As(moveErr, &pg) || pg.Code != "23514"):
					t.Errorf("%s -> %s: unexpected error %v", from, to, moveErr)
				case !allowed:
					refused++
				}
				return errRollback
			})
			if err != nil && !errors.Is(err, errRollback) {
				t.Fatalf("%s -> %s: %v", from, to, err)
			}
		}
	}
	allowed := 0
	for _, tos := range application.Transitions {
		allowed += len(tos)
	}
	if refused != tried-allowed || allowed != 17 {
		t.Fatalf("refused %d of %d moves; %d allowed", refused, tried, allowed)
	}
	// A new application cannot start anywhere but draft.
	_, err := h.pool.Exec(ctx, `INSERT INTO applications (reference, applicant_id, programme_id, status, submitted_at, declaration_accepted_at, submitted_snapshot)
		SELECT 'DTC-APP-TEST-1', applicant_id, programme_id, 'offered', now(), now(), '{}' FROM applications WHERE id = $1`, id)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "23514" {
		t.Fatalf("insert as offered: %v", err)
	}
}

var errRollback = errors.New("rollback")

// workflowActors are the staff accounts the workflow tests use.
type workflowActors struct {
	officer, officer2, registrar, head, editor *session
	officerID, officer2ID                      uuid.UUID
}

func (h *harness) actors() workflowActors {
	h.t.Helper()
	var w workflowActors
	w.officerID = h.createUser("officer@example.invalid", "admissions_officer")
	w.officer2ID = h.createUser("officer2@example.invalid", "admissions_officer")
	h.createUser("registrar@example.invalid", "registrar")
	h.createUser("head@example.invalid", "head_of_school")
	h.createUser("editor@example.invalid", "content_editor")
	w.officer = h.mustLogin("officer@example.invalid")
	w.officer2 = h.mustLogin("officer2@example.invalid")
	w.registrar = h.mustLogin("registrar@example.invalid")
	w.head = h.mustLogin("head@example.invalid")
	w.editor = h.mustLogin("editor@example.invalid")
	return w
}

var studentNumberPattern = regexp.MustCompile(`^DTC/\d{4}/\d{4}$`)

// Brief 8 end to end: review, request information, reply, offer with a
// letter, acceptance and enrolment, each change recorded with who and why.
func TestAdmissionsWorkflow(t *testing.T) {
	h := newHarness(t)
	w := h.actors()
	const email = "applicant101@example.invalid"
	a, id := h.submitted(email)
	if _, d := h.readyApplication("applicant102@example.invalid"); d == "" {
		t.Fatal("no draft")
	}

	// The inbox lists submitted applications only.
	r := h.do(req{method: http.MethodGet, path: "/api/v1/staff/applications", sess: w.officer})
	expectStatus(t, r, http.StatusOK, "")
	var page struct {
		Items []struct{ ID, Status string }
		Total int
	}
	if err := json.Unmarshal(r.Body.Bytes(), &page); err != nil || page.Total != 1 || page.Items[0].ID != id || page.Items[0].Status != "submitted" {
		t.Fatalf("inbox: %s", r.Body.String())
	}
	d := h.staffApp(w.officer, id, http.StatusOK)
	if d.Submitted == nil || d.Submitted.Reference != d.Reference || len(d.Documents) != 3 ||
		!slices.Equal(d.Actions, []string{"assign", "note"}) {
		t.Fatalf("detail: %+v", d)
	}

	// Assigning starts the review.
	h.staffAct(w.officer, id, "assign", map[string]any{"assignee_id": w.officerID}).expect(t, http.StatusOK, "")
	h.staffAct(w.officer, id, "assign", map[string]any{"assignee_id": w.officerID}).expect(t, http.StatusConflict, "unchanged")
	h.staffAct(w.officer, id, "assign", map[string]any{"assignee_id": uuid.New()}).expect(t, http.StatusUnprocessableEntity, "not_assignable")
	h.staffAct(w.officer, id, "notes", map[string]any{"body": "KCSE slip is blurred."}).expect(t, http.StatusOK, "")
	d = h.staffApp(w.officer, id, http.StatusOK)
	if d.Status != "under_review" || len(d.Notes) != 1 || !slices.Contains(d.Actions, "offer") {
		t.Fatalf("after assign: %+v", d)
	}

	// Ask for more; the applicant sees the request, uploads and replies.
	h.staffAct(w.officer, id, "request-info", map[string]any{"message": ""}).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	h.staffAct(w.officer, id, "request-info", map[string]any{"message": "Please upload a clearer KCSE result slip."}).expect(t, http.StatusOK, "")
	if text := h.waitMail(email, "we need more information"); !strings.Contains(text, "clearer KCSE result slip") {
		t.Fatalf("info email: %s", text)
	}
	own := h.do(req{method: http.MethodGet, path: "/api/v1/applicant/applications/" + id, sess: a.session})
	if !strings.Contains(own.Body.String(), `"info_request":{"message":"Please upload a clearer KCSE result slip."`) ||
		!strings.Contains(own.Body.String(), `"can_upload":true`) {
		t.Fatalf("applicant view: %s", own.Body.String())
	}
	// Documents sent before the request stay; new ones can be added and removed.
	var firstDoc string
	if err := h.pool.QueryRow(context.Background(), `SELECT id FROM application_documents WHERE application_id = $1 ORDER BY created_at LIMIT 1`, id).Scan(&firstDoc); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, h.do(req{method: http.MethodDelete, path: "/api/v1/applicant/applications/" + id + "/documents/" + firstDoc, sess: a.session}), http.StatusConflict, "not_editable")
	extra := uploadedID(t, h.uploadDoc(a.session, id, "kcse_certificate", "kcse-clear.pdf", testPDF))
	expectStatus(t, h.do(req{method: http.MethodDelete, path: "/api/v1/applicant/applications/" + id + "/documents/" + extra, sess: a.session}), http.StatusOK, "")
	h.uploadDoc(a.session, id, "kcse_certificate", "kcse-clear.pdf", testPDF).expect(t, http.StatusCreated, "")
	h.applicantAct(a.session, id, "reply", map[string]any{"message": " "}).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	h.applicantAct(a.session, id, "reply", map[string]any{"message": "Uploaded a clearer copy."}).expect(t, http.StatusOK, "")
	h.applicantAct(a.session, id, "reply", map[string]any{"message": "Again"}).expect(t, http.StatusConflict, "not_allowed")
	d = h.staffApp(w.officer, id, http.StatusOK)
	if d.Status != "under_review" || len(d.InfoRequests) != 1 || d.InfoRequests[0].Response != "Uploaded a clearer copy." ||
		d.InfoRequests[0].ClosedByName != nil || len(d.Documents) != 4 || !d.Documents[3].AfterSubmission || d.Documents[0].AfterSubmission {
		t.Fatalf("after reply: %+v", d)
	}
	// The submitted copy never changes.
	var snapDocs int
	if err := h.pool.QueryRow(context.Background(), `SELECT jsonb_array_length(submitted_snapshot->'documents') FROM applications WHERE id = $1`, id).Scan(&snapDocs); err != nil || snapDocs != 3 {
		t.Fatalf("snapshot documents = %d (%v)", snapDocs, err)
	}

	// Offer: a dated letter, kept privately, reachable by link.
	h.exec(`INSERT INTO fee_items (programme_id, label, category, amount_cents, frequency) SELECT programme_id, 'Tuition (Sample)', 'tuition', 4500000, 'per_term' FROM applications WHERE id = $1`, id)
	h.staffAct(w.officer, id, "offer", map[string]any{"respond_by": time.Now().Format(time.DateOnly)}).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	h.staffAct(w.officer, id, "offer", map[string]any{"conditions": "Bring your original KCSE certificate."}).expect(t, http.StatusOK, "")
	d = h.staffApp(w.officer, id, http.StatusOK)
	wantBy := time.Now().In(time.FixedZone("EAT", 3*3600)).AddDate(0, 0, 14).Format(time.DateOnly)
	if d.Status != "offered" || d.Offer == nil || d.Offer.RespondBy != wantBy || !d.Offer.HasLetter || d.Offer.Conditions == "" {
		t.Fatalf("after offer: %+v", d.Offer)
	}
	if text := h.waitMail(email, "Offer of a place"); !strings.Contains(text, "/apply/application?id="+id) {
		t.Fatalf("offer email: %s", text)
	}
	letter := h.link(req{method: http.MethodPost, path: "/api/v1/staff/applications/" + id + "/offer-letter/link", sess: w.officer})
	lw := h.do(req{method: http.MethodGet, path: letter, sess: w.officer})
	if lw.Code != http.StatusOK || !bytes.HasPrefix(lw.Body.Bytes(), []byte("%PDF-")) || lw.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("letter: %d %s", lw.Code, lw.Header())
	}
	own2 := h.link(req{method: http.MethodPost, path: "/api/v1/applicant/applications/" + id + "/offer-letter/link", sess: a.session})
	if lw := h.do(req{method: http.MethodGet, path: own2, sess: a.session}); lw.Code != http.StatusOK {
		t.Fatalf("applicant letter: %d", lw.Code)
	}
	if n := h.privateObjects(); n != 5 {
		t.Fatalf("private objects = %d, want 4 documents and the letter", n)
	}

	// Accepting twice changes nothing the second time.
	h.applicantAct(a.session, id, "accept-offer", map[string]any{}).expect(t, http.StatusOK, "")
	h.applicantAct(a.session, id, "accept-offer", map[string]any{}).expect(t, http.StatusConflict, "not_allowed")

	// Enrolment needs admissions.enrol: the officer cannot, the registrar can.
	h.staffAct(w.officer, id, "enrol", nil).expect(t, http.StatusForbidden, "forbidden")
	if d = h.staffApp(w.officer, id, http.StatusOK); slices.Contains(d.Actions, "enrol") {
		t.Fatalf("officer offered enrol: %v", d.Actions)
	}
	if d = h.staffApp(w.registrar, id, http.StatusOK); !slices.Contains(d.Actions, "enrol") {
		t.Fatalf("registrar not offered enrol: %v", d.Actions)
	}
	h.staffAct(w.registrar, id, "enrol", nil).expect(t, http.StatusOK, "")
	h.staffAct(w.registrar, id, "enrol", nil).expect(t, http.StatusConflict, "not_allowed")
	d = h.staffApp(w.registrar, id, http.StatusOK)
	if d.Status != "enrolled" || d.Student == nil || !studentNumberPattern.MatchString(d.Student.StudentNumber) ||
		!strings.HasSuffix(d.Student.StudentNumber, "/0001") || d.Student.InvoiceStatus != "draft" || len(d.Actions) != 1 {
		t.Fatalf("after enrol: %+v %v", d.Student, d.Actions)
	}
	if text := h.waitMail(email, "Welcome"); !strings.Contains(text, d.Student.StudentNumber) {
		t.Fatalf("enrolled email: %s", text)
	}
	// One account per person: now a student, signed out, with the Student role.
	var kind string
	var roles, lines int
	if err := h.pool.QueryRow(context.Background(), `SELECT u.kind, (SELECT count(*) FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE ur.user_id = u.id AND r.key = 'student')
		FROM users u WHERE u.email = $1`, email).Scan(&kind, &roles); err != nil || kind != "student" || roles != 1 {
		t.Fatalf("account kind %q, student role %d (%v)", kind, roles, err)
	}
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/applicant/applications/" + id, sess: a.session}), http.StatusUnauthorized, "")
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM invoices WHERE student_id = $1 AND number IS NULL`, d.Student.ID).Scan(&lines); err != nil || lines != 1 {
		t.Fatalf("draft invoices = %d (%v)", lines, err)
	}
	// Student accounts never get applicant sign-in links.
	before := h.mailsTo(email)
	h.requestLink(email, "")
	time.Sleep(50 * time.Millisecond)
	if h.mailsTo(email) != before {
		t.Fatal("a student account was sent an applicant link")
	}

	// The whole history, with who made each change.
	var steps []string
	for _, e := range d.History {
		who := "applicant"
		if e.ChangedByName != nil {
			who = *e.ChangedByName
		}
		steps = append(steps, e.To+" by "+who)
	}
	want := []string{
		"draft by applicant", "submitted by applicant", "under_review by Test User officer@example.invalid",
		"info_requested by Test User officer@example.invalid", "under_review by applicant",
		"offered by Test User officer@example.invalid", "offer_accepted by applicant", "enrolled by Test User registrar@example.invalid",
	}
	if !slices.Equal(steps, want) {
		t.Fatalf("history:\n%v\nwant\n%v", steps, want)
	}
	for _, action := range []string{"application.assigned", "application.note_added", "application.info_requested", "application.info_replied",
		"application.offered", "application.offer_accepted", "application.enrolled", "student.created"} {
		if n := h.auditCount(action); n != 1 {
			t.Errorf("%s audited %d times", action, n)
		}
	}

	// The report counts it all.
	rep := h.do(req{method: http.MethodGet, path: "/api/v1/staff/applications/report", sess: w.head})
	expectStatus(t, rep, http.StatusOK, "")
	var report struct {
		Totals map[string]int
		Rows   []map[string]any
	}
	if err := json.Unmarshal(rep.Body.Bytes(), &report); err != nil || len(report.Rows) != 1 ||
		report.Totals["submitted"] != 1 || report.Totals["offered"] != 1 || report.Totals["accepted"] != 1 || report.Totals["enrolled"] != 1 || report.Totals["in_review"] != 0 {
		t.Fatalf("report: %s", rep.Body.String())
	}
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/staff/applications/report?from=2026-12-01&to=2026-01-01", sess: w.head}), http.StatusUnprocessableEntity, "validation_failed")
}

// Brief 8 done-when: every action the API offers is refused from every
// status the state machine does not allow it from, and nothing changes.
func TestEveryIllegalTransitionIsRefusedByTheAPI(t *testing.T) {
	h := newHarness(t)
	w := h.actors()
	n := 0
	next := func() string { n++; return fmt.Sprintf("applicant%03d@example.invalid", 110+n) }

	type app struct {
		id string
		a  *applicantSession
	}
	apps := map[string]app{}
	_, draft := h.readyApplication(next())
	apps["draft"] = app{id: draft}
	mk := func(status string, steps func(id string, a *applicantSession)) {
		a, id := h.submitted(next())
		steps(id, a)
		if got := h.statusOf(id); got != status {
			t.Fatalf("setting up %s: status %s", status, got)
		}
		apps[status] = app{id: id, a: a}
	}
	review := func(id string) {
		h.staffAct(w.officer, id, "assign", map[string]any{"assignee_id": w.officerID}).expect(t, http.StatusOK, "")
	}
	offer := func(id string) {
		review(id)
		h.staffAct(w.officer, id, "offer", nil).expect(t, http.StatusOK, "")
	}
	mk("submitted", func(string, *applicantSession) {})
	mk("under_review", func(id string, _ *applicantSession) { review(id) })
	mk("info_requested", func(id string, _ *applicantSession) {
		review(id)
		h.staffAct(w.officer, id, "request-info", map[string]any{"message": "More please."}).expect(t, http.StatusOK, "")
	})
	mk("offered", func(id string, _ *applicantSession) { offer(id) })
	mk("offer_accepted", func(id string, a *applicantSession) {
		offer(id)
		h.applicantAct(a.session, id, "accept-offer", map[string]any{}).expect(t, http.StatusOK, "")
	})
	mk("enrolled", func(id string, a *applicantSession) {
		offer(id)
		h.applicantAct(a.session, id, "accept-offer", map[string]any{}).expect(t, http.StatusOK, "")
		h.staffAct(w.registrar, id, "enrol", nil).expect(t, http.StatusOK, "")
	})
	mk("rejected", func(id string, _ *applicantSession) {
		review(id)
		h.staffAct(w.officer, id, "reject", map[string]any{"reason": "Entry requirements not met."}).expect(t, http.StatusOK, "")
	})
	mk("withdrawn", func(id string, a *applicantSession) {
		h.applicantAct(a.session, id, "withdraw", map[string]any{"reason": "Found another course."}).expect(t, http.StatusOK, "")
	})
	mk("expired", func(id string, _ *applicantSession) {
		offer(id)
		h.exec(`UPDATE offers SET respond_by = current_date - 1 WHERE application_id = $1`, id)
		h.staffApp(w.officer, id, http.StatusOK) // looking at it lapses the offer
	})

	staff := []struct {
		action  string
		body    map[string]any
		allowed []string
	}{
		{"assign", map[string]any{"assignee_id": w.officer2ID}, []string{"submitted", "under_review", "info_requested"}},
		{"request-info", map[string]any{"message": "More."}, []string{"under_review"}},
		{"resume-review", map[string]any{}, []string{"info_requested"}},
		{"offer", map[string]any{}, []string{"under_review"}},
		{"reject", map[string]any{"reason": "No."}, []string{"under_review", "info_requested"}},
		{"enrol", map[string]any{}, []string{"offer_accepted"}},
	}
	applicant := []struct {
		action  string
		body    map[string]any
		allowed []string
	}{
		{"reply", map[string]any{"message": "Here."}, []string{"info_requested"}},
		{"accept-offer", map[string]any{}, []string{"offered"}},
		{"withdraw", map[string]any{}, []string{"draft", "submitted", "under_review", "info_requested", "offered", "offer_accepted"}},
	}

	refused := 0
	for status, ap := range apps {
		before, hist := h.statusOf(ap.id), h.historyCount(ap.id)
		for _, s := range staff {
			if slices.Contains(s.allowed, status) {
				continue
			}
			r := h.staffAct(w.registrar, ap.id, s.action, s.body)
			if status == "draft" {
				r.expect(t, http.StatusNotFound, "not_found")
			} else {
				r.expect(t, http.StatusConflict, "not_allowed")
			}
			refused++
		}
		for _, s := range applicant {
			if slices.Contains(s.allowed, status) || ap.a == nil {
				continue
			}
			r := h.applicantAct(ap.a.session, ap.id, s.action, s.body)
			if status == "enrolled" {
				r.expect(t, http.StatusUnauthorized, "") // now a student, signed out
			} else {
				r.expect(t, http.StatusConflict, "not_allowed")
			}
			refused++
		}
		if h.statusOf(ap.id) != before || h.historyCount(ap.id) != hist {
			t.Fatalf("%s: a refused action changed the application", status)
		}
		// The screen offers only what is allowed.
		if status != "draft" {
			d := h.staffApp(w.registrar, ap.id, http.StatusOK)
			for _, s := range staff {
				act := strings.ReplaceAll(s.action, "-", "_")
				if slices.Contains(d.Actions, act) != slices.Contains(s.allowed, status) {
					t.Errorf("%s: actions %v disagree on %s", status, d.Actions, act)
				}
			}
		}
	}
	if refused < 60 {
		t.Fatalf("only %d illegal actions tried", refused)
	}
	// Even the drafts' documents and offer letters stay out of reach.
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/staff/applications/" + draft + "/offer-letter/link", sess: w.registrar}), http.StatusNotFound, "not_found")
}

// AC-07 for the workflow: each action needs its permission.
func TestAdmissionsPermissions(t *testing.T) {
	h := newHarness(t)
	w := h.actors()
	_, id := h.submitted("applicant130@example.invalid")

	// Head of School can read but not act; a Content Editor cannot read.
	if d := h.staffApp(w.head, id, http.StatusOK); len(d.Actions) != 0 {
		t.Fatalf("head of school actions: %v", d.Actions)
	}
	h.staffApp(w.editor, id, http.StatusForbidden)
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/staff/applications", sess: w.editor}), http.StatusForbidden, "forbidden")
	for _, action := range []string{"assign", "notes", "request-info", "resume-review", "reject", "offer", "enrol"} {
		h.staffAct(w.head, id, action, map[string]any{"assignee_id": w.officerID, "body": "x", "message": "x", "reason": "x"}).
			expect(t, http.StatusForbidden, "forbidden")
	}
	// Applicants cannot use staff routes, even for their own application.
	a := h.newApplicant("applicant131@example.invalid", "")
	h.staffAct(a.session, id, "assign", map[string]any{"assignee_id": nil}).expect(t, http.StatusForbidden, "forbidden")
	// Another applicant cannot act on it.
	h.applicantAct(a.session, id, "withdraw", map[string]any{}).expect(t, http.StatusNotFound, "not_found")
	// Nobody can be given an application who cannot review.
	headID := h.createUser("head2@example.invalid", "head_of_school")
	h.staffAct(w.officer, id, "assign", map[string]any{"assignee_id": headID}).expect(t, http.StatusUnprocessableEntity, "not_assignable")
	if h.statusOf(id) != "submitted" {
		t.Fatal("status changed")
	}
}

// Two officers deciding at once: one decision wins, the other is refused.
func TestConcurrentDecisions(t *testing.T) {
	h := newHarness(t)
	w := h.actors()
	_, id := h.submitted("applicant140@example.invalid")
	h.staffAct(w.officer, id, "assign", map[string]any{"assignee_id": w.officerID}).expect(t, http.StatusOK, "")

	var wg sync.WaitGroup
	codes := make(chan int, 4)
	for i := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				codes <- h.staffAct(w.officer, id, "offer", nil).code
			} else {
				codes <- h.staffAct(w.officer2, id, "reject", map[string]any{"reason": "Not this time."}).code
			}
		}()
	}
	wg.Wait()
	close(codes)
	ok := 0
	for c := range codes {
		switch c {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
		default:
			t.Fatalf("unexpected status %d", c)
		}
	}
	var decisions, offers int
	if err := h.pool.QueryRow(context.Background(), `SELECT
		(SELECT count(*) FROM application_status_history WHERE application_id = $1 AND to_status IN ('offered', 'rejected')),
		(SELECT count(*) FROM offers WHERE application_id = $1)`, id).Scan(&decisions, &offers); err != nil {
		t.Fatal(err)
	}
	if ok != 1 || decisions != 1 || offers > 1 {
		t.Fatalf("%d succeeded, %d decisions, %d offers", ok, decisions, offers)
	}
	// A losing offer leaves no letter behind.
	if n := h.privateObjects(); n != 3+offers {
		t.Fatalf("private objects = %d", n)
	}
}

// Offers lapse after their respond-by date; the applicant is told why.
func TestOfferExpiry(t *testing.T) {
	h := newHarness(t)
	w := h.actors()
	a, id := h.submitted("applicant150@example.invalid")
	h.staffAct(w.officer, id, "assign", map[string]any{"assignee_id": w.officerID}).expect(t, http.StatusOK, "")
	tomorrow := time.Now().In(time.FixedZone("EAT", 3*3600)).AddDate(0, 0, 1).Format(time.DateOnly)
	h.staffAct(w.officer, id, "offer", map[string]any{"respond_by": tomorrow}).expect(t, http.StatusOK, "")

	// The respond-by day itself is still in time; the day after is not.
	h.now = func() time.Time { return time.Now().Add(24 * time.Hour) }
	if d := h.app(req{method: http.MethodGet, path: "/api/v1/applicant/applications/" + id, sess: a.session}, http.StatusOK); d.Status != "offered" {
		t.Fatalf("status on the respond-by day: %s", d.Status)
	}
	h.now = func() time.Time { return time.Now().Add(48 * time.Hour) }
	h.applicantAct(a.session, id, "accept-offer", map[string]any{}).expect(t, http.StatusConflict, "offer_expired")
	d := h.staffApp(w.officer, id, http.StatusOK)
	if d.Status != "expired" || d.History[len(d.History)-1].Reason != "Offer not accepted by its respond-by date" {
		t.Fatalf("after expiry: %s %+v", d.Status, d.History[len(d.History)-1])
	}
	h.applicantAct(a.session, id, "accept-offer", map[string]any{}).expect(t, http.StatusConflict, "not_allowed")
	h.now = time.Now
}

// Rejection and withdrawal: reasons are required where the brief says so
// and shown to the applicant.
func TestRejectAndWithdraw(t *testing.T) {
	h := newHarness(t)
	w := h.actors()
	const email = "applicant160@example.invalid"
	a, id := h.submitted(email)
	h.staffAct(w.officer, id, "assign", map[string]any{"assignee_id": w.officerID}).expect(t, http.StatusOK, "")
	h.staffAct(w.officer, id, "request-info", map[string]any{"message": "Send your ID."}).expect(t, http.StatusOK, "")
	h.staffAct(w.officer, id, "reject", map[string]any{"reason": "  "}).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	h.staffAct(w.officer, id, "reject", map[string]any{"reason": "The KCSE mean grade is below the entry requirement."}).expect(t, http.StatusOK, "")
	if text := h.waitMail(email, "Your application DTC-APP-"); !strings.Contains(text, "below the entry requirement") {
		t.Fatalf("rejection email: %s", text)
	}
	d := h.staffApp(w.officer, id, http.StatusOK)
	if d.InfoRequests[0].ClosedAt == nil || d.InfoRequests[0].ClosedByName == nil {
		t.Fatalf("open request not closed: %+v", d.InfoRequests)
	}
	own := h.do(req{method: http.MethodGet, path: "/api/v1/applicant/applications/" + id, sess: a.session})
	if !strings.Contains(own.Body.String(), "below the entry requirement") || !strings.Contains(own.Body.String(), `"can_withdraw":false`) {
		t.Fatalf("applicant view: %s", own.Body.String())
	}

	// Declining an offer is a withdrawal, with the reason kept.
	b, id2 := h.submitted("applicant161@example.invalid")
	h.staffAct(w.officer, id2, "assign", map[string]any{"assignee_id": w.officerID}).expect(t, http.StatusOK, "")
	h.staffAct(w.officer, id2, "offer", nil).expect(t, http.StatusOK, "")
	h.applicantAct(b.session, id2, "withdraw", map[string]any{"reason": "Moving to Mombasa."}).expect(t, http.StatusOK, "")
	d = h.staffApp(w.officer, id2, http.StatusOK)
	if d.Status != "withdrawn" || d.Offer.DeclinedAt == nil || d.History[len(d.History)-1].Reason != "Applicant declined the offer: Moving to Mombasa." {
		t.Fatalf("decline: %+v", d.History[len(d.History)-1])
	}
	// A withdrawn application no longer blocks a new one for the same intake.
	var programme string
	if err := h.pool.QueryRow(context.Background(), `SELECT p.slug FROM applications a JOIN programmes p ON p.id = a.programme_id WHERE a.id = $1`, id2).Scan(&programme); err != nil {
		t.Fatal(err)
	}
	h.start(b.session, programme, nil).expect(t, http.StatusCreated, "")
}

// Student numbers follow the setting, restart each year only when the
// year is part of them, and are unique.
func TestFormatStudentNumber(t *testing.T) {
	for _, c := range []struct {
		format string
		year   int
		n      int64
		want   string
	}{
		{admissions.DefaultStudentNumberFormat, 2027, 1, "DTC/2027/0001"},
		{"DTC-{YY}-{NNNN}", 2027, 12345, "DTC-27-12345"},
		{"S{NNNN}", 2027, 7, "S0007"},
	} {
		if got := admissions.FormatStudentNumber(c.format, c.year, c.n); got != c.want {
			t.Errorf("%s: %s, want %s", c.format, got, c.want)
		}
	}
}

// R-16: enrolments at the same moment get distinct, consecutive numbers in
// the intake's year, and enrolling one application twice at once makes one
// student.
func TestStudentNumbers(t *testing.T) {
	h := newHarness(t)
	w := h.actors()
	var ids []string
	for i := range 3 {
		a, id := h.submitted(fmt.Sprintf("applicant17%d@example.invalid", i))
		h.staffAct(w.officer, id, "assign", map[string]any{"assignee_id": w.officerID}).expect(t, http.StatusOK, "")
		h.staffAct(w.officer, id, "offer", nil).expect(t, http.StatusOK, "")
		h.applicantAct(a.session, id, "accept-offer", map[string]any{}).expect(t, http.StatusOK, "")
		ids = append(ids, id)
	}
	var wg sync.WaitGroup
	codes := make(chan int, 4)
	for _, id := range append(ids, ids[0]) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- h.staffAct(w.registrar, id, "enrol", nil).code
		}()
	}
	wg.Wait()
	close(codes)
	var got []int
	for c := range codes {
		got = append(got, c)
	}
	slices.Sort(got)
	if !slices.Equal(got, []int{http.StatusOK, http.StatusOK, http.StatusOK, http.StatusConflict}) {
		t.Fatalf("status codes %v", got)
	}
	var numbers []string
	for _, id := range ids {
		numbers = append(numbers, h.staffApp(w.registrar, id, http.StatusOK).Student.StudentNumber)
	}
	slices.Sort(numbers)
	var year, students int
	if err := h.pool.QueryRow(context.Background(), `SELECT extract(year FROM i.starts_on)::int, count(*) OVER () FROM students s
		JOIN programme_intakes i ON i.id = s.intake_id LIMIT 1`).Scan(&year, &students); err != nil {
		t.Fatal(err)
	}
	for i, n := range numbers {
		if want := fmt.Sprintf("DTC/%d/%04d", year, i+1); n != want || students != 3 {
			t.Fatalf("numbers %v (%d students), want DTC/%d/0001 to 0003", numbers, students, year)
		}
	}
}
