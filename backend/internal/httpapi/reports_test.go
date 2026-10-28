package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/reports"
)

func (h *harness) dashboard(s *session, query string) reports.Dashboard {
	h.t.Helper()
	w := h.do(req{method: http.MethodGet, path: "/api/v1/staff/reports/dashboard" + query, sess: s})
	expectStatus(h.t, w, http.StatusOK, "")
	var d reports.Dashboard
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		h.t.Fatal(err)
	}
	return d
}

// releasedClass records a released class with the given results, directly
// in the database (the results workflow is tested in results_test.go).
func (h *harness) releasedClass(school uuid.UUID, code, results string, by uuid.UUID) {
	h.t.Helper()
	var term, unit, offering, sheet uuid.UUID
	q := func(sql string, dst *uuid.UUID, args ...any) {
		if err := h.pool.QueryRow(context.Background(), sql, args...).Scan(dst); err != nil {
			h.t.Fatalf("%s: %v", sql, err)
		}
	}
	q(`INSERT INTO academic_terms (name, starts_on, ends_on) VALUES ('Term '||$1||' (Sample)', current_date - 30, current_date + 60) RETURNING id`, &term, code)
	q(`INSERT INTO units (school_id, code, title) VALUES ($1, $2, 'Unit '||$2||' (Sample)') RETURNING id`, &unit, school, code)
	q(`INSERT INTO unit_offerings (term_id, unit_id) VALUES ($1, $2) RETURNING id`, &offering, term, unit)
	q(`INSERT INTO result_sheets (offering_id, status, scheme) VALUES ($1, 'released', '{}') RETURNING id`, &sheet, offering)
	h.exec(`INSERT INTO result_releases (sheet_id, version, results, released_by) VALUES ($1, 1, $2::jsonb, $3)`, sheet, results, by)
}

