package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// school makes a school with one programme and one intake; it returns
// their ids.
func (h *harness) school(name string) (school, programme, intake uuid.UUID) {
	h.t.Helper()
	ctx := context.Background()
	q := func(sql string, dst *uuid.UUID, args ...any) {
		if err := h.pool.QueryRow(ctx, sql, args...).Scan(dst); err != nil {
			h.t.Fatalf("%s: %v", sql, err)
		}
	}
	q(`INSERT INTO schools (slug, name, short_name, accent_key) VALUES ('s-'||substr(md5(random()::text),1,8), $1, 'S', 'ict') RETURNING id`, &school, name)
	q(`INSERT INTO programmes (school_id, slug, title, level) VALUES ($1, 'p-'||substr(md5(random()::text),1,8), $2, 'diploma') RETURNING id`,
		&programme, school, "Diploma in "+name+" (Sample)")
	q(`INSERT INTO programme_intakes (programme_id, name, starts_on, status) VALUES ($1, 'January (Sample)', current_date + 60, 'open') RETURNING id`, &intake, programme)
	return
}

var studentSeq int

// student makes an enrolled student directly in the database (the
// enrolment path itself is tested in admissions_test.go).
func (h *harness) student(programme, intake uuid.UUID, cohort *uuid.UUID) uuid.UUID {
	h.t.Helper()
	studentSeq++
	ctx := context.Background()
	email := fmt.Sprintf("student%03d@example.invalid", studentSeq)
	var user, applicant, application, id uuid.UUID
	q := func(sql string, dst *uuid.UUID, args ...any) {
		if err := h.pool.QueryRow(ctx, sql, args...).Scan(dst); err != nil {
			h.t.Fatalf("%s: %v", sql, err)
		}
	}
	q(`INSERT INTO users (email, full_name, kind) VALUES ($1, 'Test Student', 'student') RETURNING id`, &user, email)
	q(`INSERT INTO applicants (user_id, email, first_name, last_name) VALUES ($1, $2, 'Test', 'Student') RETURNING id`, &applicant, user, email)
	q(`INSERT INTO applications (reference, applicant_id, programme_id, intake_id) VALUES ($1, $2, $3, $4) RETURNING id`,
		&application, fmt.Sprintf("DTC-APP-TEST-%06d", studentSeq), applicant, programme, intake)
	q(`INSERT INTO students (student_number, user_id, applicant_id, application_id, programme_id, intake_id, cohort_id, first_name, last_name, email, enrolled_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'Test', $8, $9, $2) RETURNING id`,
		&id, fmt.Sprintf("DTC/2027/%04d", studentSeq), user, applicant, application, programme, intake, cohort, fmt.Sprintf("Student %03d", studentSeq), email)
	return id
}

// cohort makes a cohort; intake may be nil (one cohort per intake).
func (h *harness) cohort(programme uuid.UUID, intake *uuid.UUID) uuid.UUID {
	h.t.Helper()
	var id uuid.UUID
	if err := h.pool.QueryRow(context.Background(), `INSERT INTO cohorts (programme_id, intake_id, name) VALUES ($1, $2, 'Cohort '||substr(md5(random()::text),1,6)) RETURNING id`,
		programme, intake).Scan(&id); err != nil {
		h.t.Fatal(err)
	}
	return id
}

// call sends a request and decodes a JSON answer into out (if given).
func (h *harness) api(s *session, method, path string, body any, want int, out any) {
	h.t.Helper()
	w := h.do(req{method: method, path: "/api/v1" + path, body: body, sess: s})
	if w.Code != want {
		h.t.Fatalf("%s %s: %d, want %d: %s", method, path, w.Code, want, w.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			h.t.Fatalf("%s %s: %v", method, path, err)
		}
	}
}

func (h *harness) code(s *session, method, path string, body any) string {
	h.t.Helper()
	w := h.do(req{method: method, path: "/api/v1" + path, body: body, sess: s})
	if w.Code < 400 {
		return fmt.Sprint(w.Code)
	}
	return fmt.Sprintf("%d %s", w.Code, errorCode(h.t, w))
}

