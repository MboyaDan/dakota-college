package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func inboxPath(rest ...string) string {
	return "/api/v1/staff/enquiries" + strings.Join(append([]string{""}, rest...), "/")
}

func (h *harness) enquiryIDByEmail(email string) string {
	h.t.Helper()
	var id string
	if err := h.pool.QueryRow(context.Background(), `SELECT id::text FROM enquiries WHERE email = $1`, email).Scan(&id); err != nil {
		h.t.Fatal(err)
	}
	return id
}

func itemsOf(page map[string]any) []map[string]any {
	var out []map[string]any
	for _, it := range page["items"].([]any) {
		out = append(out, it.(map[string]any))
	}
	return out
}

// Stage 8 done-when: an enquiry reaches the right inbox and can be assigned.
func TestEnquiryReachesTheRightInboxAndCanBeAssigned(t *testing.T) {
	h := newHarness(t)
	h.setSetting("contact.email", "info@example.invalid")
	h.setSetting("contact.finance_email", "finance-office@example.invalid")
	financeID := h.createUser("finance@example.invalid", "finance_officer")
	h.createUser("admissions@example.invalid", "admissions_officer")
	finance := h.mustLogin("finance@example.invalid")
	admissions := h.mustLogin("admissions@example.invalid")

	h.sendEnquiry(enquiryBody(map[string]any{"query_type": "fees", "email": "parent@example.invalid", "message": "How much is the diploma?"}), http.StatusAccepted)
	h.sendEnquiry(enquiryBody(map[string]any{"query_type": "admissions", "email": "applicant@example.invalid"}), http.StatusAccepted)
	h.waitForMail(2)
	feesID := h.enquiryIDByEmail("parent@example.invalid")

	// The finance office's inbox has the fee enquiry and not the admissions one.
	financeInbox := itemsOf(h.call(req{method: http.MethodGet, path: inboxPath() + "?office=finance", sess: finance}, http.StatusOK))
	if len(financeInbox) != 1 || financeInbox[0]["id"] != feesID || financeInbox[0]["status"] != "new" || financeInbox[0]["office"] != "finance" {
		t.Fatalf("finance inbox: %v", financeInbox)
	}
	if got := itemsOf(h.call(req{method: http.MethodGet, path: inboxPath() + "?office=admissions", sess: admissions}, http.StatusOK)); len(got) != 1 || got[0]["query_type"] != "admissions" {
		t.Fatalf("admissions inbox: %v", got)
	}

	// Admissions assigns the fee enquiry to the finance officer, who is emailed
	// a link without the visitor's details.
	got := h.call(req{method: http.MethodPost, path: inboxPath(feesID, "assign"), body: map[string]any{"assignee_id": financeID.String()}, sess: admissions}, http.StatusOK)
	if got["assignee_name"] != "Test User finance@example.invalid" {
		t.Fatalf("assigned: %v", got)
	}
	m := h.waitForMail(3)[2]
	if m.To != "finance@example.invalid" || !strings.Contains(m.Text, "/staff/enquiries/"+feesID) ||
		strings.Contains(m.Text, "parent@example.invalid") || strings.Contains(m.Text, "How much") {
		t.Fatalf("assignment email: %+v", m)
	}
	mine := itemsOf(h.call(req{method: http.MethodGet, path: inboxPath() + "?assigned=me", sess: finance}, http.StatusOK))
	if len(mine) != 1 || mine[0]["id"] != feesID {
		t.Fatalf("assigned to me: %v", mine)
	}
	if counts := h.call(req{method: http.MethodGet, path: inboxPath("counts"), sess: finance}, http.StatusOK); counts["new"] != float64(2) || counts["mine"] != float64(1) {
		t.Fatalf("counts: %v", counts)
	}

	// The finance officer notes, waits for the parent, then closes it.
	h.call(req{method: http.MethodPost, path: inboxPath(feesID, "notes"), body: map[string]any{"body": "Called the parent; sending the fee sheet."}, sess: finance}, http.StatusCreated)
	h.call(req{method: http.MethodPost, path: inboxPath(feesID, "status"), body: map[string]any{"status": "waiting"}, sess: finance}, http.StatusOK)
	closed := h.call(req{method: http.MethodPost, path: inboxPath(feesID, "status"), body: map[string]any{"status": "closed"}, sess: finance}, http.StatusOK)
	if closed["status"] != "closed" || closed["closed_at"] == nil {
		t.Fatalf("closed: %v", closed)
	}
	notes := closed["notes"].([]any)
	if len(notes) != 1 || notes[0].(map[string]any)["author_name"] != "Test User finance@example.invalid" {
		t.Fatalf("notes: %v", notes)
	}
	var actions []string
	for _, e := range closed["history"].([]any) {
		actions = append(actions, e.(map[string]any)["action"].(string))
	}
	want := "enquiry.assigned enquiry.note_added enquiry.status_changed enquiry.status_changed enquiry.status_changed"
	if strings.Join(actions, " ") != want {
		t.Fatalf("history %v, want %s (the note on a new enquiry opens it)", actions, want)
	}
	if n := h.auditCount("enquiry.note_added"); n != 1 {
		t.Fatalf("note audits: %d", n)
	}

	// Reopening clears the closing time.
	reopened := h.call(req{method: http.MethodPost, path: inboxPath(feesID, "status"), body: map[string]any{"status": "open"}, sess: finance}, http.StatusOK)
	if reopened["closed_at"] != nil {
		t.Fatalf("reopened: %v", reopened)
	}
}

