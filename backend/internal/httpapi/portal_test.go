package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/auth"
)

const studentPassword = "a student password 1"

// studentLogin gives a student a password and signs them in by number.
func (h *harness) studentLogin(id uuid.UUID) *session {
	h.t.Helper()
	hash, err := auth.HashPassword(studentPassword)
	if err != nil {
		h.t.Fatal(err)
	}
	var number string
	if err := h.pool.QueryRow(context.Background(), `UPDATE users u SET password_hash = $2 FROM students s
		WHERE s.id = $1 AND u.id = s.user_id RETURNING s.student_number`, id, hash).Scan(&number); err != nil {
		h.t.Fatal(err)
	}
	h.exec(`INSERT INTO user_roles (user_id, role_id) SELECT s.user_id, r.id FROM students s, roles r WHERE s.id = $1 AND r.key = 'student' ON CONFLICT DO NOTHING`, id)
	w := h.do(req{method: http.MethodPost, path: "/api/v1/student/sign-in", body: map[string]string{"student_number": strings.ToLower(number), "password": studentPassword}})
	expectStatus(h.t, w, http.StatusOK, "")
	var body struct {
		CSRFToken string `json:"csrf_token"`
		User      struct{ Kind string }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.User.Kind != "student" {
		h.t.Fatalf("sign-in: %s", w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "__Host-dtc_session" {
			return &session{cookie: c, csrf: body.CSRFToken}
		}
	}
	h.t.Fatal("no session cookie")
	return nil
}

var setupLinkPattern = regexp.MustCompile(`/reset-password\?token=([A-Za-z0-9_-]+)&portal=student`)

// D-141: enrolment emails a link to choose a password; the student then
// signs in with their number. Unknown numbers look like wrong passwords.
func TestStudentSignIn(t *testing.T) {
	h := newHarness(t)
	w := h.actors()
	const email = "applicant200@example.invalid"
	a, id := h.submitted(email)
	h.staffAct(w.officer, id, "assign", map[string]any{"assignee_id": w.officerID}).expect(t, http.StatusOK, "")
	h.staffAct(w.officer, id, "offer", nil).expect(t, http.StatusOK, "")
	h.applicantAct(a.session, id, "accept-offer", map[string]any{}).expect(t, http.StatusOK, "")
	h.staffAct(w.registrar, id, "enrol", nil).expect(t, http.StatusOK, "")
	number := h.staffApp(w.registrar, id, http.StatusOK).Student.StudentNumber

	text := h.waitMail(email, "Welcome")
	m := setupLinkPattern.FindStringSubmatch(text)
	if m == nil || !strings.Contains(text, number) {
		t.Fatalf("enrolment email has no set-up link: %s", text)
	}
	// Before choosing a password, sign-in fails like any wrong password.
	signIn := func(n, p string) *httpResponse {
		r := h.do(req{method: http.MethodPost, path: "/api/v1/student/sign-in", body: map[string]string{"student_number": n, "password": p}})
		return &httpResponse{r.Code, r.Body.String()}
	}
	signIn(number, studentPassword).expect(t, http.StatusUnauthorized, "invalid_credentials")
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/password-reset/confirm",
		body: map[string]string{"token": m[1], "password": studentPassword}}), http.StatusNoContent, "")
	// The link worked once.
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/auth/password-reset/confirm",
		body: map[string]string{"token": m[1], "password": studentPassword}}), http.StatusBadRequest, "")

	signIn("  "+strings.ToLower(number)+" ", studentPassword).expect(t, http.StatusOK, "")
	signIn(number, "not the password").expect(t, http.StatusUnauthorized, "invalid_credentials")
	signIn("DTC/1999/9999", studentPassword).expect(t, http.StatusUnauthorized, "invalid_credentials")

	// Forgot password by number: a link to the address on record; unknown
	// numbers get the same answer and no email.
	before := h.mailsTo(email)
	for _, n := range []string{number, "DTC/1999/9999"} {
		r := h.do(req{method: http.MethodPost, path: "/api/v1/student/password-reset", body: map[string]string{"student_number": n}})
		expectStatus(t, r, http.StatusAccepted, "")
	}
	deadline := time.Now().Add(3 * time.Second)
	for h.mailsTo(email) == before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if h.mailsTo(email) != before+1 {
		t.Fatalf("reset emails = %d, want 1", h.mailsTo(email)-before)
	}
	for _, msg := range h.mailer.All() {
		if msg.To == email && strings.Contains(msg.Subject, "Reset") && !strings.Contains(msg.Text, "&portal=student") {
			t.Fatalf("student reset link does not lead back to the portal: %s", msg.Text)
		}
	}
}