type newID struct{ ID uuid.UUID }

type offeringView struct {
	ID       uuid.UUID
	UnitCode string `json:"unit_code"`
	Trainers []struct {
		FullName string `json:"full_name"`
	}
	Timetable []struct {
		Kind     string
		StartsAt string `json:"starts_at"`
		EndsAt   string `json:"ends_at"`
	}
	ClassList []struct {
		RegistrationID uuid.UUID `json:"registration_id"`
		StudentNumber  string    `json:"student_number"`
		Status         string
	} `json:"class_list"`
}

// academy is one school's programme with a term, two units on offer, a
// cohort of students and the staff around them.
type academy struct {
	school, programme, intake, cohort, term, unit1, unit2, offering1, offering2 uuid.UUID
	students                                                                    []uuid.UUID
	registrar, trainer1, trainer2                                               *session
	trainer1ID, trainer2ID                                                      uuid.UUID
}

func (h *harness) academy() *academy {
	h.t.Helper()
	a := &academy{}
	a.school, a.programme, a.intake = h.school("Testing")
	a.cohort = h.cohort(a.programme, &a.intake)
	for range 3 {
		a.students = append(a.students, h.student(a.programme, a.intake, &a.cohort))
	}
	h.createUser("registrar@example.invalid", "registrar")
	a.trainer1ID = h.createUser("trainer1@example.invalid", "trainer")
	a.trainer2ID = h.createUser("trainer2@example.invalid", "trainer")
	a.registrar = h.mustLogin("registrar@example.invalid")
	a.trainer1 = h.mustLogin("trainer1@example.invalid")
	a.trainer2 = h.mustLogin("trainer2@example.invalid")

	start := time.Now().AddDate(0, 0, -7).Format(time.DateOnly)
	end := time.Now().AddDate(0, 3, 0).Format(time.DateOnly)
	var c newID
	h.api(a.registrar, http.MethodPost, "/staff/terms", map[string]any{"name": "Term 1 (Sample)", "starts_on": start, "ends_on": end}, http.StatusCreated, &c)
	a.term = c.ID
	h.api(a.registrar, http.MethodPost, "/staff/units", map[string]any{"school_id": a.school, "code": "tst 101", "title": "Testing basics", "hours": 40}, http.StatusCreated, &c)
	a.unit1 = c.ID
	h.api(a.registrar, http.MethodPost, "/staff/units", map[string]any{"school_id": a.school, "code": "TST 102", "title": "Testing further"}, http.StatusCreated, &c)
	a.unit2 = c.ID
	h.api(a.registrar, http.MethodPut, "/staff/programmes/"+a.programme.String()+"/units", map[string]any{"items": []map[string]any{
		{"unit_id": a.unit1, "stage": 1, "is_core": true}, {"unit_id": a.unit2, "stage": 1, "is_core": false},
	}}, http.StatusOK, nil)
	h.api(a.registrar, http.MethodPost, "/staff/offerings", map[string]any{"term_id": a.term, "unit_id": a.unit1}, http.StatusCreated, &c)
	a.offering1 = c.ID
	h.api(a.registrar, http.MethodPost, "/staff/offerings", map[string]any{"term_id": a.term, "unit_id": a.unit2}, http.StatusCreated, &c)
	a.offering2 = c.ID
	h.api(a.registrar, http.MethodPut, "/staff/offerings/"+a.offering1.String()+"/trainers", map[string]any{"trainer_ids": []uuid.UUID{a.trainer1ID}}, http.StatusOK, nil)
	h.api(a.registrar, http.MethodPut, "/staff/offerings/"+a.offering2.String()+"/trainers", map[string]any{"trainer_ids": []uuid.UUID{a.trainer2ID}}, http.StatusOK, nil)
	for _, o := range []uuid.UUID{a.offering1, a.offering2} {
		h.api(a.registrar, http.MethodPost, "/staff/offerings/"+o.String()+"/registrations", map[string]any{"cohort_id": a.cohort}, http.StatusOK, nil)
	}
	return a
}

