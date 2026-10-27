package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"regexp"
	"testing"

	"github.com/google/uuid"
)

type sheetView struct {
	ID     uuid.UUID
	Status string
	Scheme struct {
		Kind      string
		IsDefault bool `json:"is_default"`
	}
	Rows []struct {
		StudentID uuid.UUID `json:"student_id"`
		Result    struct {
			Complete bool
			Total    *int
			Grade    string
			Passed   bool
		}
	}
	Complete        bool
	Events          []struct{ From, To, Reason string }
	ReleasedVersion int `json:"released_version"`
}

type studentResults struct {
	Items []struct {
		UnitCode string `json:"unit_code"`
		Total    *int
		Grade    string
		Version  int
	}
	Transcripts []struct {
		ID     uuid.UUID
		Number string
	}
}

// marks enters the same CAT and exam marks for every student.
func marksFor(students []uuid.UUID, cat, exam int) map[string]any {
	var entries []map[string]any
	for _, s := range students {
		entries = append(entries, map[string]any{"student_id": s, "component": "cat", "score": cat}, map[string]any{"student_id": s, "component": "exam", "score": exam})
	}
	return map[string]any{"entries": entries}
}

// results sets up a head assigned to the academy's school.
func (h *harness) head(school uuid.UUID) *session {
	h.t.Helper()
	id := h.createUser("head@example.invalid", "head_of_school")
	h.exec(`INSERT INTO school_assignments (user_id, school_id) VALUES ($1, $2)`, id, school)
	return h.mustLogin("head@example.invalid")
}