// AC-08: a student sees only their own data.
func TestStudentsSeeOnlyTheirOwnData(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	other := h.cohort(a.programme, nil)
	h.exec(`UPDATE students SET cohort_id = $2 WHERE id = $1`, a.students[1], other)
	alice, bob := h.studentLogin(a.students[0]), h.studentLogin(a.students[1])

	var prof struct {
		StudentNumber string `json:"student_number"`
		Guardians     []struct{ ID uuid.UUID }
	}
	h.api(alice, http.MethodGet, "/student/profile", nil, http.StatusOK, &prof)
	var number string
	if err := h.pool.QueryRow(context.Background(), `SELECT student_number FROM students WHERE id = $1`, a.students[0]).Scan(&number); err != nil || prof.StudentNumber != number {
		t.Fatalf("alice sees %q, want %q", prof.StudentNumber, number)
	}

	// Bob's contact cannot be touched through Alice's account.
	h.api(bob, http.MethodPost, "/student/guardians", map[string]any{"full_name": "Test Parent B", "relationship": "father", "phone": "+254 700 000 301", "email": "", "is_primary": true}, http.StatusOK, &prof)
	bobGuardian := prof.Guardians[0].ID
	for _, c := range []struct{ method, path string }{{http.MethodPut, "/student/guardians/"}, {http.MethodDelete, "/student/guardians/"}} {
		if got := h.code(alice, c.method, c.path+bobGuardian.String(), map[string]any{"full_name": "X", "relationship": "father", "phone": "+254 700 000 302", "email": "", "is_primary": false}); got != "404 not_found" {
			t.Errorf("alice %s bob's guardian: %s", c.method, got)
		}
	}

	// Notices: Alice's cohort, not Bob's.
	h.api(a.registrar, http.MethodPost, "/staff/notices", map[string]any{"audience": "cohort", "cohort_id": a.cohort, "title": "For cohort A", "body": "Hello A."}, http.StatusOK, nil)
	h.api(a.registrar, http.MethodPost, "/staff/notices", map[string]any{"audience": "cohort", "cohort_id": other, "title": "For cohort B", "body": "Hello B."}, http.StatusOK, nil)
	var notices struct{ Items []struct{ Title string } }
	h.api(alice, http.MethodGet, "/student/notices", nil, http.StatusOK, &notices)
	if len(notices.Items) != 1 || notices.Items[0].Title != "For cohort A" {
		t.Fatalf("alice's notices: %+v", notices.Items)
	}

	// Requests: each sees their own.
	h.api(bob, http.MethodPost, "/student/requests", map[string]any{"kind": "letter", "details": "A letter for my bank."}, http.StatusOK, nil)
	var reqs struct{ Items []struct{ Details string } }
	h.api(alice, http.MethodGet, "/student/requests", nil, http.StatusOK, &reqs)
	if len(reqs.Items) != 0 {
		t.Fatalf("alice sees bob's requests: %+v", reqs.Items)
	}

	// Students cannot use staff or applicant routes; staff cannot use student routes.
	for _, p := range []string{"/staff/students", "/staff/students/" + a.students[1].String(), "/staff/teaching", "/staff/cohorts", "/staff/applications", "/applicant/applications"} {
		if got := h.code(alice, http.MethodGet, p, nil); got != "403 forbidden" {
			t.Errorf("student GET %s: %s", p, got)
		}
	}
	if got := h.code(a.registrar, http.MethodGet, "/student/profile", nil); got != "403 forbidden" {
		t.Fatalf("registrar on a student route: %s", got)
	}
}