// AC-08: a trainer sees only the units they are assigned to.
func TestTrainersSeeOnlyTheirUnits(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	h.api(a.registrar, http.MethodPost, "/staff/offerings/"+a.offering1.String()+"/timetable",
		map[string]any{"kind": "class", "weekday": 2, "starts_at": "08:30", "ends_at": "10:00", "room": "Lab 1"}, http.StatusOK, nil)

	var mine struct {
		Offerings []struct{ ID uuid.UUID }
		Timetable []struct {
			UnitCode string `json:"unit_code"`
			StartsAt string `json:"starts_at"`
		}
	}
	h.api(a.trainer1, http.MethodGet, "/staff/teaching", nil, http.StatusOK, &mine)
	if len(mine.Offerings) != 1 || mine.Offerings[0].ID != a.offering1 || len(mine.Timetable) != 1 ||
		mine.Timetable[0].UnitCode != "TST 101" || mine.Timetable[0].StartsAt != "08:30" {
		t.Fatalf("trainer 1 teaching: %+v", mine)
	}
	var class offeringView
	h.api(a.trainer1, http.MethodGet, "/staff/teaching/"+a.offering1.String(), nil, http.StatusOK, &class)
	if len(class.ClassList) != 3 {
		t.Fatalf("class list: %+v", class.ClassList)
	}
	// The class list carries no contact details.
	w := h.do(req{method: http.MethodGet, path: "/api/v1/staff/teaching/" + a.offering1.String(), sess: a.trainer1})
	if strings.Contains(w.Body.String(), "student0") {
		t.Fatal("class list shows student email addresses")
	}

	// Not another trainer's unit, not any student record, not the management views.
	for path, want := range map[string]string{
		"/staff/teaching/" + a.offering2.String():   "404 not_found",
		"/staff/teaching/" + uuid.NewString():       "404 not_found",
		"/staff/students":                           "403 forbidden",
		"/staff/students/" + a.students[0].String(): "403 forbidden",
		"/staff/offerings":                          "403 forbidden",
		"/staff/offerings/" + a.offering2.String():  "403 forbidden",
		"/staff/cohorts":                            "403 forbidden",
	} {
		if got := h.code(a.trainer1, http.MethodGet, path, nil); got != want {
			t.Errorf("trainer 1 GET %s: %s, want %s", path, got, want)
		}
	}
	h.api(a.trainer2, http.MethodGet, "/staff/teaching", nil, http.StatusOK, &mine)
	if len(mine.Offerings) != 1 || mine.Offerings[0].ID != a.offering2 {
		t.Fatalf("trainer 2 teaching: %+v", mine)
	}
	// Reassigning moves access with it, at once.
	h.api(a.registrar, http.MethodPut, "/staff/offerings/"+a.offering1.String()+"/trainers", map[string]any{"trainer_ids": []uuid.UUID{a.trainer2ID}}, http.StatusOK, nil)
	if got := h.code(a.trainer1, http.MethodGet, "/staff/teaching/"+a.offering1.String(), nil); got != "404 not_found" {
		t.Fatalf("after reassignment: %s", got)
	}
	// Only people who can teach can be assigned.
	officer := h.createUser("officer@example.invalid", "admissions_officer")
	if got := h.code(a.registrar, http.MethodPut, "/staff/offerings/"+a.offering1.String()+"/trainers", map[string]any{"trainer_ids": []uuid.UUID{officer}}); got != "422 validation_failed" {
		t.Fatalf("assigning a non-trainer: %s", got)
	}
	// The migration took students.view away from trainers.
	var n int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM role_permissions rp JOIN roles r ON r.id = rp.role_id
		WHERE r.key = 'trainer' AND rp.permission_key = 'students.view'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("trainer students.view grants = %d (%v)", n, err)
	}
}