// AC-09: results are visible to students only after approval and release,
// and every change is kept.
func TestResultsReachStudentsOnlyWhenReleased(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	head := h.head(a.school)
	alice := h.studentLogin(a.students[0])
	class := "/staff/teaching/" + a.offering1.String() + "/results"
	var sh sheetView
	var mine studentResults
	seen := func(want int) {
		t.Helper()
		h.api(alice, http.MethodGet, "/student/results", nil, http.StatusOK, &mine)
		if len(mine.Items) != want {
			t.Fatalf("student sees %d results, want %d", len(mine.Items), want)
		}
	}

	h.api(a.trainer1, http.MethodGet, class, nil, http.StatusOK, &sh)
	if sh.Status != "draft" || sh.Scheme.Kind != "marks" || !sh.Scheme.IsDefault || len(sh.Rows) != 3 {
		t.Fatalf("new sheet: %+v", sh)
	}
	// Part of the marks: cannot submit yet.
	h.api(a.trainer1, http.MethodPut, class+"/marks", marksFor(a.students[:1], 80, 70), http.StatusOK, &sh)
	if got := h.code(a.trainer1, http.MethodPost, class+"/submit", map[string]any{}); got != "422 validation_failed" {
		t.Fatalf("submit incomplete: %s", got)
	}
	if got := h.code(a.trainer1, http.MethodPut, class+"/marks", map[string]any{"entries": []map[string]any{{"student_id": a.students[1], "component": "cat", "score": 101}}}); got != "422 validation_failed" {
		t.Fatalf("score above 100: %s", got)
	}
	if got := h.code(a.trainer1, http.MethodPut, class+"/marks", map[string]any{"entries": []map[string]any{{"student_id": uuid.New(), "component": "cat", "score": 50}}}); got != "422 validation_failed" {
		t.Fatalf("student not in class: %s", got)
	}
	h.api(a.trainer1, http.MethodPut, class+"/marks", marksFor(a.students[1:], 50, 40), http.StatusOK, &sh)
	if !sh.Complete || *sh.Rows[0].Result.Total+*sh.Rows[1].Result.Total+*sh.Rows[2].Result.Total != 73+43+43 {
		t.Fatalf("computed: %+v", sh.Rows)
	}
	h.api(a.trainer1, http.MethodPost, class+"/submit", map[string]any{}, http.StatusOK, &sh)
	if got := h.code(a.trainer1, http.MethodPut, class+"/marks", marksFor(a.students[:1], 90, 90)); got != "409 waiting_for_approval" {
		t.Fatalf("marks while submitted: %s", got)
	}
	seen(0)

	// Each role does only its own step.
	approvals := "/staff/result-approvals/" + sh.ID.String()
	releases := "/staff/result-releases/" + sh.ID.String()
	if got := h.code(a.registrar, http.MethodPost, approvals+"/approve", map[string]any{}); got != "403 forbidden" {
		t.Fatalf("registrar approving: %s", got)
	}
	if got := h.code(a.trainer1, http.MethodPost, approvals+"/approve", map[string]any{}); got != "403 forbidden" {
		t.Fatalf("trainer approving: %s", got)
	}
	if got := h.code(head, http.MethodPost, releases+"/release", map[string]any{}); got != "403 forbidden" {
		t.Fatalf("head releasing: %s", got)
	}
	if got := h.code(a.registrar, http.MethodPost, releases+"/release", map[string]any{}); got != "409 not_allowed" {
		t.Fatalf("releasing before approval: %s", got)
	}
	var list struct{ Items []struct{ ID uuid.UUID } }
	h.api(head, http.MethodGet, "/staff/result-approvals", nil, http.StatusOK, &list)
	if len(list.Items) != 1 || list.Items[0].ID != sh.ID {
		t.Fatalf("head's queue: %+v", list.Items)
	}
	h.api(head, http.MethodPost, approvals+"/approve", map[string]any{}, http.StatusOK, &sh)
	seen(0) // approved is not released
	h.api(a.registrar, http.MethodPost, releases+"/release", map[string]any{}, http.StatusOK, &sh)
	seen(1)
	if mine.Items[0].UnitCode != "TST 101" || *mine.Items[0].Total != 73 || mine.Items[0].Grade != "A" || mine.Items[0].Version != 1 {
		t.Fatalf("released: %+v", mine.Items[0])
	}

	// A correction needs a reason, is invisible until released again.
	if got := h.code(a.trainer1, http.MethodPut, class+"/marks", marksFor(a.students[:1], 40, 40)); got != "422 validation_failed" {
		t.Fatalf("correction without reason: %s", got)
	}
	corr := marksFor(a.students[:1], 40, 40)
	corr["reason"] = "Exam script re-marked."
	h.api(a.trainer1, http.MethodPut, class+"/marks", corr, http.StatusOK, &sh)
	if sh.Status != "draft" {
		t.Fatalf("after correction: %s", sh.Status)
	}
	seen(1)
	if *mine.Items[0].Total != 73 {
		t.Fatalf("correction leaked before release: %+v", mine.Items[0])
	}
	h.api(a.trainer1, http.MethodPost, class+"/submit", map[string]any{}, http.StatusOK, nil)
	h.api(head, http.MethodPost, approvals+"/approve", map[string]any{}, http.StatusOK, nil)
	h.api(a.registrar, http.MethodPost, releases+"/release", map[string]any{}, http.StatusOK, &sh)
	seen(1)
	if *mine.Items[0].Total != 40 || mine.Items[0].Grade != "D" || mine.Items[0].Version != 2 || sh.ReleasedVersion != 2 {
		t.Fatalf("after re-release: %+v (sheet v%d)", mine.Items[0], sh.ReleasedVersion)
	}

	// Every change is kept: marks, events and releases cannot be edited.
	for _, sql := range []string{`UPDATE marks SET score = 99`, `DELETE FROM result_sheet_events`, `UPDATE result_releases SET version = 9`} {
		if _, err := h.pool.Exec(context.Background(), sql); err == nil {
			t.Errorf("%s was allowed", sql)
		}
	}
	var markRows int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM marks`).Scan(&markRows); err != nil || markRows != 8 {
		t.Fatalf("mark rows = %d, want 8 (6 entries and 2 corrections)", markRows)
	}
	for action, want := range map[string]int{"results.marks_entered": 3, "results.submitted": 2, "results.approved": 2, "results.released": 2} {
		if n := h.auditCount(action); n != want {
			t.Errorf("%s audited %d times, want %d", action, n, want)
		}
	}
}

// Trainers reach only their classes; heads only their schools' sheets.
func TestResultsScope(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	if got := h.code(a.trainer2, http.MethodGet, "/staff/teaching/"+a.offering1.String()+"/results", nil); got != "404 not_found" {
		t.Fatalf("other trainer: %s", got)
	}
	var sh sheetView
	h.api(a.trainer1, http.MethodGet, "/staff/teaching/"+a.offering1.String()+"/results", nil, http.StatusOK, &sh)
	other, _, _ := h.school("Other")
	head := h.head(other)
	if got := h.code(head, http.MethodGet, "/staff/result-approvals/"+sh.ID.String(), nil); got != "404 not_found" {
		t.Fatalf("other school's head: %s", got)
	}
	var list struct{ Items []any }
	h.api(head, http.MethodGet, "/staff/result-approvals?status=draft", nil, http.StatusOK, &list)
	if len(list.Items) != 0 {
		t.Fatalf("other school's head lists %d sheets", len(list.Items))
	}
	alice := h.studentLogin(a.students[0])
	if got := h.code(alice, http.MethodGet, "/staff/teaching/"+a.offering1.String()+"/results", nil); got != "403 forbidden" {
		t.Fatalf("student on a staff route: %s", got)
	}
}

// Returns need a reason; competency programmes use outcomes.
func TestReturnAndCompetency(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	head := h.head(a.school)
	assessment := "/staff/programmes/" + a.programme.String() + "/assessment"
	if got := h.code(a.registrar, http.MethodPut, assessment, map[string]any{"kind": "marks", "components": []map[string]any{{"name": "Exam", "weight": 90}}, "pass_mark": 40, "grade_bands": []map[string]any{{"grade": "P", "min": 0}}}); got != "422 validation_failed" {
		t.Fatalf("weights not 100: %s", got)
	}
	if got := h.code(a.trainer1, http.MethodPut, assessment, map[string]any{"kind": "competency"}); got != "403 forbidden" {
		t.Fatalf("trainer setting the scheme: %s", got)
	}
	h.api(a.registrar, http.MethodPut, assessment, map[string]any{"kind": "competency", "components": []any{}, "pass_mark": 0, "grade_bands": []any{}}, http.StatusOK, nil)

	class := "/staff/teaching/" + a.offering2.String() + "/results"
	var sh sheetView
	h.api(a.trainer2, http.MethodGet, class, nil, http.StatusOK, &sh)
	if sh.Scheme.Kind != "competency" {
		t.Fatalf("scheme: %+v", sh.Scheme)
	}
	var entries []map[string]any
	for i, s := range a.students {
		outcome := "competent"
		if i == 2 {
			outcome = "not_yet_competent"
		}
		entries = append(entries, map[string]any{"student_id": s, "component": "competency", "outcome": outcome})
	}
	if got := h.code(a.trainer2, http.MethodPut, class+"/marks", map[string]any{"entries": []map[string]any{{"student_id": a.students[0], "component": "competency", "score": 80}}}); got != "422 validation_failed" {
		t.Fatalf("a score on a competency unit: %s", got)
	}
	h.api(a.trainer2, http.MethodPut, class+"/marks", map[string]any{"entries": entries}, http.StatusOK, &sh)
	if !sh.Complete || !sh.Rows[0].Result.Passed {
		t.Fatalf("competency results: %+v", sh.Rows)
	}
	h.api(a.trainer2, http.MethodPost, class+"/submit", map[string]any{}, http.StatusOK, &sh)
	approvals := "/staff/result-approvals/" + sh.ID.String()
	if got := h.code(head, http.MethodPost, approvals+"/return", map[string]any{"reason": " "}); got != "422 validation_failed" {
		t.Fatalf("return without reason: %s", got)
	}
	h.api(head, http.MethodPost, approvals+"/return", map[string]any{"reason": "Check student 3's practical."}, http.StatusOK, &sh)
	if sh.Status != "draft" || sh.Events[len(sh.Events)-1].Reason != "Check student 3's practical." {
		t.Fatalf("after return: %s %+v", sh.Status, sh.Events)
	}
}

var transcriptNumber = regexp.MustCompile(`^DTC/TR/\d{4}/0001$`)

// Results slips for students; official transcripts from the registrar.
func TestResultsSlipAndTranscripts(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	head := h.head(a.school)
	alice, bob := h.studentLogin(a.students[0]), h.studentLogin(a.students[1])

	// Nothing released: a transcript cannot be issued.
	if got := h.code(a.registrar, http.MethodPost, "/staff/students/"+a.students[0].String()+"/transcripts", map[string]any{}); got != "422 validation_failed" {
		t.Fatalf("transcript before any release: %s", got)
	}
	class := "/staff/teaching/" + a.offering1.String() + "/results"
	var sh sheetView
	h.api(a.trainer1, http.MethodPut, class+"/marks", marksFor(a.students, 60, 60), http.StatusOK, &sh)
	h.api(a.trainer1, http.MethodPost, class+"/submit", map[string]any{}, http.StatusOK, &sh)
	h.api(head, http.MethodPost, "/staff/result-approvals/"+sh.ID.String()+"/approve", map[string]any{}, http.StatusOK, nil)
	h.api(a.registrar, http.MethodPost, "/staff/result-releases/"+sh.ID.String()+"/release", map[string]any{}, http.StatusOK, nil)

	w := h.do(req{method: http.MethodGet, path: "/api/v1/student/results/slip", sess: alice})
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/pdf" || !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF-")) {
		t.Fatalf("slip: %d %s", w.Code, w.Header())
	}

	// A transcript request, answered by issuing the transcript.
	h.api(alice, http.MethodPost, "/student/requests", map[string]any{"kind": "transcript", "details": "For a job application."}, http.StatusOK, nil)
	var reqID uuid.UUID
	if err := h.pool.QueryRow(context.Background(), `SELECT id FROM service_requests WHERE student_id = $1`, a.students[0]).Scan(&reqID); err != nil {
		t.Fatal(err)
	}
	// Not with another student's request.
	if got := h.code(a.registrar, http.MethodPost, "/staff/students/"+a.students[1].String()+"/transcripts", map[string]any{"service_request_id": reqID}); got != "422 validation_failed" {
		t.Fatalf("another student's request: %s", got)
	}
	if got := h.code(head, http.MethodPost, "/staff/students/"+a.students[0].String()+"/transcripts", map[string]any{}); got != "403 forbidden" {
		t.Fatalf("head issuing a transcript: %s", got)
	}
	var issued struct {
		Transcripts []struct {
			ID     uuid.UUID
			Number string
		}
	}
	h.api(a.registrar, http.MethodPost, "/staff/students/"+a.students[0].String()+"/transcripts", map[string]any{"service_request_id": reqID}, http.StatusOK, &issued)
	if len(issued.Transcripts) != 1 || !transcriptNumber.MatchString(issued.Transcripts[0].Number) {
		t.Fatalf("issued: %+v", issued)
	}
	var status string
	if err := h.pool.QueryRow(context.Background(), `SELECT status FROM service_requests WHERE id = $1`, reqID).Scan(&status); err != nil || status != "done" {
		t.Fatalf("request status %q (%v)", status, err)
	}

	var mine studentResults
	h.api(alice, http.MethodGet, "/student/results", nil, http.StatusOK, &mine)
	if len(mine.Transcripts) != 1 {
		t.Fatalf("student's transcripts: %+v", mine.Transcripts)
	}
	tid := mine.Transcripts[0].ID.String()
	url := h.link(req{method: http.MethodPost, path: "/api/v1/student/transcripts/" + tid + "/link", sess: alice})
	if w := h.do(req{method: http.MethodGet, path: url, sess: alice}); w.Code != http.StatusOK || !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF-")) {
		t.Fatalf("transcript download: %d", w.Code)
	}
	if got := h.code(bob, http.MethodPost, "/student/transcripts/"+tid+"/link", map[string]any{}); got != "404 not_found" {
		t.Fatalf("another student's transcript: %s", got)
	}
	if n := h.auditCount("results.transcript_issued"); n != 1 {
		t.Fatalf("transcripts audited %d times", n)
	}
}