// Registration rules: active, inside the window, units of the programme.
func TestStudentRegistration(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	alice := h.studentLogin(a.students[0])
	// The academy registered the cohort; start Alice clean on unit 2.
	var regID uuid.UUID
	if err := h.pool.QueryRow(context.Background(), `SELECT id FROM unit_registrations WHERE offering_id = $1 AND student_id = $2`, a.offering2, a.students[0]).Scan(&regID); err != nil {
		t.Fatal(err)
	}
	h.api(a.registrar, http.MethodPost, "/staff/offerings/"+a.offering2.String()+"/registrations/"+regID.String()+"/drop", map[string]any{}, http.StatusOK, nil)

	type view struct {
		CanRegister bool `json:"can_register"`
		Open        []struct {
			OfferingID uuid.UUID `json:"offering_id"`
			Registered bool
		}
	}
	var v view
	// No window set yet: nothing open.
	h.api(alice, http.MethodGet, "/student/registration", nil, http.StatusOK, &v)
	if !v.CanRegister || len(v.Open) != 0 {
		t.Fatalf("before the window: %+v", v)
	}
	if got := h.code(alice, http.MethodPost, "/student/registration/"+a.offering2.String(), map[string]any{}); got != "409 registration_closed" {
		t.Fatalf("outside the window: %s", got)
	}

	h.exec(`UPDATE academic_terms SET registration_opens_on = current_date - 1, registration_closes_on = current_date + 7 WHERE id = $1`, a.term)
	h.api(alice, http.MethodGet, "/student/registration", nil, http.StatusOK, &v)
	if len(v.Open) != 2 {
		t.Fatalf("open: %+v", v.Open)
	}
	h.api(alice, http.MethodPost, "/student/registration/"+a.offering2.String(), map[string]any{}, http.StatusOK, &v)
	if got := h.code(alice, http.MethodPost, "/student/registration/"+a.offering2.String(), map[string]any{}); got != "409 unchanged" {
		t.Fatalf("register twice: %s", got)
	}
	h.api(alice, http.MethodPost, "/student/registration/"+a.offering2.String()+"/drop", map[string]any{}, http.StatusOK, &v)

	// A unit outside the programme is never open to them.
	var c newID
	h.api(a.registrar, http.MethodPost, "/staff/units", map[string]any{"school_id": a.school, "code": "TST 999", "title": "Not in the programme"}, http.StatusCreated, &c)
	var off newID
	h.api(a.registrar, http.MethodPost, "/staff/offerings", map[string]any{"term_id": a.term, "unit_id": c.ID}, http.StatusCreated, &off)
	if got := h.code(alice, http.MethodPost, "/student/registration/"+off.ID.String(), map[string]any{}); got != "409 registration_closed" {
		t.Fatalf("unit outside the programme: %s", got)
	}

	// Only active students.
	h.exec(`UPDATE students SET status = 'suspended' WHERE id = $1`, a.students[0])
	if got := h.code(alice, http.MethodPost, "/student/registration/"+a.offering2.String(), map[string]any{}); got != "409 not_active" {
		t.Fatalf("suspended: %s", got)
	}
	if n := h.auditCount("student.unit_registered"); n != 1 {
		t.Fatalf("self-registrations audited %d times", n)
	}
}