// D-133: Heads of School work in their own schools; nothing at all without
// an assignment; terms are college-wide.
func TestHeadOfSchoolScope(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	other, otherProg, otherIntake := h.school("Other")
	otherStudent := h.student(otherProg, otherIntake, nil)

	admin := h.createUser("ict@example.invalid", "ict_admin")
	_ = admin
	ict := h.mustLogin("ict@example.invalid")
	headID := h.createUser("head@example.invalid", "head_of_school")
	head := h.mustLogin("head@example.invalid")

	// No school yet: an empty list, and every record is out of reach.
	var page struct {
		Total int
		Items []struct{ ID uuid.UUID }
	}
	h.api(head, http.MethodGet, "/staff/students", nil, http.StatusOK, &page)
	if page.Total != 0 {
		t.Fatalf("unassigned head sees %d students", page.Total)
	}
	if got := h.code(head, http.MethodGet, "/staff/students/"+a.students[0].String(), nil); got != "404 not_found" {
		t.Fatalf("unassigned head, student: %s", got)
	}

	var schools struct{ Items []struct{ ID uuid.UUID } }
	h.api(ict, http.MethodPut, "/admin/users/"+headID.String()+"/schools", map[string]any{"school_ids": []uuid.UUID{a.school}}, http.StatusOK, &schools)
	if len(schools.Items) != 1 || schools.Items[0].ID != a.school {
		t.Fatalf("schools: %+v", schools)
	}
	if got := h.code(ict, http.MethodPut, "/admin/users/"+headID.String()+"/schools", map[string]any{"school_ids": []uuid.UUID{uuid.New()}}); got != "422 validation_failed" {
		t.Fatalf("unknown school: %s", got)
	}
	if got := h.code(head, http.MethodPut, "/admin/users/"+headID.String()+"/schools", map[string]any{"school_ids": []uuid.UUID{other}}); got != "403 forbidden" {
		t.Fatalf("head assigning themselves: %s", got)
	}

	h.api(head, http.MethodGet, "/staff/students", nil, http.StatusOK, &page)
	if page.Total != 3 {
		t.Fatalf("head sees %d students, want 3", page.Total)
	}
	for path, want := range map[string]string{
		"/staff/students/" + a.students[0].String(): "200",
		"/staff/students/" + otherStudent.String():  "404 not_found",
		"/staff/offerings/" + a.offering1.String():  "200",
	} {
		if got := h.code(head, http.MethodGet, path, nil); got != want {
			t.Errorf("head GET %s: %s, want %s", path, got, want)
		}
	}
	// Changes stay inside the school too.
	if got := h.code(head, http.MethodPost, "/staff/units", map[string]any{"school_id": other, "code": "OTH 101", "title": "Not mine"}); got != "403 outside_your_schools" {
		t.Fatalf("unit in another school: %s", got)
	}
	if got := h.code(head, http.MethodPost, "/staff/units", map[string]any{"school_id": a.school, "code": "TST 201", "title": "Mine"}); got != "201" {
		t.Fatalf("unit in own school: %s", got)
	}
	if got := h.code(head, http.MethodPut, "/staff/programmes/"+otherProg.String()+"/units", map[string]any{"items": []any{}}); got != "404 not_found" {
		t.Fatalf("another school's programme: %s", got)
	}
	if got := h.code(head, http.MethodPost, "/staff/terms", map[string]any{"name": "Head term", "starts_on": "2027-01-04", "ends_on": "2027-04-01"}); got != "403 college_wide" {
		t.Fatalf("head adding a term: %s", got)
	}
	// Registering students of another school counts them as not eligible.
	var reg struct {
		Result struct {
			Registered  int `json:"registered"`
			NotEligible int `json:"not_eligible"`
		}
	}
	h.api(head, http.MethodPost, "/staff/offerings/"+a.offering1.String()+"/registrations", map[string]any{"student_ids": []uuid.UUID{otherStudent}}, http.StatusOK, &reg)
	if reg.Result.Registered != 0 || reg.Result.NotEligible != 1 {
		t.Fatalf("other school's student: %+v", reg.Result)
	}
	// The registrar (schools.all) sees both schools.
	h.api(a.registrar, http.MethodGet, "/staff/students", nil, http.StatusOK, &page)
	if page.Total != 4 {
		t.Fatalf("registrar sees %d students, want 4", page.Total)
	}
}

