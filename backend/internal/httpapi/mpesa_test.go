package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Test values only: a made-up shortcode and secrets (never real ones).
const (
	testShortCode      = "600999"
	testDarajaKey      = "test-consumer-key"
	testDarajaSecret   = "test-consumer-secret"
	testCallbackSecret = "test-callback-secret-0123456789abcdef"
)

// fakeDaraja stands in for Safaricom: it hands out a token for the right
// keys and records what is registered or simulated.
type fakeDaraja struct {
	server     *httptest.Server
	mu         sync.Mutex
	registered []map[string]string
	simulated  []map[string]string
	refuse     bool
	tokens     int
}

func newFakeDaraja(t *testing.T) *fakeDaraja {
	f := &fakeDaraja{}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/v1/generate", func(w http.ResponseWriter, r *http.Request) {
		if k, s, ok := r.BasicAuth(); !ok || k != testDarajaKey || s != testDarajaSecret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.mu.Lock()
		f.tokens++
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":"3599"}`))
	})
	record := func(into *[]map[string]string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer test-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.refuse {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"errorCode":"400.003.02","errorMessage":"Bad Request - Invalid ConfirmationURL"}`))
				return
			}
			*into = append(*into, body)
			_, _ = w.Write([]byte(`{"OriginatorCoversationID":"test","ResponseCode":"0","ResponseDescription":"Success"}`))
		}
	}
	mux.HandleFunc("/mpesa/c2b/v2/registerurl", record(&f.registered))
	mux.HandleFunc("/mpesa/c2b/v2/simulate", record(&f.simulated))
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

var codeSeq int

// code is a fresh 10-character M-Pesa transaction code.
func code() string { codeSeq++; return fmt.Sprintf("TST%07d", codeSeq) }

// confirm sends a confirmation as Safaricom would, and returns the status.
func (h *harness) confirmation(secret, transID, amount, account string, extra map[string]any) int {
	h.t.Helper()
	body := map[string]any{"TransactionType": "Pay Bill", "TransID": transID, "TransTime": time.Now().In(time.FixedZone("EAT", 3*3600)).Format("20060102150405"),
		"TransAmount": amount, "BusinessShortCode": testShortCode, "BillRefNumber": account, "InvoiceNumber": "", "OrgAccountBalance": "",
		"ThirdPartyTransID": "", "MSISDN": "2547 ***** 126", "FirstName": "Test", "MiddleName": "", "LastName": "Payer"}
	for k, v := range extra {
		body[k] = v
	}
	w := h.do(req{method: http.MethodPost, path: "/api/v1/payments/c2b/" + secret + "/confirmation", body: body})
	return w.Code
}

func (h *harness) count(sql string, args ...any) int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}

func numberOf(h *harness, id uuid.UUID) string {
	var n string
	if err := h.pool.QueryRow(context.Background(), `SELECT student_number FROM students WHERE id = $1`, id).Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}

// Callbacks are accepted only on the secret path; anything else is a 404
// and nothing is stored.
func TestMpesaCallbacksNeedTheSecret(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	number := numberOf(h, a.students[0])
	if got := h.confirmation("wrong-secret-0123456789abcdef0123456", code(), "100", number, nil); got != http.StatusNotFound {
		t.Fatalf("wrong secret: %d", got)
	}
	w := h.do(req{method: http.MethodPost, path: "/api/v1/payments/c2b/" + testCallbackSecret + "/validation", body: map[string]any{"TransID": "X"}})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ResultCode":"0"`) {
		t.Fatalf("validation: %d %s", w.Code, w.Body)
	}
	if n := h.count(`SELECT count(*) FROM mpesa_callbacks`); n != 1 {
		t.Fatalf("%d callbacks stored, want only the accepted one", n)
	}

	// With an allow list, other addresses are refused even with the secret.
	h.mpesaAllowed = []string{"196.201.214.0/24"}
	h.rebuild()
	if got := h.confirmation(testCallbackSecret, code(), "100", number, nil); got != http.StatusNotFound {
		t.Fatalf("address not on the allow list: %d", got)
	}
	hr := httptest.NewRequest(http.MethodPost, "/api/v1/payments/c2b/"+testCallbackSecret+"/validation", strings.NewReader(`{}`))
	hr.RemoteAddr = "196.201.214.10:443"
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, hr)
	if rec.Code != http.StatusOK {
		t.Fatalf("listed address: %d %s", rec.Code, rec.Body)
	}
	if n := h.count(`SELECT count(*) FROM mpesa_callbacks`); n != 2 {
		t.Fatalf("%d callbacks stored, want 2", n)
	}
}