// D-142: phone at once; identity details through the registrar.
func TestProfileChanges(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	alice := h.studentLogin(a.students[0])
	var prof struct {
		Phone          *string
		FirstName      string `json:"first_name"`
		ChangeRequests []struct {
			ID     uuid.UUID
			Status string
		} `json:"change_requests"`
	}
	h.api(alice, http.MethodPut, "/student/profile/phone", map[string]any{"phone": "+254 700 000 401"}, http.StatusOK, &prof)
	if prof.Phone == nil || *prof.Phone != "+254 700 000 401" {
		t.Fatalf("phone: %+v", prof.Phone)
	}
	if got := h.code(alice, http.MethodPut, "/student/profile/phone", map[string]any{"phone": "call me"}); got != "422 validation_failed" {
		t.Fatalf("bad phone: %s", got)
	}

	if got := h.code(alice, http.MethodPost, "/student/profile/change-request", map[string]any{"reason": "Typo"}); got != "422 validation_failed" {
		t.Fatalf("nothing to change: %s", got)
	}
	if got := h.code(alice, http.MethodPost, "/student/profile/change-request", map[string]any{"first_name": "Testa", "date_of_birth": "2999-01-01", "reason": "Typo"}); got != "422 validation_failed" {
		t.Fatalf("future birth date: %s", got)
	}
	h.api(alice, http.MethodPost, "/student/profile/change-request", map[string]any{"first_name": "Testa", "reason": "My name was misspelt."}, http.StatusOK, &prof)
	if prof.FirstName != "Test" || len(prof.ChangeRequests) != 1 || prof.ChangeRequests[0].Status != "pending" {
		t.Fatalf("after request: %+v", prof)
	}
	if got := h.code(alice, http.MethodPost, "/student/profile/change-request", map[string]any{"last_name": "Other", "reason": "Also"}); got != "409 change_pending" {
		t.Fatalf("second request: %s", got)
	}

	// A Head of another school cannot see or decide it.
	otherSchool, _, _ := h.school("Other")
	headID := h.createUser("head@example.invalid", "head_of_school")
	h.exec(`INSERT INTO school_assignments (user_id, school_id) VALUES ($1, $2)`, headID, otherSchool)
	head := h.mustLogin("head@example.invalid")
	var list struct{ Items []struct{ ID uuid.UUID } }
	h.api(head, http.MethodGet, "/staff/profile-requests?status=pending", nil, http.StatusOK, &list)
	if len(list.Items) != 0 {
		t.Fatalf("other school's head sees %d requests", len(list.Items))
	}
	rid := prof.ChangeRequests[0].ID.String()
	if got := h.code(head, http.MethodPost, "/staff/profile-requests/"+rid+"/approve", map[string]any{}); got != "403 forbidden" {
		t.Fatalf("head approving (no students.update): %s", got)
	}
	if got := h.code(a.registrar, http.MethodPost, "/staff/profile-requests/"+rid+"/decline", map[string]any{"decision": ""}); got != "422 validation_failed" {
		t.Fatalf("decline without reason: %s", got)
	}
	h.api(a.registrar, http.MethodPost, "/staff/profile-requests/"+rid+"/approve", map[string]any{"decision": "Checked against the ID card."}, http.StatusOK, nil)
	if got := h.code(a.registrar, http.MethodPost, "/staff/profile-requests/"+rid+"/approve", map[string]any{}); got != "409 unchanged" {
		t.Fatalf("approve twice: %s", got)
	}
	var first, account string
	if err := h.pool.QueryRow(context.Background(), `SELECT s.first_name, u.full_name FROM students s JOIN users u ON u.id = s.user_id WHERE s.id = $1`, a.students[0]).Scan(&first, &account); err != nil {
		t.Fatal(err)
	}
	if first != "Testa" || !strings.HasPrefix(account, "Testa ") {
		t.Fatalf("after approval: %q, account %q", first, account)
	}
	if h.auditCount("student.change_approved") != 1 || h.auditCount("student.phone_changed") != 1 {
		t.Fatal("changes not audited")
	}
}

