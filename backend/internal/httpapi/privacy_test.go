package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MboyaDan/dakota-college/backend/internal/privacy"
)

// scalar reads one value with a query.
func (h *harness) scalar(dst any, sql string, args ...any) {
	h.t.Helper()
	if err := h.pool.QueryRow(context.Background(), sql, args...).Scan(dst); err != nil {
		h.t.Fatalf("%s: %v", sql, err)
	}
}

func ruleResult(t *testing.T, res *privacy.RunResult, rule string) privacy.RuleResult {
	t.Helper()
	for _, r := range res.Results {
		if r.Rule == rule {
			return r
		}
	}
	t.Fatalf("no result for %s in %+v", rule, res.Results)
	return privacy.RuleResult{}
}

// rejectedApplication is a submitted application with documents, an
// information request, a staff note and a rejection with a reason.
func (h *harness) rejectedApplication(w workflowActors, email string) (applicantID, appID string) {
	h.t.Helper()
	_, id := h.submitted(email)
	h.staffAct(w.officer, id, "assign", map[string]any{"assignee_id": w.officerID}).expect(h.t, http.StatusOK, "")
	h.staffAct(w.officer, id, "request-info", map[string]any{"message": "Send a clearer copy of your ID."}).expect(h.t, http.StatusOK, "")
	h.staffAct(w.officer, id, "notes", map[string]any{"body": "Phoned the applicant about the ID copy."}).expect(h.t, http.StatusOK, "")
	h.staffAct(w.officer, id, "reject", map[string]any{"reason": "The KCSE mean grade is below the entry requirement."}).expect(h.t, http.StatusOK, "")
	h.scalar(&applicantID, `SELECT applicant_id::text FROM applications WHERE id = $1`, id)
	return applicantID, id
}