// Stage 16b: each viewer sees only the sections whose data they may see;
// count exports need reports.export plus that permission, and are audited.
func TestDashboardAndExports(t *testing.T) {
	h := newHarness(t)
	medical, nursing, nursingIntake := h.school("Medical")
	ict, _, _ := h.school("=HYPERLINK(\"x\")")
	_, cs, csIntake := h.school("ICT")
	alice := h.student(nursing, nursingIntake, nil)
	bob := h.student(nursing, nursingIntake, nil)
	h.student(cs, csIntake, nil)
	h.exec(`UPDATE students SET status = 'suspended' WHERE id = $1`, bob)

	principalID := h.createUser("principal@example.invalid", "principal")
	h.createUser("registrar@example.invalid", "registrar")
	h.createUser("trainer@example.invalid", "trainer")
	hos := h.createUser("hos@example.invalid", "head_of_school")
	h.exec(`INSERT INTO school_assignments (user_id, school_id) VALUES ($1, $2)`, hos, medical)
	principal := h.mustLogin("principal@example.invalid")
	registrar := h.mustLogin("registrar@example.invalid")
	trainer := h.mustLogin("trainer@example.invalid")
	head := h.mustLogin("hos@example.invalid")
	fin := h.financeOfficer()

	// Money: a confirmed cash payment of KES 1,234.56 and a pending one.
	today := time.Now().Format(time.DateOnly)
	var pm struct {
		ID     uuid.UUID
		Status string
	}
	rec := h.keyed(fin, http.MethodPost, "/staff/finance/payments", newKey(), map[string]any{"student_id": alice, "method": "cash", "amount_cents": 123_456, "paid_on": today})
	rec.expect(t, http.StatusOK, "")
	if err := json.Unmarshal(rec.Body.Bytes(), &pm); err != nil {
		t.Fatal(err)
	}
	if pm.Status == "pending" {
		h.keyed(fin, http.MethodPost, "/staff/finance/payments/"+pm.ID.String()+"/confirm", newKey(), map[string]any{}).expect(t, http.StatusOK, "")
	}
	h.keyed(fin, http.MethodPost, "/staff/finance/payments", newKey(), map[string]any{"student_id": alice, "method": "bank", "amount_cents": 10_000, "paid_on": today, "reference": "SLIP-1"}).expect(t, http.StatusOK, "")
	h.exec(`INSERT INTO enquiries (full_name, email, query_type, message) VALUES ('Test Visitor', 'visitor@example.invalid', 'fees', 'How much?')`)
	h.releasedClass(ict, "ICT 101", `[{"passed": true}, {"passed": false}, {"outcome": "competent"}]`, principalID)

	d := h.dashboard(principal, "")
	if d.Admissions == nil || d.Students == nil || d.Results == nil || d.Finance == nil || d.Enquiries == nil {
		t.Fatalf("the principal sees every section: %+v", d)
	}
	if s := d.Students; s.Totals.Active != 2 || s.Totals.Suspended != 1 || s.Joined != 3 || len(s.BySchool) != 2 {
		t.Fatalf("students: %+v", s)
	}
	if r := d.Results; r.Released != 1 || r.ReleasedClasses != 1 || r.Results != 3 || r.Passed != 2 {
		t.Fatalf("results: %+v", r)
	}
	if f := d.Finance; f.CollectedCents != 123_456 || f.PendingPayments != 1 || f.PendingPaymentCents != 10_000 || len(f.ByMonth) != 1 {
		t.Fatalf("finance: %+v", f)
	}
	if e := d.Enquiries; e.Received != 1 || e.NewNow != 1 || len(e.ByOffice) != 1 || e.ByOffice[0].Office != "finance" {
		t.Fatalf("enquiries: %+v", e)
	}
	// A period before anything happened counts nothing in it; "now" figures stay.
	old := h.dashboard(principal, "?from=2020-01-01&to=2020-12-31")
	if old.Finance.CollectedCents != 0 || old.Enquiries.Received != 0 || old.Enquiries.NewNow != 1 || old.Students.Joined != 0 || old.Results.ReleasedClasses != 0 {
		t.Fatalf("2020: %+v %+v", old.Finance, old.Enquiries)
	}

	if d := h.dashboard(registrar, ""); d.Finance != nil || d.Students == nil || d.Admissions == nil {
		t.Fatalf("registrar sections: finance %v students %v", d.Finance != nil, d.Students != nil)
	}
	if d := h.dashboard(head, ""); d.Students != nil || d.Results != nil || d.Finance != nil || d.Admissions == nil {
		t.Fatal("a Head of School sees no college-wide student, results or finance figures")
	}
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/staff/reports/dashboard", sess: trainer}), http.StatusForbidden, "forbidden")
	for _, q := range []string{"?from=2026-13-01", "?from=2026-05-01&to=2026-04-30", "?from=2020-01-01&to=2023-01-01"} {
		expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/staff/reports/dashboard" + q, sess: principal}), http.StatusUnprocessableEntity, "validation_failed")
	}

	export := func(s *session, kind string) *http.Response {
		w := h.do(req{method: http.MethodGet, path: "/api/v1/staff/reports/exports/" + kind, sess: s})
		return w.Result()
	}
	w := h.do(req{method: http.MethodGet, path: "/api/v1/staff/reports/exports/fee-collection", sess: fin})
	expectStatus(t, w, http.StatusOK, "")
	body := w.Body.Bytes()
	if !bytes.HasPrefix(body, []byte{0xEF, 0xBB, 0xBF}) || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/csv") ||
		!strings.Contains(w.Header().Get("Content-Disposition"), "attachment; filename=\"dakota-fee-collection-") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers: %v", w.Header())
	}
	lines := strings.Split(strings.TrimSuffix(string(body[3:]), "\r\n"), "\r\n")
	if len(lines) != 3 || lines[0] != "Month,Billed (KES),Collected (KES),Charges (KES),Credits (KES)" ||
		!strings.HasSuffix(lines[1], ",0.00,1234.56,0.00,0.00") || lines[2] != "Total,0.00,1234.56,0.00,0.00" {
		t.Fatalf("fee collection CSV:\n%s", body)
	}
	if strings.Contains(string(body), "alice") || strings.Contains(string(body), "@example.invalid") {
		t.Fatal("an export carries personal data")
	}

	w = h.do(req{method: http.MethodGet, path: "/api/v1/staff/reports/exports/enrolments", sess: registrar})
	expectStatus(t, w, http.StatusOK, "")
	if csv := w.Body.String(); !strings.Contains(csv, "Medical,Diploma in Medical (Sample),suspended,1,1") || !strings.Contains(csv, "Total,,,3,3") {
		t.Fatalf("enrolments CSV:\n%s", csv)
	}
	w = h.do(req{method: http.MethodGet, path: "/api/v1/staff/reports/exports/results", sess: registrar})
	expectStatus(t, w, http.StatusOK, "")
	// The school name starts with "=": a spreadsheet must not run it.
	if csv := w.Body.String(); !strings.Contains(csv, "Total,,,,,,3,2,66.7") || !strings.Contains(csv, `"'=HYPERLINK(""x"")"`) || strings.Contains(csv, `,"=HYPERLINK`) {
		t.Fatalf("results CSV:\n%s", w.Body)
	}
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/staff/reports/exports/admissions", sess: principal}), http.StatusOK, "")

	// The data permission is needed as well as reports.export.
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/staff/reports/exports/fee-collection", sess: registrar}), http.StatusForbidden, "forbidden")
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/staff/reports/exports/admissions", sess: fin}), http.StatusForbidden, "forbidden")
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/staff/reports/exports/admissions", sess: head}), http.StatusForbidden, "forbidden")
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/staff/reports/exports/students", sess: principal}), http.StatusNotFound, "not_found")
	if r := export(principal, "results?from=nope"); r.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("bad period: %d", r.StatusCode)
	}
	if n := h.auditCount("reports.exported"); n != 4 {
		t.Fatalf("%d export audit rows, want 4 (refused exports are not audited as exports)", n)
	}
}
