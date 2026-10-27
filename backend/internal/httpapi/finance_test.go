package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type financeSummary struct {
	BalanceCents int64   `json:"balance_cents"`
	PaidCents    int64   `json:"paid_cents"`
	PendingCents int64   `json:"pending_cents"`
	OverdueCents int64   `json:"overdue_cents"`
	NextDueOn    *string `json:"next_due_on"`
	NextDueCents int64   `json:"next_due_cents"`
}

type statementEntry struct {
	Kind         string
	Reference    string
	DebitCents   int64 `json:"debit_cents"`
	CreditCents  int64 `json:"credit_cents"`
	BalanceCents int64 `json:"balance_cents"`
}

type accountView struct {
	Summary   financeSummary
	Statement []statementEntry
	Invoices  []struct {
		ID     uuid.UUID
		Status string
		Number *string
	}
	Payments []struct {
		ID            uuid.UUID
		Status        string
		ReceiptNumber *string `json:"receipt_number"`
	}
}

type paymentView struct {
	ID            uuid.UUID
	Status        string
	Reference     string
	ReceiptNumber *string `json:"receipt_number"`
}

type invoiceView struct {
	ID          uuid.UUID
	Status      string
	Number      *string
	TotalCents  int64 `json:"total_cents"`
	Lines       []struct{ Label string }
	Instalments []struct {
		Percent     int
		AmountCents *int64  `json:"amount_cents"`
		DueOn       *string `json:"due_on"`
	}
}

var keySeq int

func newKey() string { keySeq++; return fmt.Sprintf("test-key-%06d", keySeq) }

// keyedResponse is a response with its headers, for replay checks.
type keyedResponse struct{ *httptest.ResponseRecorder }

func (r keyedResponse) expect(t *testing.T, status int, code string) {
	t.Helper()
	expectStatus(t, r.ResponseRecorder, status, code)
}

func (r keyedResponse) decode(t *testing.T, out any) {
	t.Helper()
	if err := json.Unmarshal(r.Body.Bytes(), out); err != nil {
		t.Fatalf("decode %s: %v", r.Body.String(), err)
	}
}

// keyed sends a request with an Idempotency-Key.
func (h *harness) keyed(s *session, method, path, key string, body any) keyedResponse {
	h.t.Helper()
	return keyedResponse{h.do(req{method: method, path: "/api/v1" + path, body: body, sess: s, headers: map[string]string{"Idempotency-Key": key}})}
}

func (h *harness) financeOfficer() *session {
	h.t.Helper()
	h.createUser("finance@example.invalid", "finance_officer")
	return h.mustLogin("finance@example.invalid")
}

// feeStructure sets up registration KES 5,000 once and tuition KES 30,000
// per term, half due on issue and half after 30 days.
func (h *harness) feeStructure(fin *session, programme uuid.UUID, intake *uuid.UUID) uuid.UUID {
	h.t.Helper()
	var s newID
	h.api(fin, http.MethodPost, "/staff/finance/fee-structures", map[string]any{"programme_id": programme, "intake_id": intake}, http.StatusCreated, &s)
	h.api(fin, http.MethodPut, "/staff/finance/fee-structures/"+s.ID.String(), map[string]any{
		"lines": []map[string]any{
			{"label": "Registration (Sample)", "category": "registration", "charge": "once", "amount_cents": 500_000},
			{"label": "Tuition (Sample)", "category": "tuition", "charge": "per_term", "amount_cents": 3_000_000},
		},
		"instalments": []map[string]any{{"percent": 50, "due_days": 0}, {"percent": 50, "due_days": 30}},
	}, http.StatusOK, nil)
	return s.ID
}

// independentBalance sums a student's records directly, for AC-10.
func (h *harness) independentBalance(student uuid.UUID) int64 {
	h.t.Helper()
	var b int64
	if err := h.pool.QueryRow(context.Background(), `SELECT
		  coalesce((SELECT sum(l.amount_cents) FROM invoice_lines l JOIN invoices i ON i.id = l.invoice_id WHERE i.student_id = $1 AND i.status = 'issued'), 0)
		+ coalesce((SELECT sum(amount_cents) FROM adjustments WHERE student_id = $1 AND kind = 'charge'), 0)
		- coalesce((SELECT sum(amount_cents) FROM payments WHERE student_id = $1 AND status = 'confirmed'), 0)
		- coalesce((SELECT sum(amount_cents) FROM adjustments WHERE student_id = $1 AND kind = 'credit'), 0)`, student).Scan(&b); err != nil {
		h.t.Fatal(err)
	}
	return b
}