// Brief 13: the retention schedule erases applicants who did not enrol and
// old enquiries once their period ends, counts first in report mode, and
// keeps references and counts.
func TestRetentionSchedule(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	w := h.actors()
	const email = "applicant170@example.invalid"
	applicantID, appID := h.rejectedApplication(w, email)
	if h.privateObjects() != 3 {
		t.Fatalf("private objects = %d", h.privateObjects())
	}
	// A draft that will expire, and a submitted application still in review:
	// the second applicant is not erased.
	draft := h.newApplicant("applicant171@example.invalid", "Test Applicant 171")
	slug, _, _ := h.openProgramme(1)
	draftID := idOf(t, h.start(draft.session, slug, nil))
	h.submitted("applicant172@example.invalid")
	h.exec(`INSERT INTO enquiries (full_name, email, query_type, message, status, closed_at) VALUES
		('Test Enquirer 001', 'enquirer001@example.invalid', 'fees', 'How much?', 'closed', now()),
		('Test Enquirer 002', 'enquirer002@example.invalid', 'fees', 'Still open', 'open', NULL)`)
	_, studentProgramme, studentIntake := h.school("Retention")
	h.student(studentProgramme, studentIntake, nil)
	students := h.count(`SELECT count(*) FROM applicants a JOIN students s ON s.applicant_id = a.id WHERE a.erased_at IS NULL`)

	// Nothing personal is due today.
	res, err := h.privacy.Run(ctx, privacy.ModeApply)
	if err != nil {
		t.Fatal(err)
	}
	if r := ruleResult(t, res, "applicants"); r.Due != 0 {
		t.Fatalf("applicants due today: %+v", r)
	}

	// Sixteen months on (the test intakes start within three months, and
	// the period counts from the intake start when that is later), a report
	// counts without erasing.
	h.now = func() time.Time { return time.Now().AddDate(0, 16, 0) }
	res, err = h.privacy.Run(ctx, privacy.ModeReport)
	if err != nil {
		t.Fatal(err)
	}
	// The rejected applicant, and the draft's applicant: the draft expired
	// (housekeeping) in this run, at the real time, which the moved clock
	// already sees as more than 12 months ago.
	if r := ruleResult(t, res, "applicants"); r.Due != 2 || r.Done != 0 {
		t.Fatalf("report applicants: %+v", r)
	}
	if r := ruleResult(t, res, "enquiries"); r.Due != 1 || r.Done != 0 {
		t.Fatalf("report enquiries: %+v", r)
	}
	if r := ruleResult(t, res, "expired_drafts"); r.Done != 1 {
		t.Fatalf("drafts: %+v", r)
	}
	if r := ruleResult(t, res, "sign_in_links"); r.Done == 0 {
		t.Fatalf("spent sign-in links not removed in report mode: %+v", r)
	}
	if h.statusOf(draftID) != "expired" {
		t.Fatal("overdue draft not expired")
	}
	if h.count(`SELECT count(*) FROM applicants WHERE erased_at IS NOT NULL`) != 0 || h.enquiryCount() != 2 {
		t.Fatal("report mode erased something")
	}
	if h.auditCount("privacy.retention_applied") != 1 {
		t.Fatal("report mode was audited as applied")
	}

	// Apply erases exactly what the report counted.
	res, err = h.privacy.Run(ctx, privacy.ModeApply)
	if err != nil {
		t.Fatal(err)
	}
	if r := ruleResult(t, res, "applicants"); r.Due != 2 || r.Done != 2 {
		t.Fatalf("apply applicants: %+v", r)
	}
	if r := ruleResult(t, res, privacy.FilesRule); r.Due != 3 || r.Done != 3 {
		t.Fatalf("files: %+v", r)
	}
	// The applicant still in review keeps their three documents.
	if h.privateObjects() != 3 || strings.Contains(strings.Join(h.storage.Keys(), " "), appID) {
		t.Fatalf("storage after erasure: %v", h.storage.Keys())
	}

	var a struct {
		Email, First, Last, Identity, County string
		Phone                                *string
		Dob                                  *time.Time
	}
	if err := h.pool.QueryRow(ctx, `SELECT email, first_name, last_name, identity_document, county, phone, date_of_birth FROM applicants WHERE id = $1`, applicantID).
		Scan(&a.Email, &a.First, &a.Last, &a.Identity, &a.County, &a.Phone, &a.Dob); err != nil {
		t.Fatal(err)
	}
	if a.Email != "erased-"+applicantID+"@erased.invalid" || a.First != "Erased" || a.Identity != "" || a.County != "" || a.Phone != nil || a.Dob != nil {
		t.Fatalf("applicant not erased: %+v", a)
	}
	for _, c := range []struct{ name, sql string }{
		{"education", `SELECT count(*) FROM application_education WHERE application_id = $1`},
		{"documents", `SELECT count(*) FROM application_documents WHERE application_id = $1`},
		{"notes", `SELECT count(*) FROM application_notes WHERE application_id = $1 AND body <> '[Erased]'`},
		{"reasons", `SELECT count(*) FROM application_status_history WHERE application_id = $1 AND reason NOT IN ('', '[Erased]')`},
		{"info requests", `SELECT count(*) FROM application_info_requests WHERE application_id = $1 AND message <> '[Erased]'`},
		{"answers", `SELECT count(*) FROM applications WHERE id = $1 AND submitted_snapshot <> '{"erased": true}'::jsonb`},
		{"consents", `SELECT count(*) FROM consents c JOIN applications p ON p.applicant_id = c.subject_id WHERE p.id = $1 AND c.given_by_name <> '[Erased]'`},
		{"account", `SELECT count(*) FROM users u JOIN applicants a ON a.user_id = u.id JOIN applications p ON p.applicant_id = a.id WHERE p.id = $1 AND (u.status <> 'disabled' OR u.email NOT LIKE 'erased-%')`},
		{"sessions", `SELECT count(*) FROM sessions s JOIN applicants a ON a.user_id = s.user_id JOIN applications p ON p.applicant_id = a.id WHERE p.id = $1`},
	} {
		if n := h.count(c.sql, appID); n != 0 {
			t.Errorf("%s left: %d", c.name, n)
		}
	}
	if h.count(`SELECT count(*) FROM notifications WHERE recipient = $1`, email) != 0 {
		t.Error("messages to the erased applicant left")
	}
	// The application still counts, with its reference and outcome.
	if h.statusOf(appID) != "rejected" || h.count(`SELECT count(*) FROM applications WHERE id = $1 AND reference LIKE 'DTC-APP-%'`, appID) != 1 {
		t.Fatal("application row lost")
	}
	if h.count(`SELECT count(*) FROM enquiries`) != 1 || h.count(`SELECT count(*) FROM enquiries WHERE status = 'open'`) != 1 {
		t.Fatal("wrong enquiries removed")
	}
	if h.count(`SELECT count(*) FROM applicants a JOIN students s ON s.applicant_id = a.id WHERE a.erased_at IS NULL`) != students {
		t.Fatal("a student's applicant record was erased")
	}
	if h.auditCount("privacy.applicant_erased") != 2 || h.auditCount("privacy.retention_applied") != 2 {
		t.Fatalf("audit: erased %d applied %d", h.auditCount("privacy.applicant_erased"), h.auditCount("privacy.retention_applied"))
	}

	// A second run finds nothing more.
	res, err = h.privacy.Run(ctx, privacy.ModeApply)
	if err != nil {
		t.Fatal(err)
	}
	if r := ruleResult(t, res, "applicants"); r.Due != 0 {
		t.Fatalf("second run: %+v", r)
	}
	if h.count(`SELECT count(*) FROM retention_runs WHERE finished_at IS NOT NULL AND error = ''`) != 4 {
		t.Fatal("runs not logged")
	}
}