// AC-11 verified flow: a confirmation for our shortcode whose account number
// is a student's number becomes a confirmed, receipted payment once, however
// often Safaricom repeats it.
func TestMpesaPaymentIsMatchedAndReceipted(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	fin := h.financeOfficer()
	alice := a.students[0]
	// The payer typed the number in lower case without slashes.
	account := strings.ToLower(strings.ReplaceAll(numberOf(h, alice), "/", ""))
	tx := code()
	for range 3 {
		if got := h.confirmation(testCallbackSecret, tx, "1500.50", account, nil); got != http.StatusOK {
			t.Fatalf("confirmation: %d", got)
		}
	}
	if n := h.count(`SELECT count(*) FROM mpesa_callbacks WHERE kind = 'confirmation'`); n != 3 {
		t.Fatalf("%d callbacks stored, want every one (3)", n)
	}
	if n := h.count(`SELECT count(*) FROM payments WHERE reference = $1`, tx); n != 1 {
		t.Fatalf("%d payments for one M-Pesa code", n)
	}
	var status, via string
	var recorder, decider *uuid.UUID
	if err := h.pool.QueryRow(context.Background(), `SELECT p.status, p.confirmed_via, p.recorded_by, p.decided_by FROM payments p WHERE p.reference = $1`, tx).
		Scan(&status, &via, &recorder, &decider); err != nil {
		t.Fatal(err)
	}
	if status != "confirmed" || via != "provider" || recorder != nil || decider != nil {
		t.Fatalf("payment: %s via %s by %v/%v", status, via, recorder, decider)
	}
	acc := h.reconciles(fin, alice)
	if acc.Summary.BalanceCents != -150_050 || len(acc.Payments) != 1 || acc.Payments[0].ReceiptNumber == nil || !receiptNumberPattern.MatchString(*acc.Payments[0].ReceiptNumber) {
		t.Fatalf("account after M-Pesa: %+v", acc)
	}

	// The student sees it with its receipt, and how to pay next time.
	var fees struct {
		MpesaPaybill  *string `json:"mpesa_paybill"`
		AccountNumber string  `json:"account_number"`
		Payments      []struct{ Status string }
	}
	h.api(h.studentLogin(alice), http.MethodGet, "/student/fees", nil, http.StatusOK, &fees)
	if fees.MpesaPaybill == nil || *fees.MpesaPaybill != testShortCode || fees.AccountNumber != numberOf(h, alice) || len(fees.Payments) != 1 || fees.Payments[0].Status != "confirmed" {
		t.Fatalf("student fees: %+v", fees)
	}

	// Paid to someone else's shortcode: recorded as rejected, never credited.
	other := code()
	h.confirmation(testCallbackSecret, other, "100", account, map[string]any{"BusinessShortCode": "123456"})
	var list struct {
		Items []struct {
			TransID string `json:"trans_id"`
			Status  string
			Reason  string
		}
	}
	h.api(fin, http.MethodGet, "/staff/finance/mpesa/transactions?status=rejected", nil, http.StatusOK, &list)
	if len(list.Items) != 1 || list.Items[0].TransID != other || !strings.Contains(list.Items[0].Reason, "123456") {
		t.Fatalf("rejected: %+v", list.Items)
	}
	if n := h.count(`SELECT count(*) FROM payments WHERE reference = $1`, other); n != 0 {
		t.Fatal("a payment to another shortcode was credited")
	}

	// What Safaricom sent never changes.
	for _, sql := range []string{`UPDATE mpesa_transactions SET amount_cents = 1`, `DELETE FROM mpesa_transactions`, `UPDATE mpesa_callbacks SET body = '{}'`, `DELETE FROM mpesa_callbacks`} {
		if _, err := h.pool.Exec(context.Background(), sql); err == nil {
			t.Errorf("allowed: %s", sql)
		}
	}
	if n := h.auditCount("finance.mpesa_received"); n != 2 {
		t.Errorf("finance.mpesa_received audited %d times, want 2", n)
	}
}

