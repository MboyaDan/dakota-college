package mpesa

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/finance"
)

// What Finance sees and does with M-Pesa: the setup, payments that match no
// student, and a day's reconciliation against the M-Pesa statement.

type Status struct {
	Environment     string     `json:"environment"`
	ShortCode       string     `json:"short_code"`
	ResponseType    string     `json:"response_type"`
	ConfirmationURL string     `json:"confirmation_url"`
	ValidationURL   string     `json:"validation_url"`
	IPAllowList     bool       `json:"ip_allow_list"`
	LastCallbackAt  *time.Time `json:"last_callback_at"`
	Unmatched       int64      `json:"unmatched"`
}

// urls are the callback URLs; shown is the same with the secret hidden.
func (s *Service) urls(shown bool) (confirmation, validation string) {
	secret := s.cfg.CallbackSecret
	if shown {
		secret = secret[:4] + "…"
	}
	return s.cfg.CallbackBaseURL + CallbackPath(secret, "confirmation"), s.cfg.CallbackBaseURL + CallbackPath(secret, "validation")
}

func (s *Service) Status(ctx context.Context) (*Status, error) {
	a, err := dbq.New(s.pool).MpesaActivity(ctx)
	if err != nil {
		return nil, err
	}
	out := &Status{Environment: s.cfg.Environment, ShortCode: s.cfg.ShortCode, ResponseType: s.cfg.ResponseType,
		IPAllowList: s.guard.HasAllowList(), Unmatched: a.Unmatched}
	out.ConfirmationURL, out.ValidationURL = s.urls(true)
	if a.LastCallbackEpoch > 0 {
		t := time.Unix(a.LastCallbackEpoch, 0)
		out.LastCallbackAt = &t
	}
	return out, nil
}

// Register tells Safaricom where to send C2B validation and confirmation
// calls for the college shortcode.
func (s *Service) Register(ctx context.Context, req audit.Request) (string, error) {
	confirmation, validation := s.urls(false)
	msg, err := s.daraja.register(ctx, registerRequest{ShortCode: s.cfg.ShortCode, ResponseType: s.cfg.ResponseType,
		ConfirmationURL: confirmation, ValidationURL: validation})
	if err != nil {
		return "", err
	}
	return msg, s.tx(ctx, func(q *dbq.Queries) error {
		return audit.Write(ctx, q, req, audit.Entry{Action: "finance.mpesa_urls_registered", EntityType: "mpesa", EntityID: s.cfg.ShortCode,
			Details: map[string]any{"environment": s.cfg.Environment, "answer": msg}})
	})
}

// Simulate asks the Daraja sandbox to send a test payment to our URLs.
func (s *Service) Simulate(ctx context.Context, amountCents int64, msisdn, account string) (string, error) {
	if s.cfg.Environment != "sandbox" {
		return "", ErrSandboxOnly
	}
	var pr content.Problems
	if amountCents < 100 || amountCents%100 != 0 || amountCents > 15_000_000 {
		pr.Add("amount_cents", "Enter whole shillings from 1 to 150,000.")
	}
	if !isDigits(msisdn, 12) {
		pr.Add("msisdn", "Enter the sandbox test phone number, 12 digits starting with 254.")
	}
	if strings.TrimSpace(account) == "" {
		pr.Add("account", "Enter the account number (a student number).")
	}
	if err := pr.Err(); err != nil {
		return "", err
	}
	return s.daraja.simulate(ctx, simulateRequest{ShortCode: s.cfg.ShortCode, CommandID: "CustomerPayBillOnline",
		Amount: fmt.Sprint(amountCents / 100), Msisdn: msisdn, BillRefNumber: account})
}