// Student records: status changes with reasons and history, guardians,
// cohort moves; every change audited.
func TestStudentRecords(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	id := a.students[0].String()
	path := "/staff/students/" + id

	type student struct {
		Status    string
		CohortID  *uuid.UUID `json:"cohort_id"`
		Guardians []struct {
			ID        uuid.UUID
			FullName  string `json:"full_name"`
			IsPrimary bool   `json:"is_primary"`
		}
		History []struct {
			From          *string
			To            string
			Reason        string
			ChangedByName *string `json:"changed_by_name"`
		}
		Registrations []struct {
			UnitCode string `json:"unit_code"`
		}
	}
	var st student
	h.api(a.registrar, http.MethodGet, path, nil, http.StatusOK, &st)
	if st.Status != "active" || len(st.Registrations) != 2 {
		t.Fatalf("student: %+v", st)
	}

	if got := h.code(a.registrar, http.MethodPost, path+"/status", map[string]any{"status": "deferred", "reason": ""}); got != "422 validation_failed" {
		t.Fatalf("no reason: %s", got)
	}
	if got := h.code(a.registrar, http.MethodPost, path+"/status", map[string]any{"status": "expelled", "reason": "x"}); got != "422 validation_failed" {
		t.Fatalf("unknown status: %s", got)
	}
	h.api(a.registrar, http.MethodPost, path+"/status", map[string]any{"status": "deferred", "reason": "Medical, one term."}, http.StatusOK, &st)
	if got := h.code(a.registrar, http.MethodPost, path+"/status", map[string]any{"status": "deferred", "reason": "Again"}); got != "409 unchanged" {
		t.Fatalf("same status: %s", got)
	}
	h.api(a.registrar, http.MethodPost, path+"/status", map[string]any{"status": "graduated", "reason": "Completed."}, http.StatusOK, &st)
	if got := h.code(a.registrar, http.MethodPost, path+"/status", map[string]any{"status": "active", "reason": "Oops"}); got != "422 validation_failed" {
		t.Fatalf("after graduation: %s", got)
	}
	if len(st.History) != 2 || st.History[1].To != "graduated" || st.History[0].Reason != "Medical, one term." || st.History[0].ChangedByName == nil {
		t.Fatalf("history: %+v", st.History)
	}
	// The history is append-only.
	if _, err := h.pool.Exec(context.Background(), `UPDATE student_status_history SET reason = 'changed'`); err == nil {
		t.Fatal("status history was edited")
	}

	// Guardians: one primary at a time; a phone or an email is needed.
	g := path + "/guardians"
	if got := h.code(a.registrar, http.MethodPost, g, map[string]any{"full_name": "Test Parent", "relationship": "mother", "phone": "", "email": "", "is_primary": true}); got != "422 validation_failed" {
		t.Fatalf("no contact: %s", got)
	}
	h.api(a.registrar, http.MethodPost, g, map[string]any{"full_name": "Test Parent 1", "relationship": "mother", "phone": "+254 700 000 101", "email": "", "is_primary": true}, http.StatusOK, &st)
	h.api(a.registrar, http.MethodPost, g, map[string]any{"full_name": "Test Parent 2", "relationship": "father", "phone": "", "email": "parent2@example.invalid", "is_primary": true}, http.StatusOK, &st)
	if len(st.Guardians) != 2 || !st.Guardians[0].IsPrimary || st.Guardians[0].FullName != "Test Parent 2" || st.Guardians[1].IsPrimary {
		t.Fatalf("guardians: %+v", st.Guardians)
	}
	h.api(a.registrar, http.MethodDelete, g+"/"+st.Guardians[1].ID.String(), nil, http.StatusOK, &st)
	if len(st.Guardians) != 1 {
		t.Fatalf("after removal: %+v", st.Guardians)
	}

	// A cohort of the student's programme only.
	_, otherProg, otherIntake := h.school("Other")
	if got := h.code(a.registrar, http.MethodPost, path+"/cohort", map[string]any{"cohort_id": h.cohort(otherProg, &otherIntake)}); got != "422 validation_failed" {
		t.Fatalf("cohort of another programme: %s", got)
	}
	repeat := h.cohort(a.programme, nil)
	h.api(a.registrar, http.MethodPost, path+"/cohort", map[string]any{"cohort_id": repeat}, http.StatusOK, &st)
	if st.CohortID == nil || *st.CohortID != repeat {
		t.Fatalf("cohort: %+v", st.CohortID)
	}

	// Head of School may read (students.view) but not change (no students.update).
	headID := h.createUser("head@example.invalid", "head_of_school")
	h.exec(`INSERT INTO school_assignments (user_id, school_id) VALUES ($1, $2)`, headID, a.school)
	head := h.mustLogin("head@example.invalid")
	if got := h.code(head, http.MethodPost, path+"/status", map[string]any{"status": "active", "reason": "x"}); got != "403 forbidden" {
		t.Fatalf("head changing status: %s", got)
	}
	for _, action := range []string{"student.status_changed", "student.guardian_added", "student.guardian_removed", "student.cohort_changed"} {
		if h.auditCount(action) == 0 {
			t.Errorf("%s not audited", action)
		}
	}
}