// A payment whose account number matches no student waits for Finance, who
// assigns it to a student or marks it for refund.
func TestMpesaUnmatchedPayments(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	fin := h.financeOfficer()
	h.exec(`INSERT INTO roles (key, name, description) VALUES ('cashier', 'Cashier', 'Records payments')`)
	h.exec(`INSERT INTO role_permissions (role_id, permission_key) SELECT id, k FROM roles, unnest(ARRAY['finance.view', 'finance.payment.record']) k WHERE key = 'cashier'`)
	h.createUser("cashier@example.invalid", "cashier")
	cashier := h.mustLogin("cashier@example.invalid")

	typo, wrong := code(), code()
	h.confirmation(testCallbackSecret, typo, "2000", "DTC 2027 99", nil)
	h.confirmation(testCallbackSecret, wrong, "300", "", nil)
	var list struct {
		Items []struct {
			ID      uuid.UUID
			TransID string `json:"trans_id"`
			Reason  string
		}
		Total int
	}
	h.api(fin, http.MethodGet, "/staff/finance/mpesa/transactions?status=unmatched", nil, http.StatusOK, &list)
	if list.Total != 2 || !strings.Contains(list.Items[0].Reason+list.Items[1].Reason, "No student has the account number") {
		t.Fatalf("unmatched: %+v", list)
	}
	byCode := map[string]uuid.UUID{}
	for _, i := range list.Items {
		byCode[i.TransID] = i.ID
	}
	var status struct{ Unmatched int }
	h.api(fin, http.MethodGet, "/staff/finance/mpesa", nil, http.StatusOK, &status)
	if status.Unmatched != 2 {
		t.Fatalf("status: %+v", status)
	}

	assign := "/staff/finance/mpesa/transactions/" + byCode[typo].String() + "/assign"
	if got := h.code(cashier, http.MethodPost, assign, map[string]any{"student_id": a.students[1], "note": "Parent phoned"}); got != "403 forbidden" {
		t.Fatalf("cashier assigning: %s", got)
	}
	if got := h.code(fin, http.MethodPost, assign, map[string]any{"student_id": a.students[1]}); got != "422 validation_failed" {
		t.Fatalf("assign without a note: %s", got)
	}
	h.api(fin, http.MethodPost, assign, map[string]any{"student_id": a.students[1], "note": "Parent phoned with the code; account number mistyped."}, http.StatusNoContent, nil)
	if got := h.code(fin, http.MethodPost, assign, map[string]any{"student_id": a.students[1], "note": "Again, by mistake"}); got != "409 already_settled" {
		t.Fatalf("assigning twice: %s", got)
	}
	acc := h.reconciles(fin, a.students[1])
	if acc.Summary.BalanceCents != -200_000 || len(acc.Payments) != 1 || acc.Payments[0].ReceiptNumber == nil {
		t.Fatalf("after assigning: %+v", acc)
	}

	refund := "/staff/finance/mpesa/transactions/" + byCode[wrong].String() + "/refund"
	h.api(fin, http.MethodPost, refund, map[string]any{"note": "Not a student of the college; returned from the M-Pesa portal."}, http.StatusNoContent, nil)
	if got := h.code(fin, http.MethodPost, "/staff/finance/mpesa/transactions/"+byCode[wrong].String()+"/assign",
		map[string]any{"student_id": a.students[0], "note": "Changed my mind"}); got != "409 already_settled" {
		t.Fatalf("assigning a refund: %s", got)
	}

	// The day's reconciliation: everything received, what was credited.
	var rec struct {
		ReceivedCount  int   `json:"received_count"`
		ReceivedCents  int64 `json:"received_cents"`
		CreditedCents  int64 `json:"credited_cents"`
		UnmatchedCents int64 `json:"unmatched_cents"`
		Lines          []struct {
			Status string
			Count  int
		}
	}
	h.api(fin, http.MethodGet, "/staff/finance/mpesa/reconciliation?day="+time.Now().In(time.FixedZone("EAT", 3*3600)).Format(time.DateOnly), nil, http.StatusOK, &rec)
	if rec.ReceivedCount != 2 || rec.ReceivedCents != 230_000 || rec.CreditedCents != 200_000 || rec.UnmatchedCents != 0 || len(rec.Lines) != 2 {
		t.Fatalf("reconciliation: %+v", rec)
	}
	for action, want := range map[string]int{"finance.mpesa_assigned": 1, "finance.mpesa_refund": 1} {
		if n := h.auditCount(action); n != want {
			t.Errorf("%s audited %d times, want %d", action, n, want)
		}
	}
}

