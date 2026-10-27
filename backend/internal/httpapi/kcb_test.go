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

// Test values only (never real ones).
const (
	testKcbSecret  = "test-kcb-ipn-secret-xxxxxxxxxxxxxxxxxxxxxxxx" // obviously fake, low entropy: not a secret
	testKcbAccount = "TESTACCOUNT001"
)

var kcbSeq int

func kcbRef() string { kcbSeq++; return fmt.Sprintf("FT%08d", kcbSeq) }

// notify sends a notification as KCB would and returns the response.
func (h *harness) kcbNotify(secret, ref, amount, customerRef string, extra map[string]any) keyedResponse {
	h.t.Helper()
	body := map[string]any{"transactionReference": ref, "requestId": uuid.NewString(), "channelCode": "202",
		"timestamp": time.Now().In(time.FixedZone("EAT", 3*3600)).Format("20060102150405"), "transactionAmount": amount, "currency": "KES",
		"customerReference": customerRef, "customerName": "Test Payer", "customerMobileNumber": "254700000000", "balance": "",
		"narration": "Fees", "creditAccountIdentifier": testKcbAccount, "organizationShortCode": "000000", "tillNumber": "000000"}
	for k, v := range extra {
		body[k] = v
	}
	return keyedResponse{h.do(req{method: http.MethodPost, path: "/api/v1/payments/kcb/" + secret + "/notification", body: body,
		headers: map[string]string{"Signature": "dGVzdC1zaWduYXR1cmU="}})}
}

// Until KCB's signature can be checked (D-175), a notification is never
// proof of payment: a match becomes a pending bank payment that Finance
// confirms; it is recorded once however often KCB repeats it.
func TestKcbNotificationBecomesPendingPayment(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	fin := h.financeOfficer()
	alice := a.students[0]
	account := strings.ToLower(strings.ReplaceAll(numberOf(h, alice), "/", " "))

	h.kcbNotify("wrong-secret-0123456789abcdef0123456789", kcbRef(), "100.00", account, nil).expect(t, http.StatusNotFound, "not_found")
	if n := h.count(`SELECT count(*) FROM kcb_notifications`); n != 0 {
		t.Fatalf("%d notifications stored from a wrong secret", n)
	}

	ref := kcbRef()
	var ack struct {
		TransactionID string `json:"transactionID"`
		StatusCode    string `json:"statusCode"`
		StatusMessage string `json:"statusMessage"`
	}
	for range 2 {
		w := h.kcbNotify(testKcbSecret, ref, "2500.50", account, nil)
		w.expect(t, http.StatusOK, "")
		w.decode(t, &ack)
	}
	if ack.TransactionID != ref || ack.StatusCode != "0" || ack.StatusMessage != "Notification received" {
		t.Fatalf("acknowledgement: %+v", ack)
	}
	if n := h.count(`SELECT count(*) FROM kcb_notifications WHERE signature = 'dGVzdC1zaWduYXR1cmU='`); n != 2 {
		t.Fatalf("%d notifications stored with their signature, want 2", n)
	}
	var id uuid.UUID
	var status, source string
	var recorder *uuid.UUID
	if err := h.pool.QueryRow(context.Background(), `SELECT id, status, source, recorded_by FROM payments WHERE reference = $1`, ref).
		Scan(&id, &status, &source, &recorder); err != nil {
		t.Fatal(err)
	}
	if status != "pending" || source != "kcb" || recorder != nil || h.count(`SELECT count(*) FROM payments WHERE reference = $1`, ref) != 1 {
		t.Fatalf("payment: %s from %s by %v", status, source, recorder)
	}
	acc := h.reconciles(fin, alice)
	if acc.Summary.BalanceCents != 0 || acc.Summary.PendingCents != 250_050 {
		t.Fatalf("a notification counted before confirmation: %+v", acc.Summary)
	}
	var pm paymentView
	h.api(fin, http.MethodGet, "/staff/finance/payments/"+id.String(), nil, http.StatusOK, &pm)
	var shown struct {
		RecordedByName string `json:"recorded_by_name"`
		Method         string
	}
	h.api(fin, http.MethodGet, "/staff/finance/payments/"+id.String(), nil, http.StatusOK, &shown)
	if shown.RecordedByName != "KCB notification" || shown.Method != "bank" {
		t.Fatalf("shown as: %+v", shown)
	}
	// Finance finds it on the KCB statement and confirms it.
	h.keyed(fin, http.MethodPost, "/staff/finance/payments/"+id.String()+"/confirm", newKey(), map[string]any{"note": "On the KCB statement."}).
		expect(t, http.StatusOK, "")
	if acc := h.reconciles(fin, alice); acc.Summary.BalanceCents != -250_050 {
		t.Fatalf("after confirmation: %+v", acc.Summary)
	}

	// Another account, or not shillings: recorded, never credited.
	other, dollars := kcbRef(), kcbRef()
	h.kcbNotify(testKcbSecret, other, "100", account, map[string]any{"creditAccountIdentifier": "SOMEONEELSE"}).expect(t, http.StatusOK, "")
	h.kcbNotify(testKcbSecret, dollars, "100", account, map[string]any{"currency": "USD"}).expect(t, http.StatusOK, "")
	var list struct {
		Items []struct {
			TransID  string `json:"trans_id"`
			Reason   string
			Verified bool
		}
	}
	h.api(fin, http.MethodGet, "/staff/finance/kcb/transactions?status=rejected", nil, http.StatusOK, &list)
	if len(list.Items) != 2 || list.Items[0].Verified {
		t.Fatalf("rejected: %+v", list.Items)
	}
	if n := h.count(`SELECT count(*) FROM payments WHERE reference IN ($1, $2)`, other, dollars); n != 0 {
		t.Fatal("a payment for another account or currency was recorded")
	}
	for _, sql := range []string{`UPDATE kcb_transactions SET amount_cents = 1`, `DELETE FROM kcb_transactions`, `UPDATE kcb_notifications SET signature = ''`} {
		if _, err := h.pool.Exec(context.Background(), sql); err == nil {
			t.Errorf("allowed: %s", sql)
		}
	}
}