// Offerings: timetable rules, registration rules, drops kept.
func TestOfferingsAndRegistration(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	o := "/staff/offerings/" + a.offering1.String()

	for _, bad := range []map[string]any{
		{"kind": "class", "starts_at": "08:00", "ends_at": "09:00"},                                                              // no weekday
		{"kind": "class", "weekday": 1, "starts_at": "10:00", "ends_at": "09:00"},                                                // ends first
		{"kind": "exam", "on_date": time.Now().AddDate(1, 0, 0).Format(time.DateOnly), "starts_at": "09:00", "ends_at": "11:00"}, // after the term
		{"kind": "lecture", "weekday": 1, "starts_at": "08:00", "ends_at": "09:00"},
		{"kind": "class", "weekday": 1, "starts_at": "8am", "ends_at": "09:00"},
	} {
		if got := h.code(a.registrar, http.MethodPost, o+"/timetable", bad); got != "422 validation_failed" {
			t.Errorf("timetable %v: %s", bad, got)
		}
	}
	var view offeringView
	h.api(a.registrar, http.MethodPost, o+"/timetable", map[string]any{"kind": "exam", "on_date": time.Now().AddDate(0, 1, 0).Format(time.DateOnly), "starts_at": "09:00", "ends_at": "11:00", "room": "Hall"}, http.StatusOK, &view)
	if len(view.Timetable) != 1 || view.Timetable[0].EndsAt != "11:00" {
		t.Fatalf("timetable: %+v", view.Timetable)
	}
	if got := h.code(a.registrar, http.MethodPost, "/staff/offerings", map[string]any{"term_id": a.term, "unit_id": a.unit1}); got != "409 duplicate" {
		t.Fatalf("same unit twice in a term: %s", got)
	}

	// Registered already by cohort; again changes nothing; a student of
	// another programme and an inactive one are not eligible.
	_, otherProg, otherIntake := h.school("Other")
	outsider := h.student(otherProg, otherIntake, nil)
	h.exec(`UPDATE students SET status = 'suspended' WHERE id = $1`, a.students[2])
	var reg struct {
		Result struct {
			Registered        int `json:"registered"`
			AlreadyRegistered int `json:"already_registered"`
			NotEligible       int `json:"not_eligible"`
		}
		Offering offeringView
	}
	h.api(a.registrar, http.MethodPost, o+"/registrations", map[string]any{"student_ids": []uuid.UUID{a.students[0], a.students[2], outsider}}, http.StatusOK, &reg)
	if reg.Result.Registered != 0 || reg.Result.AlreadyRegistered != 1 || reg.Result.NotEligible != 2 {
		t.Fatalf("register: %+v", reg.Result)
	}
	if got := h.code(a.registrar, http.MethodPost, o+"/registrations", map[string]any{}); got != "422 validation_failed" {
		t.Fatalf("nobody to register: %s", got)
	}

	// Dropping keeps the row; registering again brings it back.
	first := reg.Offering.ClassList[0]
	h.api(a.registrar, http.MethodPost, o+"/registrations/"+first.RegistrationID.String()+"/drop", map[string]any{}, http.StatusOK, &view)
	dropped := 0
	for _, c := range view.ClassList {
		if c.Status == "dropped" {
			dropped++
		}
	}
	if dropped != 1 || len(view.ClassList) != 3 {
		t.Fatalf("after drop: %+v", view.ClassList)
	}
	if got := h.code(a.registrar, http.MethodPost, o+"/registrations/"+first.RegistrationID.String()+"/drop", map[string]any{}); got != "404 not_found" {
		t.Fatalf("drop twice: %s", got)
	}

	// After the term ends, registration is closed.
	h.exec(`UPDATE academic_terms SET starts_on = current_date - 100, ends_on = current_date - 1 WHERE id = $1`, a.term)
	if got := h.code(a.registrar, http.MethodPost, o+"/registrations", map[string]any{"cohort_id": a.cohort}); got != "422 validation_failed" {
		t.Fatalf("after the term: %s", got)
	}
	// Units: codes are upper-cased and unique.
	var unit struct{ Code string }
	h.api(a.registrar, http.MethodGet, "/staff/units/"+a.unit1.String(), nil, http.StatusOK, &unit)
	if unit.Code != "TST 101" {
		t.Fatalf("code = %q", unit.Code)
	}
	if got := h.code(a.registrar, http.MethodPost, "/staff/units", map[string]any{"school_id": a.school, "code": "TST 101", "title": "Copy"}); got != "422 validation_failed" {
		t.Fatalf("duplicate code: %s", got)
	}
	// Any member of staff can read the catalogue and terms; applicants cannot.
	editor := h.createUser("editor@example.invalid", "content_editor")
	_ = editor
	ed := h.mustLogin("editor@example.invalid")
	if got := h.code(ed, http.MethodGet, "/staff/units", nil); got != "200" {
		t.Fatalf("editor reading units: %s", got)
	}
	ap := h.newApplicant("applicant900@example.invalid", "")
	if got := h.code(ap.session, http.MethodGet, "/staff/terms", nil); got != "403 forbidden" {
		t.Fatalf("applicant reading terms: %s", got)
	}
}