// Notices by managers and trainers; service requests handled by staff.
func TestNoticesAndServiceRequests(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	alice := h.studentLogin(a.students[0])

	// The trainer posts to the class they teach, not to another.
	h.api(a.trainer1, http.MethodPost, "/staff/teaching/"+a.offering1.String()+"/notices", map[string]any{"title": "Bring laptops", "body": "Tomorrow we use laptops.", "ends_on": time.Now().AddDate(0, 0, 3).Format(time.DateOnly)}, http.StatusOK, nil)
	if got := h.code(a.trainer1, http.MethodPost, "/staff/teaching/"+a.offering2.String()+"/notices", map[string]any{"title": "x", "body": "y"}); got != "404 not_found" {
		t.Fatalf("trainer posting to another class: %s", got)
	}
	if got := h.code(a.trainer1, http.MethodPost, "/staff/notices", map[string]any{"audience": "all", "title": "x", "body": "y"}); got != "403 forbidden" {
		t.Fatalf("trainer posting to everyone: %s", got)
	}
	// A Head of School cannot post to the whole college.
	headID := h.createUser("head@example.invalid", "head_of_school")
	h.exec(`INSERT INTO school_assignments (user_id, school_id) VALUES ($1, $2)`, headID, a.school)
	head := h.mustLogin("head@example.invalid")
	if got := h.code(head, http.MethodPost, "/staff/notices", map[string]any{"audience": "all", "title": "x", "body": "y"}); got != "403 college_wide" {
		t.Fatalf("head posting to everyone: %s", got)
	}
	h.api(head, http.MethodPost, "/staff/notices", map[string]any{"audience": "school", "school_id": a.school, "title": "School assembly", "body": "Friday at 10."}, http.StatusOK, nil)
	var staffList struct {
		Items []struct {
			ID    uuid.UUID
			Title string
		}
	}
	h.api(a.registrar, http.MethodPost, "/staff/notices", map[string]any{"audience": "all", "title": "College closed Monday", "body": "Public holiday."}, http.StatusOK, &staffList)

	var notices struct{ Items []struct{ Title string } }
	h.api(alice, http.MethodGet, "/student/notices", nil, http.StatusOK, &notices)
	if len(notices.Items) != 3 {
		t.Fatalf("alice's notices: %+v", notices.Items)
	}
	// Withdrawn notices disappear; a trainer can withdraw only their own.
	var collegeNotice uuid.UUID
	for _, n := range staffList.Items {
		if n.Title == "College closed Monday" {
			collegeNotice = n.ID
		}
	}
	if got := h.code(a.trainer1, http.MethodPost, "/staff/teaching/"+a.offering1.String()+"/notices/"+collegeNotice.String()+"/withdraw", map[string]any{}); got != "404 not_found" {
		t.Fatalf("trainer withdrawing someone else's notice: %s", got)
	}
	h.api(a.registrar, http.MethodPost, "/staff/notices/"+collegeNotice.String()+"/withdraw", map[string]any{}, http.StatusOK, nil)
	h.api(alice, http.MethodGet, "/student/notices", nil, http.StatusOK, &notices)
	if len(notices.Items) != 2 {
		t.Fatalf("after withdrawal: %+v", notices.Items)
	}

	// Service requests.
	if got := h.code(alice, http.MethodPost, "/student/requests", map[string]any{"kind": "pizza", "details": "x"}); got != "422 validation_failed" {
		t.Fatalf("unknown kind: %s", got)
	}
	for i := range 5 {
		h.api(alice, http.MethodPost, "/student/requests", map[string]any{"kind": "letter", "details": "Letter " + string(rune('A'+i))}, http.StatusOK, nil)
	}
	if got := h.code(alice, http.MethodPost, "/student/requests", map[string]any{"kind": "letter", "details": "One more"}); got != "409 too_many_requests_open" {
		t.Fatalf("sixth open request: %s", got)
	}
	var list struct {
		Items []struct {
			ID     uuid.UUID
			Status string
		}
	}
	h.api(a.registrar, http.MethodGet, "/staff/service-requests?status=open", nil, http.StatusOK, &list)
	if len(list.Items) != 5 {
		t.Fatalf("open requests: %d", len(list.Items))
	}
	rid := list.Items[0].ID.String()
	if got := h.code(a.registrar, http.MethodPost, "/staff/service-requests/"+rid, map[string]any{"status": "declined", "response": ""}); got != "422 validation_failed" {
		t.Fatalf("decline without reason: %s", got)
	}
	h.api(a.registrar, http.MethodPost, "/staff/service-requests/"+rid, map[string]any{"status": "done", "response": "Collect it from the office."}, http.StatusOK, nil)
	if got := h.code(a.registrar, http.MethodPost, "/staff/service-requests/"+rid, map[string]any{"status": "in_progress"}); got != "422 validation_failed" {
		t.Fatalf("reopening a done request: %s", got)
	}
	var email string
	if err := h.pool.QueryRow(context.Background(), `SELECT email FROM students WHERE id = $1`, a.students[0]).Scan(&email); err != nil {
		t.Fatal(err)
	}
	if text := h.waitMail(email, "Your request"); !strings.Contains(text, "Collect it from the office.") || !strings.Contains(text, "/student/requests") {
		t.Fatalf("request email: %s", text)
	}
	// Trainers do not see students' requests.
	if got := h.code(a.trainer1, http.MethodGet, "/staff/service-requests", nil); got != "403 forbidden" {
		t.Fatalf("trainer listing requests: %s", got)
	}
}