// A payment Finance recorded by hand with its code is confirmed by the
// callback rather than credited twice; a mismatch waits for Finance.
func TestMpesaConfirmsAPaymentRecordedByHand(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	fin := h.financeOfficer()
	alice := a.students[0]
	same, differs := code(), code()
	for c, amount := range map[string]int{same: 50_000, differs: 70_000} {
		h.keyed(fin, http.MethodPost, "/staff/finance/payments", newKey(), map[string]any{"student_id": alice, "method": "mpesa", "amount_cents": amount,
			"paid_on": time.Now().Format(time.DateOnly), "reference": c}).expect(t, http.StatusOK, "")
	}
	number := numberOf(h, alice)
	h.confirmation(testCallbackSecret, same, "500", number, nil)
	h.confirmation(testCallbackSecret, differs, "800", number, nil)

	if n := h.count(`SELECT count(*) FROM payments WHERE reference = $1`, same); n != 1 {
		t.Fatalf("%d payments for a code recorded by hand", n)
	}
	var st, via string
	if err := h.pool.QueryRow(context.Background(), `SELECT status, confirmed_via FROM payments WHERE reference = $1`, same).Scan(&st, &via); err != nil || st != "confirmed" || via != "provider" {
		t.Fatalf("hand-recorded payment after the callback: %s %s %v", st, via, err)
	}
	var reason string
	if err := h.pool.QueryRow(context.Background(), `SELECT status || ': ' || reason FROM mpesa_transactions WHERE trans_id = $1`, differs).Scan(&reason); err != nil || !strings.HasPrefix(reason, "unmatched: A payment with this code") {
		t.Fatalf("mismatch: %q %v", reason, err)
	}
	if acc := h.reconciles(fin, alice); acc.Summary.PendingCents != 70_000 || acc.Summary.BalanceCents != -50_000 {
		t.Fatalf("account: %+v", acc.Summary)
	}
}