// Notifications that match no student, are above the limit, or clash with
// a hand-recorded payment wait for Finance to assign or return.
func TestKcbUnmatchedNotifications(t *testing.T) {
	h := newHarness(t)
	h.kcbMax = 1_000_000 // KES 10,000
	h.rebuild()
	a := h.academy()
	fin := h.financeOfficer()
	number := numberOf(h, a.students[0])

	stray, large, clash := kcbRef(), kcbRef(), kcbRef()
	h.kcbNotify(testKcbSecret, stray, "750", "INV-0001", nil).expect(t, http.StatusOK, "")
	h.kcbNotify(testKcbSecret, large, "10000.01", number, nil).expect(t, http.StatusOK, "")
	h.keyed(fin, http.MethodPost, "/staff/finance/payments", newKey(), map[string]any{"student_id": a.students[1], "method": "bank",
		"amount_cents": 100, "paid_on": time.Now().Format(time.DateOnly), "reference": clash}).expect(t, http.StatusOK, "")
	h.kcbNotify(testKcbSecret, clash, "1", number, nil).expect(t, http.StatusOK, "")

	var list struct {
		Items []struct {
			ID      uuid.UUID
			TransID string `json:"trans_id"`
			Reason  string
		}
	}
	h.api(fin, http.MethodGet, "/staff/finance/kcb/transactions?status=unmatched", nil, http.StatusOK, &list)
	reasons := map[string]string{}
	ids := map[string]uuid.UUID{}
	for _, i := range list.Items {
		reasons[i.TransID], ids[i.TransID] = i.Reason, i.ID
	}
	if !strings.Contains(reasons[stray], "No student") || !strings.Contains(reasons[large], "Above") || !strings.Contains(reasons[clash], "recorded by hand") {
		t.Fatalf("reasons: %+v", reasons)
	}
	var status struct{ Unmatched int }
	h.api(fin, http.MethodGet, "/staff/finance/kcb", nil, http.StatusOK, &status)
	if status.Unmatched != 3 {
		t.Fatalf("status: %+v", status)
	}

	h.api(fin, http.MethodPost, "/staff/finance/kcb/transactions/"+ids[stray].String()+"/assign",
		map[string]any{"student_id": a.students[2], "note": "On the KCB statement; the parent used the invoice number."}, http.StatusNoContent, nil)
	if got := h.code(fin, http.MethodPost, "/staff/finance/kcb/transactions/"+ids[stray].String()+"/assign",
		map[string]any{"student_id": a.students[2], "note": "Twice by mistake"}); got != "409 already_settled" {
		t.Fatalf("assigning twice: %s", got)
	}
	if acc := h.reconciles(fin, a.students[2]); acc.Summary.BalanceCents != -75_000 || acc.Payments[0].ReceiptNumber == nil {
		t.Fatalf("assigned: %+v", acc)
	}
	h.api(fin, http.MethodPost, "/staff/finance/kcb/transactions/"+ids[large].String()+"/refund",
		map[string]any{"note": "Paid twice by the sponsor; returned by the bank."}, http.StatusNoContent, nil)

	var rec struct {
		ReceivedCount int   `json:"received_count"`
		CreditedCents int64 `json:"credited_cents"`
	}
	h.api(fin, http.MethodGet, "/staff/finance/kcb/reconciliation?day="+time.Now().In(time.FixedZone("EAT", 3*3600)).Format(time.DateOnly), nil, http.StatusOK, &rec)
	if rec.ReceivedCount != 3 || rec.CreditedCents != 75_000 {
		t.Fatalf("reconciliation: %+v", rec)
	}
}

// A notification that cannot be read is stored, acknowledged with status 1,
// and can be processed again; amounts are read exactly.
func TestKcbStoredFirst(t *testing.T) {
	h := newHarness(t)
	a := h.academy()
	h.createUser("admin@example.invalid", "super_admin")
	admin := h.mustLogin("admin@example.invalid")
	number := numberOf(h, a.students[0])

	var ack struct{ StatusCode string }
	w := h.kcbNotify(testKcbSecret, kcbRef(), "12.345", number, nil)
	w.expect(t, http.StatusOK, "")
	w.decode(t, &ack)
	if ack.StatusCode != "1" || h.count(`SELECT count(*) FROM kcb_notifications`) != 1 || h.count(`SELECT count(*) FROM kcb_transactions`) != 0 {
		t.Fatalf("unreadable amount: %+v", ack)
	}
	ref := kcbRef()
	raw, _ := json.Marshal(map[string]any{"transactionReference": ref, "transactionAmount": "10.30", "currency": "KES",
		"customerReference": number, "creditAccountIdentifier": testKcbAccount, "timestamp": "2021111103005"})
	h.exec(`INSERT INTO kcb_notifications (body) VALUES ($1)`, raw)
	var out struct{ Processed int }
	h.api(admin, http.MethodPost, "/staff/finance/kcb/reprocess", map[string]any{}, http.StatusOK, &out)
	var cents int64
	if err := h.pool.QueryRow(context.Background(), `SELECT amount_cents FROM payments WHERE reference = $1`, ref).Scan(&cents); err != nil || out.Processed != 1 || cents != 1030 {
		t.Fatalf("reprocessed %d, amount %d (%v)", out.Processed, cents, err)
	}
}