// Enrolment files the new student into their intake's cohort and starts
// their status history (D-134).
func TestEnrolmentFilesCohort(t *testing.T) {
	h := newHarness(t)
	w := h.actors()
	ids := []string{}
	for i := range 2 {
		ap, id := h.submitted(fmt.Sprintf("applicant18%d@example.invalid", i))
		h.staffAct(w.officer, id, "assign", map[string]any{"assignee_id": w.officerID}).expect(t, http.StatusOK, "")
		h.staffAct(w.officer, id, "offer", nil).expect(t, http.StatusOK, "")
		h.applicantAct(ap.session, id, "accept-offer", map[string]any{}).expect(t, http.StatusOK, "")
		h.staffAct(w.registrar, id, "enrol", nil).expect(t, http.StatusOK, "")
		ids = append(ids, h.staffApp(w.registrar, id, http.StatusOK).Student.ID)
	}
	var cohorts, history int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(DISTINCT cohort_id), (SELECT count(*) FROM student_status_history WHERE student_id = ANY($1::uuid[]))
		FROM students WHERE id = ANY($1::uuid[]) AND cohort_id IS NOT NULL`, ids).Scan(&cohorts, &history); err != nil {
		t.Fatal(err)
	}
	// Each test application has its own programme (readyApplication), so two cohorts.
	if cohorts != 2 || history != 2 {
		t.Fatalf("cohorts %d, history rows %d", cohorts, history)
	}
}
