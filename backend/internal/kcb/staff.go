package kcb

import (
	"context"
	"errors"
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

// What Finance sees and does with KCB notifications. The shapes match the
// M-Pesa ones, so the screens are shared.

type Status struct {
	CreditAccount      string     `json:"credit_account"`
	Till               string     `json:"till"`
	NotificationURL    string     `json:"notification_url"`
	IPAllowList        bool       `json:"ip_allow_list"`
	SignatureVerified  bool       `json:"signature_verified"`
	LastNotificationAt *time.Time `json:"last_notification_at"`
	Unmatched          int64      `json:"unmatched"`
}

func (s *Service) Status(ctx context.Context) (*Status, error) {
	a, err := dbq.New(s.pool).KcbActivity(ctx)
	if err != nil {
		return nil, err
	}
	out := &Status{CreditAccount: s.cfg.CreditAccount, Till: s.cfg.Till, IPAllowList: s.guard.HasAllowList(),
		NotificationURL: NotificationPath(s.cfg.Secret[:4] + "…"), Unmatched: a.Unmatched}
	if a.LastNotificationEpoch > 0 {
		t := time.Unix(a.LastNotificationEpoch, 0)
		out.LastNotificationAt = &t
	}
	return out, nil
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
	Narration     string     `json:"narration"`
	Verified      bool       `json:"verified"`
	Status        string     `json:"status"`
	Reason        string     `json:"reason"`
	Note          string     `json:"note"`
	StudentID     *uuid.UUID `json:"student_id"`
	StudentNumber *string    `json:"student_number"`
	StudentName   *string    `json:"student_name"`
	PaymentID     *uuid.UUID `json:"payment_id"`
	PaymentStatus *string    `json:"payment_status"`
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

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *Service) List(ctx context.Context, f Filter) ([]Transaction, int64, error) {
	var q *string
	if v := strings.TrimSpace(f.Query); v != "" {
		v = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(v)
		q = &v
	}
	page := max(f.Page, 1)
	rows, err := dbq.New(s.pool).ListKcbTransactions(ctx, dbq.ListKcbTransactionsParams{Status: f.Status, Day: f.Day, Q: q,
		Lim: pageSize, Off: int32((page - 1) * pageSize)})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Transaction, 0, len(rows))
	var total int64
	for _, r := range rows {
		total = r.TotalCount
		t := Transaction{ID: r.ID, TransID: r.TransactionReference, TransType: "KCB " + r.ChannelCode, TransTime: r.SentAt,
			AmountCents: r.AmountCents, AccountNumber: r.CustomerReference, Phone: r.CustomerMobile, PayerName: r.CustomerName,
			Narration: r.Narration, Verified: r.Verified, Status: r.Status, Reason: r.Reason, Note: r.Note, StudentID: r.StudentID,
			StudentNumber: r.StudentNumber, PaymentID: r.PaymentID, PaymentStatus: r.PaymentStatus, ReceiptNumber: r.ReceiptNumber,
			HandledAt: r.HandledAt, HandledByName: r.HandledByName}
		if r.FirstName != nil {
			name := strings.Join(strings.Fields(*r.FirstName+" "+deref(r.MiddleName)+" "+deref(r.LastName)), " ")
			t.StudentName = &name
		}
		out = append(out, t)
	}
	return out, total, nil
}

// Assign credits an unmatched notification to the student Finance chose,
// after Finance has found it on the KCB statement: it becomes a confirmed,
// receipted bank payment. Audited, with the note.
func (s *Service) Assign(ctx context.Context, p *auth.Principal, id, student uuid.UUID, note string, req audit.Request) error {
	note = strings.TrimSpace(note)
	if n := len([]rune(note)); n < 5 || n > 1000 {
		return content.Problems{{Field: "note", Message: "Say how you know whose payment it is, in 5 to 1,000 characters."}}.Err()
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		t, err := q.LockKcbTransaction(ctx, id)
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
		if _, err := q.BankPaymentByReference(ctx, t.TransactionReference); err == nil {
			return content.Problems{{Field: "student_id", Message: "A bank payment with this reference is already recorded. Reject or correct that payment first."}}.Err()
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		payment, err := q.InsertBankNotificationPayment(ctx, dbq.InsertBankNotificationPaymentParams{StudentID: student, AmountCents: t.AmountCents,
			PaidOn: dateOf(t.SentAt), Reference: t.TransactionReference, PayerName: t.CustomerName, Note: "Assigned by Finance: " + note,
			RecordedBy: &p.UserID})
		if err != nil {
			return err
		}
		if _, err := finance.ConfirmByFinance(ctx, q, payment, p.UserID, "Found on the KCB statement; reference did not match.", s.now()); err != nil {
			return err
		}
		if err := q.SettleKcbTransaction(ctx, dbq.SettleKcbTransactionParams{ID: id, Status: "assigned", StudentID: &student,
			PaymentID: &payment, HandledBy: &p.UserID, Note: note}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "finance.kcb_assigned", EntityType: "kcb_transaction", EntityID: id.String(),
			Details: map[string]any{"student_id": student.String(), "payment_id": payment.String(), "amount_cents": t.AmountCents}})
	})
}

// Refund marks an unmatched notification to be returned to the payer (done
// outside the system).
func (s *Service) Refund(ctx context.Context, p *auth.Principal, id uuid.UUID, note string, req audit.Request) error {
	note = strings.TrimSpace(note)
	if n := len([]rune(note)); n < 5 || n > 1000 {
		return content.Problems{{Field: "note", Message: "Say why it is being returned, in 5 to 1,000 characters."}}.Err()
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		t, err := q.LockKcbTransaction(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if t.Status != "unmatched" {
			return ErrDecided
		}
		if err := q.SettleKcbTransaction(ctx, dbq.SettleKcbTransactionParams{ID: id, Status: "refund", HandledBy: &p.UserID, Note: note}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "finance.kcb_refund", EntityType: "kcb_transaction", EntityID: id.String(),
			Details: map[string]any{"amount_cents": t.AmountCents}})
	})
}

type DayLine struct {
	Status     string `json:"status"`
	Count      int64  `json:"count"`
	TotalCents int64  `json:"total_cents"`
}

type Reconciliation struct {
	Day            string    `json:"day"`
	Lines          []DayLine `json:"lines"`
	ReceivedCount  int64     `json:"received_count"`
	ReceivedCents  int64     `json:"received_cents"`
	CreditedCents  int64     `json:"credited_cents"`
	UnmatchedCents int64     `json:"unmatched_cents"`
}

// Reconcile is one day's KCB notifications, to compare with the statement.
func (s *Service) Reconcile(ctx context.Context, day time.Time) (*Reconciliation, error) {
	rows, err := dbq.New(s.pool).KcbDaySummary(ctx, day)
	if err != nil {
		return nil, err
	}
	out := &Reconciliation{Day: day.Format(time.DateOnly), Lines: []DayLine{}}
	for _, r := range rows {
		out.Lines = append(out.Lines, DayLine{Status: r.Status, Count: r.N, TotalCents: r.TotalCents})
		if r.Status == "rejected" {
			continue
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
	return out, nil
}

// Reprocess records stored notifications that were not processed.
func (s *Service) Reprocess(ctx context.Context) (int, error) {
	rows, err := dbq.New(s.pool).UnprocessedKcbNotifications(ctx)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, r := range rows {
		if err := s.process(ctx, r.ID, r.Body); err != nil {
			continue
		}
		done++
	}
	return done, nil
}