// The append-only tables accept an erasure that blanks the listed text,
// and nothing else.
func TestAppendOnlyExceptErasure(t *testing.T) {
	h := newHarness(t)
	w := h.actors()
	_, appID := h.rejectedApplication(w, "applicant175@example.invalid")
	ctx := context.Background()
	try := func(allow bool, sql string) error {
		tx, err := h.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if allow {
			if _, err := tx.Exec(ctx, `SELECT set_config('dtc.erasure', 'on', true)`); err != nil {
				t.Fatal(err)
			}
		}
		_, err = tx.Exec(ctx, sql, appID)
		return err
	}
	for _, c := range []struct {
		allow bool
		sql   string
		ok    bool
	}{
		{false, `UPDATE application_notes SET body = '[Erased]' WHERE application_id = $1`, false},
		{true, `UPDATE application_notes SET body = '[Erased]' WHERE application_id = $1`, true},
		{true, `UPDATE application_notes SET body = '[Erased]', created_at = now() - interval '1 day' WHERE application_id = $1`, false},
		{true, `DELETE FROM application_notes WHERE application_id = $1`, false},
		{true, `UPDATE application_status_history SET to_status = 'withdrawn' WHERE application_id = $1 AND to_status = 'rejected'`, false},
		{true, `UPDATE application_status_history SET reason = '[Erased]' WHERE application_id = $1 AND reason <> ''`, true},
		{true, `UPDATE audit_logs SET details = '{}' WHERE entity_id = $1::text`, false},
	} {
		err := try(c.allow, c.sql)
		if (err == nil) != c.ok {
			t.Errorf("%s (erasure %v): err = %v", c.sql, c.allow, err)
		}
	}
}