func TestEnquiryInboxPermissionsAndRules(t *testing.T) {
	h := newHarness(t)
	h.createUser("principal@example.invalid", "principal")
	h.createUser("registrar@example.invalid", "registrar")
	editorID := h.createUser("editor@example.invalid", "content_editor")
	principal := h.mustLogin("principal@example.invalid")
	registrar := h.mustLogin("registrar@example.invalid")
	editor := h.mustLogin("editor@example.invalid")

	h.sendEnquiry(enquiryBody(map[string]any{"email": "one@example.invalid", "full_name": "Amina Test"}), http.StatusAccepted)
	h.sendEnquiry(enquiryBody(map[string]any{"email": "two@example.invalid", "query_type": "alumni", "message": "Transcript please"}), http.StatusAccepted)
	id := h.enquiryIDByEmail("one@example.invalid")

	// No inbox for roles without enquiries.view; read-only for the Principal.
	expectStatus(t, h.do(req{method: http.MethodGet, path: inboxPath(), sess: editor}), http.StatusForbidden, "forbidden")
	h.call(req{method: http.MethodGet, path: inboxPath(id), sess: principal}, http.StatusOK)
	expectStatus(t, h.do(req{method: http.MethodPost, path: inboxPath(id, "status"), body: map[string]any{"status": "open"}, sess: principal}), http.StatusForbidden, "forbidden")
	expectStatus(t, h.do(req{method: http.MethodPost, path: inboxPath(id, "notes"), body: map[string]any{"body": "x"}, sess: principal}), http.StatusForbidden, "forbidden")

	// Only people who can see the inbox can be given an enquiry.
	expectStatus(t, h.do(req{method: http.MethodPost, path: inboxPath(id, "assign"), body: map[string]any{"assignee_id": editorID.String()}, sess: registrar}), http.StatusUnprocessableEntity, "not_assignable")
	people := itemsOf(h.call(req{method: http.MethodGet, path: inboxPath("assignees"), sess: registrar}, http.StatusOK))
	for _, p := range people {
		if p["id"] == editorID.String() {
			t.Fatal("a content editor is offered as an assignee")
		}
	}
	expectStatus(t, h.do(req{method: http.MethodPost, path: inboxPath(id, "assign"), body: map[string]any{"assignee_id": nil}, sess: registrar}), http.StatusConflict, "unchanged")

	// Status rules.
	expectStatus(t, h.do(req{method: http.MethodPost, path: inboxPath(id, "status"), body: map[string]any{"status": "new"}, sess: registrar}), http.StatusUnprocessableEntity, "invalid_status")
	expectStatus(t, h.do(req{method: http.MethodPost, path: inboxPath(id, "status"), body: map[string]any{"status": "done"}, sess: registrar}), http.StatusUnprocessableEntity, "invalid_status")
	h.call(req{method: http.MethodPost, path: inboxPath(id, "status"), body: map[string]any{"status": "open"}, sess: registrar}, http.StatusOK)
	expectStatus(t, h.do(req{method: http.MethodPost, path: inboxPath(id, "status"), body: map[string]any{"status": "open"}, sess: registrar}), http.StatusConflict, "unchanged")
	expectStatus(t, h.do(req{method: http.MethodPost, path: inboxPath(id, "notes"), body: map[string]any{"body": "   "}, sess: registrar}), http.StatusUnprocessableEntity, "note_empty")
	expectStatus(t, h.do(req{method: http.MethodGet, path: inboxPath("00000000-0000-0000-0000-000000000000"), sess: registrar}), http.StatusNotFound, "not_found")

	// Filters.
	if got := itemsOf(h.call(req{method: http.MethodGet, path: inboxPath() + "?status=new", sess: registrar}, http.StatusOK)); len(got) != 1 || got[0]["query_type"] != "alumni" {
		t.Fatalf("status=new: %v", got)
	}
	if got := itemsOf(h.call(req{method: http.MethodGet, path: inboxPath() + "?office=general&assigned=none", sess: registrar}, http.StatusOK)); len(got) != 1 {
		t.Fatalf("general unassigned: %v", got)
	}
	if got := itemsOf(h.call(req{method: http.MethodGet, path: inboxPath() + "?q=amina", sess: registrar}, http.StatusOK)); len(got) != 1 || got[0]["full_name"] != "Amina Test" {
		t.Fatalf("search: %v", got)
	}
	if got := itemsOf(h.call(req{method: http.MethodGet, path: inboxPath() + "?q=transcript", sess: registrar}, http.StatusOK)); len(got) != 1 {
		t.Fatalf("search in message: %v", got)
	}
	expectStatus(t, h.do(req{method: http.MethodGet, path: inboxPath() + "?office=library", sess: registrar}), http.StatusBadRequest, "invalid_request")
	expectStatus(t, h.do(req{method: http.MethodGet, path: inboxPath() + "?status=archived", sess: registrar}), http.StatusBadRequest, "invalid_request")
	page := h.call(req{method: http.MethodGet, path: inboxPath() + "?page=2", sess: registrar}, http.StatusOK)
	if len(page["items"].([]any)) != 0 || page["total"] != float64(2) || page["total_pages"] != float64(1) {
		t.Fatalf("page 2: %v", page)
	}
}

// Migration 00009 gives Finance Officers the inbox (owner, 2026-10-23).
func TestFinanceOfficerCanUseTheInbox(t *testing.T) {
	h := newHarness(t)
	var n int
	if err := h.pool.QueryRow(context.Background(), `
		SELECT count(*) FROM role_permissions rp JOIN roles r ON r.id = rp.role_id
		WHERE r.key = 'finance_officer' AND rp.permission_key IN ('enquiries.view', 'enquiries.manage')`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("finance officer has %d of the 2 enquiry permissions", n)
	}
}