// Setup: only finance.mpesa.setup registers the URLs, with the secret path;
// Safaricom's refusal is shown; stored callbacks can be processed again.
func TestMpesaSetup(t *testing.T) {
	h := newHarness(t)
	fin := h.financeOfficer()
	h.createUser("admin@example.invalid", "super_admin")
	admin := h.mustLogin("admin@example.invalid")

	if got := h.code(fin, http.MethodPost, "/staff/finance/mpesa/register-urls", map[string]any{}); got != "403 forbidden" {
		t.Fatalf("finance officer registering: %s", got)
	}
	h.api(admin, http.MethodPost, "/staff/finance/mpesa/register-urls", map[string]any{}, http.StatusOK, nil)
	h.daraja.mu.Lock()
	reg := h.daraja.registered
	h.daraja.mu.Unlock()
	if len(reg) != 1 || reg[0]["ShortCode"] != testShortCode || reg[0]["ResponseType"] != "Completed" ||
		reg[0]["ConfirmationURL"] != testOrigin+"/api/v1/payments/c2b/"+testCallbackSecret+"/confirmation" ||
		strings.Contains(strings.ToLower(reg[0]["ConfirmationURL"]), "mpesa") {
		t.Fatalf("registered: %+v", reg)
	}
	// The status page never shows the secret.
	var status struct {
		ConfirmationURL string `json:"confirmation_url"`
	}
	h.api(fin, http.MethodGet, "/staff/finance/mpesa", nil, http.StatusOK, &status)
	if strings.Contains(status.ConfirmationURL, testCallbackSecret) {
		t.Fatalf("secret shown: %s", status.ConfirmationURL)
	}
	h.daraja.mu.Lock()
	h.daraja.refuse = true
	h.daraja.mu.Unlock()
	if got := h.code(admin, http.MethodPost, "/staff/finance/mpesa/register-urls", map[string]any{}); got != "502 mpesa_refused" {
		t.Fatalf("Safaricom refusing: %s", got)
	}
	h.daraja.mu.Lock()
	h.daraja.refuse = false
	h.daraja.mu.Unlock()

	h.api(admin, http.MethodPost, "/staff/finance/mpesa/simulate", map[string]any{"amount_cents": 10_000, "msisdn": "254708374149", "account": "DTC/2027/0001"}, http.StatusOK, nil)
	if got := h.code(admin, http.MethodPost, "/staff/finance/mpesa/simulate", map[string]any{"amount_cents": 10_050, "msisdn": "0708", "account": ""}); got != "422 validation_failed" {
		t.Fatalf("bad simulation: %s", got)
	}

	// A confirmation stored but not processed (here: inserted directly) is
	// picked up by reprocessing; a malformed one stays stored and is skipped.
	a := h.academy()
	raw, _ := json.Marshal(map[string]any{"TransID": code(), "TransTime": time.Now().Format("20060102150405"), "TransAmount": "250",
		"BusinessShortCode": testShortCode, "BillRefNumber": numberOf(h, a.students[0])})
	h.exec(`INSERT INTO mpesa_callbacks (kind, body) VALUES ('confirmation', $1), ('confirmation', '{"TransID": "bad"}')`, raw)
	var out struct{ Processed int }
	h.api(admin, http.MethodPost, "/staff/finance/mpesa/reprocess", map[string]any{}, http.StatusOK, &out)
	if out.Processed != 1 || h.count(`SELECT count(*) FROM mpesa_transactions`) != 1 {
		t.Fatalf("reprocessed %d", out.Processed)
	}
	if h.auditCount("finance.mpesa_urls_registered") != 1 {
		t.Error("registration not audited")
	}
}

// Amounts are read on the digits.
func TestMpesaAmountsAreExact(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	number := numberOf(h, a.students[0])
	for i, c := range []struct {
		amount any
		cents  int64
	}{{"10.30", 1030}, {"19.99", 1999}, {"7", 700}, {json.Number("12.5"), 1250}} {
		tx := code()
		raw := map[string]any{"TransAmount": c.amount}
		if got := h.confirmation(testCallbackSecret, tx, "0", number, raw); got != http.StatusOK {
			t.Fatalf("%d: %d", i, got)
		}
		var cents int64
		if err := h.pool.QueryRow(context.Background(), `SELECT amount_cents FROM payments WHERE reference = $1`, tx).Scan(&cents); err != nil || cents != c.cents {
			t.Errorf("%v: %d cents (%v), want %d", c.amount, cents, err, c.cents)
		}
	}
	// No amount, no money: stored, not credited.
	bad := code()
	h.confirmation(testCallbackSecret, bad, "-5", number, nil)
	if n := h.count(`SELECT count(*) FROM payments WHERE reference = $1`, bad); n != 0 {
		t.Fatal("a negative amount was credited")
	}
}