// reconciles checks AC-10 for one student: the statement's closing
// balance, the summary and the records summed separately all agree, and
// each running balance follows from the row before.
func (h *harness) reconciles(fin *session, student uuid.UUID) accountView {
	h.t.Helper()
	var acc accountView
	h.api(fin, http.MethodGet, "/staff/finance/students/"+student.String(), nil, http.StatusOK, &acc)
	var running int64
	for i, e := range acc.Statement {
		running += e.DebitCents - e.CreditCents
		if e.BalanceCents != running {
			h.t.Fatalf("statement row %d balance %d, want %d", i, e.BalanceCents, running)
		}
	}
	if want := h.independentBalance(student); acc.Summary.BalanceCents != want || running != want {
		h.t.Fatalf("balance %d, statement %d, records %d", acc.Summary.BalanceCents, running, want)
	}
	return acc
}

var (
	invoiceNumberPattern = regexp.MustCompile(`^DTC/INV/\d{4}/\d{5}$`)
	receiptNumberPattern = regexp.MustCompile(`^DTC/RCT/\d{4}/\d{5}$`)
)

// AC-10 and AC-11 end to end: bill a class, issue, record and confirm
// payments, adjust, and the statement reconciles at every step.
func TestFinanceFlow(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	fin := h.financeOfficer()
	alice, bob := a.students[0], a.students[1]
	aliceSession := h.studentLogin(alice)

	// Billing needs a fee structure first.
	if got := h.code(fin, http.MethodPost, "/staff/finance/billing", map[string]any{"cohort_id": a.cohort, "term_id": a.term}); got != "422 validation_failed" {
		t.Fatalf("billing without a structure: %s", got)
	}
	h.feeStructure(fin, a.programme, nil)
	var run struct{ Created, Skipped int }
	h.api(fin, http.MethodPost, "/staff/finance/billing", map[string]any{"cohort_id": a.cohort, "term_id": a.term}, http.StatusOK, &run)
	if run.Created != 3 || run.Skipped != 0 {
		t.Fatalf("billing run: %+v", run)
	}
	// Running it again bills nobody twice.
	h.api(fin, http.MethodPost, "/staff/finance/billing", map[string]any{"cohort_id": a.cohort, "term_id": a.term}, http.StatusOK, &run)
	if run.Created != 0 || run.Skipped != 3 {
		t.Fatalf("second billing run: %+v", run)
	}

	var drafts struct {
		Items []struct {
			ID         uuid.UUID
			TotalCents int64 `json:"total_cents"`
		}
		Total int64
	}
	h.api(fin, http.MethodGet, "/staff/finance/invoices?status=draft", nil, http.StatusOK, &drafts)
	if drafts.Total != 3 || drafts.Items[0].TotalCents != 3_000_000 {
		t.Fatalf("drafts: %+v", drafts)
	}
	// A draft is invisible to the student and adds nothing to the balance.
	var mine struct {
		Summary  financeSummary
		Invoices []struct{ ID uuid.UUID }
	}
	h.api(aliceSession, http.MethodGet, "/student/fees", nil, http.StatusOK, &mine)
	if len(mine.Invoices) != 0 || mine.Summary.BalanceCents != 0 {
		t.Fatalf("student sees a draft: %+v", mine)
	}

	ids := []uuid.UUID{drafts.Items[0].ID, drafts.Items[1].ID, drafts.Items[2].ID}
	var issued struct {
		Issued  int
		Numbers []string
	}
	h.api(fin, http.MethodPost, "/staff/finance/invoices/issue", map[string]any{"ids": ids}, http.StatusOK, &issued)
	if issued.Issued != 3 {
		t.Fatalf("issued: %+v", issued)
	}
	for i, n := range issued.Numbers {
		if !invoiceNumberPattern.MatchString(n) || n[len(n)-5:] != fmt.Sprintf("%05d", i+1) {
			t.Fatalf("invoice numbers not gapless: %v", issued.Numbers)
		}
	}
	// Each student's own invoice, from their account.
	acc := h.reconciles(fin, alice)
	aliceInvoice := acc.Invoices[0].ID
	bobInvoice := h.reconciles(fin, bob).Invoices[0].ID
	var inv invoiceView
	h.api(fin, http.MethodGet, "/staff/finance/invoices/"+aliceInvoice.String(), nil, http.StatusOK, &inv)
	if inv.Status != "issued" || len(inv.Instalments) != 2 || *inv.Instalments[0].AmountCents != 1_500_000 || *inv.Instalments[1].AmountCents != 1_500_000 {
		t.Fatalf("issued invoice: %+v", inv)
	}
	// An issued invoice never changes.
	if got := h.code(fin, http.MethodPut, "/staff/finance/invoices/"+aliceInvoice.String(), map[string]any{"lines": []map[string]any{}}); got != "409 not_draft" {
		t.Fatalf("editing an issued invoice: %s", got)
	}
	if got := h.code(fin, http.MethodPost, "/staff/finance/invoices/"+aliceInvoice.String()+"/void", map[string]any{"reason": "Changed my mind"}); got != "409 not_draft" {
		t.Fatalf("voiding an issued invoice: %s", got)
	}
	acc = h.reconciles(fin, alice)
	if acc.Summary.BalanceCents != 3_000_000 || acc.Summary.OverdueCents != 1_500_000 || acc.Summary.NextDueCents != 1_500_000 {
		t.Fatalf("after issue: %+v", acc.Summary)
	}

	// AC-11: an M-Pesa payment recorded with its code is pending, and does
	// not count until finance confirms it.
	key := newKey()
	body := map[string]any{"student_id": alice, "method": "mpesa", "amount_cents": 1_000_000, "paid_on": time.Now().Format(time.DateOnly), "reference": "qk12 ab34cd"}
	first := h.keyed(fin, http.MethodPost, "/staff/finance/payments", key, body)
	first.expect(t, http.StatusOK, "")
	var pm paymentView
	first.decode(t, &pm)
	if pm.Status != "pending" || pm.Reference != "QK12AB34CD" || pm.ReceiptNumber != nil {
		t.Fatalf("recorded M-Pesa payment: %+v", pm)
	}
	// NN-04: a retry with the same key gets the same answer and records nothing more.
	again := h.keyed(fin, http.MethodPost, "/staff/finance/payments", key, body)
	again.expect(t, http.StatusOK, "")
	if again.Header().Get("Idempotent-Replayed") != "true" || !bytes.Equal(again.Body.Bytes(), first.Body.Bytes()) {
		t.Fatalf("replay differs:\n%s\n%s", first.Body, again.Body)
	}
	body["amount_cents"] = 2_000_000
	h.keyed(fin, http.MethodPost, "/staff/finance/payments", key, body).expect(t, http.StatusUnprocessableEntity, "idempotency_key_reused")
	h.keyed(fin, http.MethodPost, "/staff/finance/payments", newKey(), body).expect(t, http.StatusUnprocessableEntity, "validation_failed") // same M-Pesa code
	h.keyed(fin, http.MethodPost, "/staff/finance/payments", "", body).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	acc = h.reconciles(fin, alice)
	if acc.Summary.BalanceCents != 3_000_000 || acc.Summary.PendingCents != 1_000_000 || len(acc.Payments) != 1 {
		t.Fatalf("pending payment counted: %+v", acc.Summary)
	}

	confirmKey := newKey()
	h.keyed(fin, http.MethodPost, "/staff/finance/payments/"+pm.ID.String()+"/confirm", confirmKey, map[string]any{"note": "On the M-Pesa statement."}).expect(t, http.StatusOK, "")
	h.keyed(fin, http.MethodPost, "/staff/finance/payments/"+pm.ID.String()+"/confirm", confirmKey, map[string]any{"note": "On the M-Pesa statement."}).expect(t, http.StatusOK, "")
	h.keyed(fin, http.MethodPost, "/staff/finance/payments/"+pm.ID.String()+"/confirm", newKey(), map[string]any{}).expect(t, http.StatusConflict, "already_decided")
	h.keyed(fin, http.MethodPost, "/staff/finance/payments/"+pm.ID.String()+"/reject", newKey(), map[string]any{"reason": "Not on the statement."}).expect(t, http.StatusConflict, "already_decided")
	h.api(fin, http.MethodGet, "/staff/finance/payments/"+pm.ID.String(), nil, http.StatusOK, &pm)
	if pm.Status != "confirmed" || pm.ReceiptNumber == nil || !receiptNumberPattern.MatchString(*pm.ReceiptNumber) {
		t.Fatalf("confirmed payment: %+v", pm)
	}
	acc = h.reconciles(fin, alice)
	if acc.Summary.BalanceCents != 2_000_000 || acc.Summary.PendingCents != 0 || acc.Summary.OverdueCents != 500_000 {
		t.Fatalf("after confirmation: %+v", acc.Summary)
	}

	// Cash taken at the office is receipted at once.
	var cash paymentView
	w := h.keyed(fin, http.MethodPost, "/staff/finance/payments", newKey(), map[string]any{"student_id": alice, "method": "cash", "amount_cents": 500_000, "paid_on": time.Now().Format(time.DateOnly)})
	w.expect(t, http.StatusOK, "")
	w.decode(t, &cash)
	if cash.Status != "confirmed" || cash.ReceiptNumber == nil || (*cash.ReceiptNumber)[len(*cash.ReceiptNumber)-5:] != "00002" {
		t.Fatalf("cash payment: %+v", cash)
	}

	// A bank payment that is not on the statement is rejected and never counts.
	var bank paymentView
	w = h.keyed(fin, http.MethodPost, "/staff/finance/payments", newKey(), map[string]any{"student_id": alice, "method": "bank", "amount_cents": 900_000, "paid_on": time.Now().Format(time.DateOnly), "reference": "SLIP 0042"})
	w.expect(t, http.StatusOK, "")
	w.decode(t, &bank)
	h.keyed(fin, http.MethodPost, "/staff/finance/payments/"+bank.ID.String()+"/reject", newKey(), map[string]any{"reason": "no"}).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	h.keyed(fin, http.MethodPost, "/staff/finance/payments/"+bank.ID.String()+"/reject", newKey(), map[string]any{"reason": "Not on the bank statement."}).expect(t, http.StatusOK, "")
	acc = h.reconciles(fin, alice)
	if acc.Summary.BalanceCents != 1_500_000 || acc.Summary.OverdueCents != 0 {
		t.Fatalf("after cash and a rejected transfer: %+v", acc.Summary)
	}

	// Corrections are compensating entries with a reason.
	h.keyed(fin, http.MethodPost, "/staff/finance/students/"+alice.String()+"/adjustments", newKey(), map[string]any{"kind": "credit", "amount_cents": 200_000}).
		expect(t, http.StatusUnprocessableEntity, "validation_failed")
	h.keyed(fin, http.MethodPost, "/staff/finance/students/"+alice.String()+"/adjustments", newKey(), map[string]any{
		"kind": "credit", "amount_cents": 200_000, "reason": "Bursary (Sample)", "invoice_id": aliceInvoice}).expect(t, http.StatusOK, "")
	h.keyed(fin, http.MethodPost, "/staff/finance/students/"+alice.String()+"/adjustments", newKey(), map[string]any{
		"kind": "charge", "amount_cents": 500_000, "reason": "Cash counted twice: reversed", "payment_id": cash.ID}).expect(t, http.StatusOK, "")
	h.keyed(fin, http.MethodPost, "/staff/finance/students/"+alice.String()+"/adjustments", newKey(), map[string]any{
		"kind": "credit", "amount_cents": 1, "reason": "Another student's invoice", "invoice_id": bobInvoice}).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	acc = h.reconciles(fin, alice)
	if acc.Summary.BalanceCents != 1_800_000 || len(acc.Statement) != 5 {
		t.Fatalf("after adjustments: %+v, %d rows", acc.Summary, len(acc.Statement))
	}

	// The student sees the same balance, issued invoices only, and not the rejected transfer.
	var fees struct {
		Summary  financeSummary
		Invoices []struct {
			ID    uuid.UUID
			Lines []struct{ Label string }
		}
		Payments  []struct{ Status string }
		Statement []statementEntry
		Structure *struct{ Lines []struct{ Label string } }
	}
	h.api(aliceSession, http.MethodGet, "/student/fees", nil, http.StatusOK, &fees)
	if fees.Summary.BalanceCents != 1_800_000 || len(fees.Invoices) != 1 || len(fees.Invoices[0].Lines) != 1 || len(fees.Payments) != 2 ||
		len(fees.Statement) != 5 || fees.Structure == nil || len(fees.Structure.Lines) != 2 {
		t.Fatalf("student's fees: %+v", fees)
	}

	// Documents: own receipt, invoice and statement; nobody else's.
	pdf := func(s *session, path string, want int) {
		t.Helper()
		w := h.do(req{method: http.MethodGet, path: "/api/v1" + path, sess: s})
		if w.Code != want {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if want == http.StatusOK && (w.Header().Get("Content-Type") != "application/pdf" || !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF"))) {
			t.Fatalf("%s is not a PDF", path)
		}
	}
	pdf(aliceSession, "/student/fees/payments/"+pm.ID.String()+"/receipt", http.StatusOK)
	pdf(aliceSession, "/student/fees/payments/"+bank.ID.String()+"/receipt", http.StatusNotFound) // rejected: no receipt
	pdf(aliceSession, "/student/fees/invoices/"+aliceInvoice.String(), http.StatusOK)
	pdf(aliceSession, "/student/fees/invoices/"+bobInvoice.String(), http.StatusNotFound) // Bob's
	pdf(aliceSession, "/student/fees/statement", http.StatusOK)
	pdf(fin, "/staff/finance/payments/"+cash.ID.String()+"/receipt", http.StatusOK)
	pdf(fin, "/staff/finance/students/"+bob.String()+"/statement", http.StatusOK)
	pdf(fin, "/staff/finance/invoices/"+bobInvoice.String()+"/pdf", http.StatusOK)

	// Arrears: everyone still owes; Alice's overdue part is the reversed cash.
	var arrears struct {
		Items []struct {
			Student      struct{ ID uuid.UUID }
			OverdueCents int64 `json:"overdue_cents"`
		}
	}
	h.api(fin, http.MethodGet, "/staff/finance/arrears", nil, http.StatusOK, &arrears)
	if len(arrears.Items) != 3 {
		t.Fatalf("arrears: %+v", arrears.Items)
	}
	var summary struct {
		NetOutstandingCents int64 `json:"net_outstanding_cents"`
	}
	h.api(fin, http.MethodGet, "/staff/finance/summary", nil, http.StatusOK, &summary)
	if want := h.independentBalance(alice) + h.independentBalance(bob) + h.independentBalance(a.students[2]); summary.NetOutstandingCents != want {
		t.Fatalf("outstanding %d, want %d", summary.NetOutstandingCents, want)
	}

	for action, want := range map[string]int{"finance.invoice_issued": 3, "finance.payment_recorded": 3, "finance.payment_confirmed": 2,
		"finance.payment_rejected": 1, "finance.adjustment": 2, "finance.billing_run": 2} {
		if n := h.auditCount(action); n != want {
			t.Errorf("%s audited %d times, want %d", action, n, want)
		}
	}
}

// NN-05: the database itself refuses to change completed financial records.
func TestFinanceRecordsCannotBeEdited(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	fin := h.financeOfficer()
	h.feeStructure(fin, a.programme, nil)
	var run struct {
		InvoiceIDs []uuid.UUID `json:"invoice_ids"`
	}
	h.api(fin, http.MethodPost, "/staff/finance/billing", map[string]any{"cohort_id": a.cohort, "term_id": a.term}, http.StatusOK, &run)
	h.api(fin, http.MethodPost, "/staff/finance/invoices/"+run.InvoiceIDs[0].String()+"/issue", map[string]any{}, http.StatusOK, nil)
	h.keyed(fin, http.MethodPost, "/staff/finance/payments", newKey(), map[string]any{"student_id": a.students[0], "method": "cash", "amount_cents": 100, "paid_on": time.Now().Format(time.DateOnly)}).
		expect(t, http.StatusOK, "")
	pending := h.keyed(fin, http.MethodPost, "/staff/finance/payments", newKey(), map[string]any{"student_id": a.students[0], "method": "bank", "amount_cents": 100, "paid_on": time.Now().Format(time.DateOnly), "reference": "X1"})
	pending.expect(t, http.StatusOK, "")
	h.keyed(fin, http.MethodPost, "/staff/finance/students/"+a.students[0].String()+"/adjustments", newKey(), map[string]any{"kind": "credit", "amount_cents": 100, "reason": "Test credit"}).
		expect(t, http.StatusOK, "")

	for _, sql := range []string{
		`UPDATE invoices SET note = 'changed' WHERE status = 'issued'`,
		`DELETE FROM invoices`,
		`UPDATE invoice_lines SET amount_cents = 1 WHERE invoice_id = '` + run.InvoiceIDs[0].String() + `'`,
		`DELETE FROM invoice_lines WHERE invoice_id = '` + run.InvoiceIDs[0].String() + `'`,
		`INSERT INTO invoice_lines (invoice_id, label, category, amount_cents) VALUES ('` + run.InvoiceIDs[0].String() + `', 'Extra', 'other', 1)`,
		`UPDATE invoice_instalments SET amount_cents = 1 WHERE invoice_id = '` + run.InvoiceIDs[0].String() + `'`,
		`UPDATE payments SET amount_cents = 1 WHERE status = 'confirmed'`,
		`UPDATE payments SET amount_cents = 1 WHERE status = 'pending'`, // what was received never changes
		`DELETE FROM payments`,
		`UPDATE receipts SET number = 'X'`,
		`DELETE FROM receipts`,
		`UPDATE adjustments SET amount_cents = 1`,
		`DELETE FROM adjustments`,
		// A payment cannot be marked paid without a confirmer (AC-11).
		`UPDATE payments SET status = 'confirmed', decided_at = now() WHERE status = 'pending'`,
		`INSERT INTO payments (student_id, method, amount_cents, paid_on, reference, status, decided_at, recorded_by)
		   SELECT student_id, 'mpesa', 5, current_date, 'AB12CD34EF', 'confirmed', now(), recorded_by FROM payments LIMIT 1`,
	} {
		if _, err := h.pool.Exec(context.Background(), sql); err == nil {
			t.Errorf("allowed: %s", sql)
		}
	}
}

func TestFinancePermissions(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	fin := h.financeOfficer()
	h.createUser("principal@example.invalid", "principal")
	principal := h.mustLogin("principal@example.invalid")
	student := h.studentLogin(a.students[0])
	h.feeStructure(fin, a.programme, nil)

	// A clerk who may record payments but not confirm them (custom role).
	h.exec(`INSERT INTO roles (key, name, description) VALUES ('cashier', 'Cashier', 'Records payments')`)
	h.exec(`INSERT INTO role_permissions (role_id, permission_key) SELECT id, k FROM roles, unnest(ARRAY['finance.view', 'finance.payment.record']) k WHERE key = 'cashier'`)
	h.createUser("cashier@example.invalid", "cashier")
	cashier := h.mustLogin("cashier@example.invalid")

	// The registrar has no finance access; the principal can only look.
	if got := h.code(a.registrar, http.MethodGet, "/staff/finance/summary", nil); got != "403 forbidden" {
		t.Fatalf("registrar: %s", got)
	}
	h.api(principal, http.MethodGet, "/staff/finance/summary", nil, http.StatusOK, nil)
	h.api(principal, http.MethodGet, "/staff/finance/students/"+a.students[0].String(), nil, http.StatusOK, nil)
	h.keyed(principal, http.MethodPost, "/staff/finance/payments", newKey(), map[string]any{"student_id": a.students[0], "method": "cash", "amount_cents": 100, "paid_on": time.Now().Format(time.DateOnly)}).
		expect(t, http.StatusForbidden, "forbidden")
	if got := h.code(principal, http.MethodPost, "/staff/finance/billing", map[string]any{"cohort_id": a.cohort, "term_id": a.term}); got != "403 forbidden" {
		t.Fatalf("principal billing: %s", got)
	}

	// Cash recorded by someone who cannot confirm waits like any other payment.
	var pm paymentView
	w := h.keyed(cashier, http.MethodPost, "/staff/finance/payments", newKey(), map[string]any{"student_id": a.students[0], "method": "cash", "amount_cents": 100, "paid_on": time.Now().Format(time.DateOnly)})
	w.expect(t, http.StatusOK, "")
	w.decode(t, &pm)
	if pm.Status != "pending" {
		t.Fatalf("cashier's cash: %+v", pm)
	}
	h.keyed(cashier, http.MethodPost, "/staff/finance/payments/"+pm.ID.String()+"/confirm", newKey(), map[string]any{}).expect(t, http.StatusForbidden, "forbidden")
	h.keyed(cashier, http.MethodPost, "/staff/finance/students/"+a.students[0].String()+"/adjustments", newKey(), map[string]any{"kind": "credit", "amount_cents": 100, "reason": "Just because"}).
		expect(t, http.StatusForbidden, "forbidden")
	// The finance officer confirms it; the payment counts only then.
	h.keyed(fin, http.MethodPost, "/staff/finance/payments/"+pm.ID.String()+"/confirm", newKey(), map[string]any{}).expect(t, http.StatusOK, "")

	// Students reach only their own fees, never the staff side.
	if got := h.code(student, http.MethodGet, "/staff/finance/students/"+a.students[0].String(), nil); got != "403 forbidden" {
		t.Fatalf("student on staff route: %s", got)
	}
	if got := h.code(fin, http.MethodGet, "/student/fees", nil); got != "403 forbidden" {
		t.Fatalf("staff on student route: %s", got)
	}
	// Validation: amounts are whole cents in range, dates not in the future, codes the right shape.
	for _, body := range []map[string]any{
		{"student_id": a.students[0], "method": "cash", "amount_cents": 0, "paid_on": time.Now().Format(time.DateOnly)},
		{"student_id": a.students[0], "method": "cash", "amount_cents": 100, "paid_on": time.Now().AddDate(0, 0, 2).Format(time.DateOnly)},
		{"student_id": a.students[0], "method": "mpesa", "amount_cents": 100, "paid_on": time.Now().Format(time.DateOnly), "reference": "SHORT"},
		{"student_id": a.students[0], "method": "bank", "amount_cents": 100, "paid_on": time.Now().Format(time.DateOnly)},
		{"student_id": a.students[0], "method": "sponsor", "amount_cents": 100, "paid_on": time.Now().Format(time.DateOnly)},
		{"student_id": a.students[0], "method": "cheque", "amount_cents": 100, "paid_on": time.Now().Format(time.DateOnly)},
		{"student_id": uuid.New(), "method": "cash", "amount_cents": 100, "paid_on": time.Now().Format(time.DateOnly)},
	} {
		h.keyed(fin, http.MethodPost, "/staff/finance/payments", newKey(), body).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	}
	// Floats are not money: a fractional amount is not accepted.
	h.keyed(fin, http.MethodPost, "/staff/finance/payments", newKey(), map[string]any{"student_id": a.students[0], "method": "cash", "amount_cents": 10.5, "paid_on": time.Now().Format(time.DateOnly)}).
		expect(t, http.StatusBadRequest, "invalid_request")
}

// AC-10 under many operations: after a random mix of invoices, payments
// and adjustments, every statement reconciles and the college total is the
// sum of the students' balances.
func TestStatementsReconcile(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	fin := h.financeOfficer()
	h.feeStructure(fin, a.programme, nil)
	rng := rand.New(rand.NewPCG(14, 10))
	var pending []uuid.UUID
	today := time.Now().Format(time.DateOnly)
	for i := range 60 {
		st := a.students[rng.IntN(len(a.students))]
		switch op := rng.IntN(6); op {
		case 0: // an invoice by hand, issued
			var inv invoiceView
			h.api(fin, http.MethodPost, "/staff/finance/students/"+st.String()+"/invoices", map[string]any{"kind": "other"}, http.StatusCreated, &inv)
			h.api(fin, http.MethodPut, "/staff/finance/invoices/"+inv.ID.String(), map[string]any{"lines": []map[string]any{
				{"label": "Item", "category": "other", "amount_cents": 1 + rng.Int64N(5_000_000)},
				{"label": "Item 2", "category": "assessment", "amount_cents": 1 + rng.Int64N(99)},
			}, "instalments": []map[string]any{{"percent": 33, "due_days": 0}, {"percent": 67, "due_days": 1 + rng.IntN(90)}}}, http.StatusOK, nil)
			h.api(fin, http.MethodPost, "/staff/finance/invoices/"+inv.ID.String()+"/issue", map[string]any{}, http.StatusOK, nil)
		case 1, 2: // a payment, pending or receipted at once
			method := []string{"cash", "bank", "mpesa", "sponsor"}[rng.IntN(4)]
			body := map[string]any{"student_id": st, "method": method, "amount_cents": 1 + rng.Int64N(2_000_000), "paid_on": today,
				"reference": fmt.Sprintf("RF%08d", i), "payer_name": "Sponsor (Sample)"}
			var pm paymentView
			w := h.keyed(fin, http.MethodPost, "/staff/finance/payments", newKey(), body)
			w.expect(t, http.StatusOK, "")
			w.decode(t, &pm)
			if pm.Status == "pending" {
				pending = append(pending, pm.ID)
			}
		case 3: // decide a pending payment
			if len(pending) == 0 {
				continue
			}
			id := pending[0]
			pending = pending[1:]
			action, body := "confirm", map[string]any{}
			if rng.IntN(3) == 0 {
				action, body = "reject", map[string]any{"reason": "Not on the statement."}
			}
			h.keyed(fin, http.MethodPost, "/staff/finance/payments/"+id.String()+"/"+action, newKey(), body).expect(t, http.StatusOK, "")
		case 4: // an adjustment either way
			kind := []string{"charge", "credit"}[rng.IntN(2)]
			h.keyed(fin, http.MethodPost, "/staff/finance/students/"+st.String()+"/adjustments", newKey(), map[string]any{"kind": kind, "amount_cents": 1 + rng.Int64N(300_000), "reason": "Random correction"}).
				expect(t, http.StatusOK, "")
		case 5: // a draft that is voided changes nothing
			var inv invoiceView
			h.api(fin, http.MethodPost, "/staff/finance/students/"+st.String()+"/invoices", map[string]any{"kind": "other"}, http.StatusCreated, &inv)
			h.api(fin, http.MethodPut, "/staff/finance/invoices/"+inv.ID.String(), map[string]any{"lines": []map[string]any{{"label": "Never billed", "category": "other", "amount_cents": 999}}}, http.StatusOK, nil)
			h.api(fin, http.MethodPost, "/staff/finance/invoices/"+inv.ID.String()+"/void", map[string]any{"reason": "Raised by mistake"}, http.StatusOK, nil)
		}
		if i%10 == 9 {
			for _, s := range a.students {
				h.reconciles(fin, s)
			}
		}
	}
	var total int64
	for _, s := range a.students {
		total += h.reconciles(fin, s).Summary.BalanceCents
	}
	var summary struct {
		NetOutstandingCents int64 `json:"net_outstanding_cents"`
	}
	h.api(fin, http.MethodGet, "/staff/finance/summary", nil, http.StatusOK, &summary)
	if summary.NetOutstandingCents != total {
		t.Fatalf("college outstanding %d, students' balances add up to %d", summary.NetOutstandingCents, total)
	}
}

// Concurrent confirmations: one wins, and receipt numbers have no gaps.
func TestConcurrentPayments(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	fin := h.financeOfficer()
	var pm paymentView
	w := h.keyed(fin, http.MethodPost, "/staff/finance/payments", newKey(), map[string]any{"student_id": a.students[0], "method": "mpesa", "amount_cents": 100, "paid_on": time.Now().Format(time.DateOnly), "reference": "AB12CD34EF"})
	w.expect(t, http.StatusOK, "")
	w.decode(t, &pm)

	keys := make([]string, 8)
	for i := range keys {
		keys[i] = newKey()
	}
	codes := make([]int, len(keys))
	var wg sync.WaitGroup
	for i, k := range keys {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = h.do(req{method: http.MethodPost, path: "/api/v1/staff/finance/payments/" + pm.ID.String() + "/confirm", body: map[string]any{}, sess: fin,
				headers: map[string]string{"Idempotency-Key": k}}).Code
		}()
	}
	wg.Wait()
	ok := 0
	for _, c := range codes {
		if c == http.StatusOK {
			ok++
		} else if c != http.StatusConflict {
			t.Fatalf("confirm answered %d", c)
		}
	}
	if ok != 1 {
		t.Fatalf("%d confirmations succeeded, want 1", ok)
	}

	// Cash in parallel: every receipt gets the next number.
	var cashKeys []string
	for range 10 {
		cashKeys = append(cashKeys, newKey())
	}
	for _, k := range cashKeys {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.do(req{method: http.MethodPost, path: "/api/v1/staff/finance/payments", sess: fin, headers: map[string]string{"Idempotency-Key": k},
				body: map[string]any{"student_id": a.students[1], "method": "cash", "amount_cents": 100, "paid_on": time.Now().Format(time.DateOnly)}})
		}()
	}
	wg.Wait()
	var n, maxSeq int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*), max(right(number, 5)::int) FROM receipts`).Scan(&n, &maxSeq); err != nil {
		t.Fatal(err)
	}
	if n != 11 || maxSeq != 11 {
		t.Fatalf("%d receipts, highest number %d: want 11 and 11 (no gaps)", n, maxSeq)
	}
}

// Enrolment opens the first invoice from the fee structure: every line
// once, with the structure's instalments.
func TestEnrolmentFillsFirstInvoice(t *testing.T) {
	h := newHarness(t)
	w := h.actors()
	fin := h.financeOfficer()
	app, id := h.submitted("applicant301@example.invalid")
	var programme uuid.UUID
	var intake *uuid.UUID
	if err := h.pool.QueryRow(context.Background(), `SELECT programme_id, intake_id FROM applications WHERE id = $1`, id).Scan(&programme, &intake); err != nil {
		t.Fatal(err)
	}
	h.feeStructure(fin, programme, intake)
	h.staffAct(w.officer, id, "assign", map[string]any{"assignee_id": w.officerID}).expect(t, http.StatusOK, "")
	h.staffAct(w.officer, id, "offer", nil).expect(t, http.StatusOK, "")
	h.applicantAct(app.session, id, "accept-offer", map[string]any{}).expect(t, http.StatusOK, "")
	h.staffAct(w.registrar, id, "enrol", nil).expect(t, http.StatusOK, "")

	var invoice uuid.UUID
	if err := h.pool.QueryRow(context.Background(), `SELECT i.id FROM invoices i JOIN students s ON s.id = i.student_id WHERE s.application_id = $1`, id).Scan(&invoice); err != nil {
		t.Fatal(err)
	}
	var inv invoiceView
	h.api(fin, http.MethodGet, "/staff/finance/invoices/"+invoice.String(), nil, http.StatusOK, &inv)
	if inv.Status != "draft" || inv.TotalCents != 3_500_000 || len(inv.Lines) != 2 || len(inv.Instalments) != 2 || inv.Instalments[0].AmountCents != nil {
		t.Fatalf("first invoice: %+v", inv)
	}
	// Fill is for term and enrolment invoices; issuing fixes the plan.
	h.api(fin, http.MethodPost, "/staff/finance/invoices/"+invoice.String()+"/fill", map[string]any{}, http.StatusOK, &inv)
	h.api(fin, http.MethodPost, "/staff/finance/invoices/"+invoice.String()+"/issue", map[string]any{}, http.StatusOK, &inv)
	if inv.Number == nil || *inv.Instalments[0].AmountCents+*inv.Instalments[1].AmountCents != 3_500_000 || inv.Instalments[1].DueOn == nil {
		t.Fatalf("issued first invoice: %+v", inv)
	}
}