type dataRequestView struct {
	ID        string `json:"id"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
	DueOn     string `json:"due_on"`
	Received  string `json:"received_on"`
	Matches   struct {
		Applicants []struct {
			ID       string `json:"id"`
			Erasable bool   `json:"erasable"`
		} `json:"applicants"`
		Enquiries []struct {
			ID string `json:"id"`
		} `json:"enquiries"`
		Students []struct {
			ID string `json:"id"`
		} `json:"students"`
	} `json:"matches"`
	Events []struct {
		Action string `json:"action"`
	} `json:"events"`
}

// Brief 13: requests are recorded with a due date, identity is checked
// before anything is released or erased, exports hold the person's data
// without secrets, and erasure refuses students and live applications.
func TestDataSubjectRequests(t *testing.T) {
	h := newHarness(t)
	w := h.actors()
	reg := w.registrar
	const email = "applicant180@example.invalid"
	_, appID := h.submitted(email)
	h.exec(`INSERT INTO enquiries (full_name, email, query_type, message) VALUES ('Test Applicant 180', $1, 'fees', 'Fees please')`, email)

	// Only privacy.manage may see the register.
	h.api(w.officer, http.MethodGet, "/staff/privacy/requests", nil, http.StatusForbidden, nil)
	h.api(reg, http.MethodGet, "/staff/privacy/requests", nil, http.StatusOK, nil)
	h.api(reg, http.MethodGet, "/staff/privacy/retention", nil, http.StatusOK, nil)

	h.api(reg, http.MethodPost, "/staff/privacy/requests", map[string]any{
		"kind": "access", "requester_name": "Test Applicant 180", "requester_contact": email,
	}, http.StatusUnprocessableEntity, nil)
	var access dataRequestView
	h.api(reg, http.MethodPost, "/staff/privacy/requests", map[string]any{
		"kind": "access", "requester_name": "Test Applicant 180", "requester_contact": email, "subject_email": strings.ToUpper(email),
	}, http.StatusCreated, &access)
	if !strings.HasPrefix(access.Reference, "DTC-DSR-") || !strings.HasSuffix(access.Reference, "-0001") {
		t.Fatalf("reference %q", access.Reference)
	}
	received, _ := time.Parse(time.DateOnly, access.Received)
	if access.DueOn != received.AddDate(0, 0, privacy.DueDays).Format(time.DateOnly) {
		t.Fatalf("due %s for received %s", access.DueOn, access.Received)
	}
	if len(access.Matches.Applicants) != 1 || len(access.Matches.Enquiries) != 1 || access.Matches.Applicants[0].Erasable {
		t.Fatalf("matches: %+v", access.Matches)
	}
	base := "/staff/privacy/requests/" + access.ID

	// Nothing leaves before identity is checked.
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1" + base + "/export", sess: reg}), http.StatusConflict, "identity_not_checked")
	h.api(reg, http.MethodPost, base+"/identity", map[string]any{"method": "  "}, http.StatusUnprocessableEntity, nil)
	h.api(reg, http.MethodPost, base+"/identity", map[string]any{"method": "Saw national ID at the front office"}, http.StatusOK, nil)
	exp := h.do(req{method: http.MethodPost, path: "/api/v1" + base + "/export", sess: reg})
	if exp.Code != http.StatusOK || !strings.Contains(exp.Header().Get("Content-Disposition"), "dtc-dsr-") {
		t.Fatalf("export: %d %v", exp.Code, exp.Header())
	}
	body := exp.Body.String()
	var ref string
	h.scalar(&ref, `SELECT reference FROM applications WHERE id = $1`, appID)
	for _, want := range []string{email, ref, `"account"`, `"enquiries"`, "Fees please", `"consents"`, "Test School"} {
		if !strings.Contains(body, want) {
			t.Errorf("export lacks %q", want)
		}
	}
	for _, never := range []string{"password_hash", "token_hash", "payload", "argon2"} {
		if strings.Contains(body, never) {
			t.Errorf("export contains %q", never)
		}
	}
	if !json.Valid(exp.Body.Bytes()) {
		t.Fatal("export is not JSON")
	}
	if h.auditCount("privacy.request_exported") != 1 {
		t.Fatal("export not audited")
	}
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1" + base + "/erase", sess: reg,
		body: map[string]any{"record_type": "applicant", "record_id": access.Matches.Applicants[0].ID}}), http.StatusConflict, "wrong_request_kind")
	h.api(reg, http.MethodPost, base+"/close", map[string]any{"status": "completed", "outcome": " "}, http.StatusUnprocessableEntity, nil)
	h.api(reg, http.MethodPost, base+"/close", map[string]any{"status": "completed", "outcome": "Sent the file by email on the due date."}, http.StatusOK, nil)
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1" + base + "/notes", sess: reg, body: map[string]any{"note": "late"}}), http.StatusConflict, "request_closed")

	// Erasure: a live application blocks it until it is decided.
	var erasure dataRequestView
	h.api(reg, http.MethodPost, "/staff/privacy/requests", map[string]any{
		"kind": "erasure", "requester_name": "Test Applicant 180", "requester_contact": "+254 700 000 020", "subject_phone": "0700000020",
	}, http.StatusCreated, &erasure)
	if len(erasure.Matches.Applicants) != 1 {
		t.Fatalf("phone did not find the applicant: %+v", erasure.Matches)
	}
	eb := "/staff/privacy/requests/" + erasure.ID
	target := map[string]any{"record_type": "applicant", "record_id": erasure.Matches.Applicants[0].ID}
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1" + eb + "/erase", sess: reg, body: target}), http.StatusConflict, "identity_not_checked")
	h.api(reg, http.MethodPost, eb+"/identity", map[string]any{"method": "Called back on the number we hold"}, http.StatusOK, nil)
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1" + eb + "/erase", sess: reg, body: target}), http.StatusConflict, "application_in_progress")
	// Someone else's record is refused.
	// Test applicants share one phone number; give this one another.
	h.submitted("applicant181@example.invalid")
	h.exec(`UPDATE applicants SET phone = '+254 700 000 181' WHERE email = 'applicant181@example.invalid'`)
	var otherID string
	h.scalar(&otherID, `SELECT id::text FROM applicants WHERE email = 'applicant181@example.invalid'`)
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1" + eb + "/erase", sess: reg,
		body: map[string]any{"record_type": "applicant", "record_id": otherID}}), http.StatusConflict, "not_this_person")

	h.staffAct(w.officer, appID, "assign", map[string]any{"assignee_id": w.officerID}).expect(t, http.StatusOK, "")
	h.staffAct(w.officer, appID, "reject", map[string]any{"reason": "Withdrawn at the applicant's request."}).expect(t, http.StatusOK, "")
	var after dataRequestView
	h.api(reg, http.MethodPost, eb+"/erase", target, http.StatusOK, &after)
	if h.count(`SELECT count(*) FROM applicants WHERE id = $1 AND erased_at IS NOT NULL`, erasure.Matches.Applicants[0].ID) != 1 {
		t.Fatal("applicant not erased")
	}
	if len(after.Matches.Applicants) != 0 || after.Events[len(after.Events)-1].Action != "erased" {
		t.Fatalf("after erase: %+v", after)
	}
	// The enquiry carried the email, not the phone, so this request does
	// not reach it; the earlier access request's email did.
	if len(after.Matches.Enquiries) != 0 {
		t.Fatalf("enquiry matched by phone: %+v", after.Matches.Enquiries)
	}

	// Students are kept.
	_, programme, intake := h.school("Privacy")
	sid := h.student(programme, intake, nil)
	var studentEmail string
	h.scalar(&studentEmail, `SELECT email FROM students WHERE id = $1`, sid)
	var st dataRequestView
	h.api(reg, http.MethodPost, "/staff/privacy/requests", map[string]any{
		"kind": "erasure", "requester_name": "Test Student", "requester_contact": studentEmail, "subject_email": studentEmail,
	}, http.StatusCreated, &st)
	h.api(reg, http.MethodPost, "/staff/privacy/requests/"+st.ID+"/identity", map[string]any{"method": "Student card"}, http.StatusOK, nil)
	if len(st.Matches.Students) != 1 || len(st.Matches.Applicants) != 1 || st.Matches.Applicants[0].Erasable {
		t.Fatalf("student matches: %+v", st.Matches)
	}
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/staff/privacy/requests/" + st.ID + "/erase", sess: reg,
		body: map[string]any{"record_type": "applicant", "record_id": st.Matches.Applicants[0].ID}}), http.StatusConflict, "is_student")

	var list struct {
		Items []struct{ Reference, Status string }
		Total int
	}
	h.api(reg, http.MethodGet, "/staff/privacy/requests?status=open", nil, http.StatusOK, &list)
	if list.Total != 2 {
		t.Fatalf("open requests: %+v", list)
	}
	h.api(reg, http.MethodGet, "/staff/privacy/requests?status=nonsense", nil, http.StatusUnprocessableEntity, nil)
}