func isDigits(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Reprocess records stored confirmations that were not processed, for
// example because the database was briefly unavailable.
func (s *Service) Reprocess(ctx context.Context) (int, error) {
	rows, err := dbq.New(s.pool).UnprocessedMpesaCallbacks(ctx)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, r := range rows {
		if err := s.process(ctx, r.ID, r.Body); err != nil {
			s.logger.Warn("M-Pesa callback still not processed", "callback", r.ID, "error", err.Error())
			continue
		}
		done++
	}
	return done, nil
}

type Transaction struct {
	ID            uuid.UUID  `json:"id"`
	TransID       string     `json:"trans_id"`
	TransType     string     `json:"trans_type"`
	TransTime     time.Time  `json:"trans_time"`
	AmountCents   int64      `json:"amount_cents"`
	AccountNumber string     `json:"account_number"`
	Phone         string     `json:"phone"`
	PayerName     string     `json:"payer_name"`
	Status        string     `json:"status"`
	Reason        string     `json:"reason"`
	Note          string     `json:"note"`
	StudentID     *uuid.UUID `json:"student_id"`
	StudentNumber *string    `json:"student_number"`
	StudentName   *string    `json:"student_name"`
	PaymentID     *uuid.UUID `json:"payment_id"`
	ReceiptNumber *string    `json:"receipt_number"`
	HandledAt     *time.Time `json:"handled_at"`
	HandledByName *string    `json:"handled_by_name"`
}

type Filter struct {
	Status *string
	Day    *time.Time
	Query  string
	Page   int
}

const pageSize = 50

func (s *Service) List(ctx context.Context, f Filter) ([]Transaction, int64, error) {
	var q *string
	if v := strings.TrimSpace(f.Query); v != "" {
		v = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(v)
		q = &v
	}
	page := max(f.Page, 1)
	rows, err := dbq.New(s.pool).ListMpesaTransactions(ctx, dbq.ListMpesaTransactionsParams{Status: f.Status, Day: f.Day, Q: q,
		Lim: pageSize, Off: int32((page - 1) * pageSize)})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Transaction, 0, len(rows))
	var total int64
	for _, r := range rows {
		total = r.TotalCount
		t := Transaction{ID: r.ID, TransID: r.TransID, TransType: r.TransType, TransTime: r.TransTime, AmountCents: r.AmountCents,
			AccountNumber: r.BillRef, Phone: r.Msisdn, PayerName: r.PayerName, Status: r.Status, Reason: r.Reason, Note: r.Note,
			StudentID: r.StudentID, StudentNumber: r.StudentNumber, PaymentID: r.PaymentID, ReceiptNumber: r.ReceiptNumber,
			HandledAt: r.HandledAt, HandledByName: r.HandledByName}
		if r.FirstName != nil {
			name := strings.Join(strings.Fields(*r.FirstName+" "+deref(r.MiddleName)+" "+deref(r.LastName)), " ")
			t.StudentName = &name
		}
		out = append(out, t)
	}
	return out, total, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Assign credits an unmatched payment to the student Finance chose: it
// becomes a confirmed, receipted payment (the money is in the M-Pesa
// statement already). Audited, with the reason.
func (s *Service) Assign(ctx context.Context, p *auth.Principal, id, student uuid.UUID, note string, req audit.Request) error {
	note = strings.TrimSpace(note)
	if n := len([]rune(note)); n < 5 || n > 1000 {
		return content.Problems{{Field: "note", Message: "Say how you know whose payment it is, in 5 to 1,000 characters."}}.Err()
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		t, err := q.LockMpesaTransaction(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if t.Status != "unmatched" {
			return ErrDecided
		}
		if _, err := q.FinanceStudent(ctx, student); errors.Is(err, pgx.ErrNoRows) {
			return content.Problems{{Field: "student_id", Message: "Choose a student."}}.Err()
		} else if err != nil {
			return err
		}
		if _, err := q.MpesaPaymentByReference(ctx, t.TransID); err == nil {
			return content.Problems{{Field: "student_id", Message: "A payment with this M-Pesa code is already recorded. Reject or correct that payment first."}}.Err()
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		payment, err := q.InsertProviderPayment(ctx, dbq.InsertProviderPaymentParams{StudentID: student, AmountCents: t.AmountCents,
			PaidOn: dateOf(t.TransTime), Reference: t.TransID, PayerName: t.PayerName, Note: "Assigned by Finance: " + note, RecordedBy: &p.UserID})
		if err != nil {
			return err
		}
		if _, err := finance.ConfirmByProvider(ctx, q, payment, &p.UserID, "Paid to the college paybill; account number did not match.", s.now()); err != nil {
			return err
		}
		if err := q.SettleMpesaTransaction(ctx, dbq.SettleMpesaTransactionParams{ID: id, Status: "assigned", StudentID: &student,
			PaymentID: &payment, HandledBy: &p.UserID, Note: note}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "finance.mpesa_assigned", EntityType: "mpesa_transaction", EntityID: id.String(),
			Details: map[string]any{"student_id": student.String(), "payment_id": payment.String(), "amount_cents": t.AmountCents}})
	})
}

// Refund marks an unmatched payment to be returned to the payer. The refund
// itself is made outside the system (from the M-Pesa portal).
func (s *Service) Refund(ctx context.Context, p *auth.Principal, id uuid.UUID, note string, req audit.Request) error {
	note = strings.TrimSpace(note)
	if n := len([]rune(note)); n < 5 || n > 1000 {
		return content.Problems{{Field: "note", Message: "Say why it is being returned, in 5 to 1,000 characters."}}.Err()
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		t, err := q.LockMpesaTransaction(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if t.Status != "unmatched" {
			return ErrDecided
		}
		if err := q.SettleMpesaTransaction(ctx, dbq.SettleMpesaTransactionParams{ID: id, Status: "refund", HandledBy: &p.UserID, Note: note}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "finance.mpesa_refund", EntityType: "mpesa_transaction", EntityID: id.String(),
			Details: map[string]any{"amount_cents": t.AmountCents}})
	})
}

type DayLine struct {
	Status     string `json:"status"`
	Count      int64  `json:"count"`
	TotalCents int64  `json:"total_cents"`
}

type Reconciliation struct {
	Day string `json:"day"`
	// What arrived from Safaricom, by outcome; compare the total with the
	// M-Pesa statement for the day.
	Lines          []DayLine `json:"lines"`
	ReceivedCount  int64     `json:"received_count"`
	ReceivedCents  int64     `json:"received_cents"`
	CreditedCents  int64     `json:"credited_cents"`
	UnmatchedCents int64     `json:"unmatched_cents"`
	// M-Pesa payments recorded by hand for the day that Safaricom has not
	// told us about: check each against the statement.
	ManualCount   int64 `json:"manual_count"`
	ManualCents   int64 `json:"manual_cents"`
	ManualPending int64 `json:"manual_pending"`
}

// Reconcile is one day's M-Pesa money for comparing with the statement.
func (s *Service) Reconcile(ctx context.Context, day time.Time) (*Reconciliation, error) {
	q := dbq.New(s.pool)
	rows, err := q.MpesaDaySummary(ctx, day)
	if err != nil {
		return nil, err
	}
	out := &Reconciliation{Day: day.Format(time.DateOnly), Lines: []DayLine{}}
	for _, r := range rows {
		out.Lines = append(out.Lines, DayLine{Status: r.Status, Count: r.N, TotalCents: r.TotalCents})
		if r.Status == "rejected" {
			continue // not paid to the college's shortcode
		}
		out.ReceivedCount += r.N
		out.ReceivedCents += r.TotalCents
		switch r.Status {
		case "matched", "assigned":
			out.CreditedCents += r.TotalCents
		case "unmatched":
			out.UnmatchedCents += r.TotalCents
		}
	}
	m, err := q.MpesaManualPaymentsOn(ctx, day)
	if err != nil {
		return nil, err
	}
	out.ManualCount, out.ManualCents, out.ManualPending = m.N, m.TotalCents, m.Pending
	return out, nil
}
