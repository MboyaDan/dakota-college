package finance

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// Payments (brief 9, AC-11, D-159). Finance records what was received.
// Cash taken at the office by someone who may confirm payments is confirmed
// and receipted at once. Bank, M-Pesa and sponsor payments wait until a
// finance officer has checked them against the statement; until then they
// do not count. Every write takes an Idempotency-Key (NN-04).

type Payment struct {
	ID             uuid.UUID  `json:"id"`
	StudentID      uuid.UUID  `json:"student_id"`
	StudentNumber  string     `json:"student_number,omitempty"`
	StudentName    string     `json:"student_name,omitempty"`
	Method         string     `json:"method"`
	AmountCents    int64      `json:"amount_cents"`
	PaidOn         string     `json:"paid_on"`
	Reference      string     `json:"reference"`
	PayerName      string     `json:"payer_name"`
	Note           string     `json:"note,omitempty"`
	Status         string     `json:"status"`
	ConfirmedVia   *string    `json:"confirmed_via,omitempty"`
	RecordedAt     time.Time  `json:"recorded_at"`
	RecordedByName string     `json:"recorded_by_name,omitempty"`
	DecidedAt      *time.Time `json:"decided_at"`
	DecidedByName  *string    `json:"decided_by_name,omitempty"`
	DecisionNote   string     `json:"decision_note,omitempty"`
	ReceiptID      *uuid.UUID `json:"receipt_id"`
	ReceiptNumber  *string    `json:"receipt_number"`
}

func getPayment(ctx context.Context, q *dbq.Queries, id uuid.UUID) (*Payment, error) {
	r, err := q.GetPayment(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &Payment{ID: r.ID, StudentID: r.StudentID, StudentNumber: r.StudentNumber, StudentName: fullName(r.FirstName, r.MiddleName, r.LastName),
		Method: r.Method, AmountCents: r.AmountCents, PaidOn: r.PaidOn.Format(time.DateOnly), Reference: r.Reference, PayerName: r.PayerName,
		Note: r.Note, Status: r.Status, ConfirmedVia: r.ConfirmedVia, RecordedAt: r.RecordedAt, RecordedByName: r.RecordedByName,
		DecidedAt: r.DecidedAt, DecidedByName: r.DecidedByName, DecisionNote: r.DecisionNote, ReceiptID: r.ReceiptID, ReceiptNumber: r.ReceiptNumber}, nil
}

func (s *Service) GetPayment(ctx context.Context, id uuid.UUID) (*Payment, error) {
	return getPayment(ctx, dbq.New(s.pool), id)
}

type PaymentFilter struct {
	Status *string
	Method *string
	Query  string
	Page   int
}

func (s *Service) ListPayments(ctx context.Context, f PaymentFilter) ([]Payment, int64, error) {
	rows, err := dbq.New(s.pool).ListPayments(ctx, dbq.ListPaymentsParams{Status: f.Status, Method: f.Method, Q: likePattern(f.Query), Lim: pageSize, Off: offset(f.Page)})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Payment, 0, len(rows))
	var total int64
	for _, r := range rows {
		total = r.TotalCount
		out = append(out, Payment{ID: r.ID, StudentID: r.StudentID, StudentNumber: r.StudentNumber, StudentName: fullName(r.FirstName, r.MiddleName, r.LastName),
			Method: r.Method, AmountCents: r.AmountCents, PaidOn: r.PaidOn.Format(time.DateOnly), Reference: r.Reference, PayerName: r.PayerName,
			Status: r.Status, RecordedAt: r.RecordedAt, RecordedByName: r.RecordedByName, ReceiptNumber: r.ReceiptNumber})
	}
	return out, total, nil
}

type PaymentInput struct {
	StudentID   uuid.UUID `json:"student_id"`
	Method      string    `json:"method"`
	AmountCents int64     `json:"amount_cents"`
	PaidOn      string    `json:"paid_on"`
	Reference   string    `json:"reference"`
	PayerName   string    `json:"payer_name"`
	Note        string    `json:"note"`
}

var mpesaCode = regexp.MustCompile(`^[A-Z0-9]{10}$`)

// check validates a payment and tidies its text.
func (in *PaymentInput) check(today time.Time) (time.Time, error) {
	var pr content.Problems
	switch in.Method {
	case "cash", "bank", "mpesa", "sponsor":
	default:
		pr.Add("method", "Choose cash, bank, M-Pesa or sponsor.")
	}
	if in.AmountCents < 1 || in.AmountCents > MaxAmountCents {
		pr.Add("amount_cents", "Enter an amount above KES 0.00 and up to KES 1,000,000,000.00.")
	}
	paid, err := time.Parse(time.DateOnly, strings.TrimSpace(in.PaidOn))
	switch {
	case err != nil:
		pr.Add("paid_on", "Enter the date the money was paid.")
	case paid.After(today):
		pr.Add("paid_on", "The date cannot be in the future.")
	case paid.Before(today.AddDate(-1, 0, 0)):
		pr.Add("paid_on", "Payments more than a year old cannot be recorded here. Ask the ICT office.")
	}
	in.Reference = strings.Join(strings.Fields(in.Reference), " ")
	in.PayerName = strings.Join(strings.Fields(in.PayerName), " ")
	in.Note = strings.TrimSpace(in.Note)
	switch in.Method {
	case "mpesa":
		in.Reference = strings.ToUpper(strings.ReplaceAll(in.Reference, " ", ""))
		if !mpesaCode.MatchString(in.Reference) {
			pr.Add("reference", "Enter the 10-character M-Pesa transaction code, letters and numbers only.")
		}
	case "bank":
		if in.Reference == "" {
			pr.Add("reference", "Enter the bank slip number or transfer reference.")
		}
	case "sponsor":
		if in.PayerName == "" {
			pr.Add("payer_name", "Enter the name of the sponsor who paid.")
		}
	}
	if len([]rune(in.Reference)) > 60 {
		pr.Add("reference", "Use 60 characters or fewer.")
	}
	if len([]rune(in.PayerName)) > 120 {
		pr.Add("payer_name", "Use 120 characters or fewer.")
	}
	if len([]rune(in.Note)) > 1000 {
		pr.Add("note", "Use 1,000 characters or fewer.")
	}
	return paid, pr.Err()
}

// receipt confirms a pending payment and numbers its receipt
// DTC/RCT/YYYY/NNNNN, in the caller's transaction.
func (s *Service) receipt(ctx context.Context, q *dbq.Queries, p *auth.Principal, id uuid.UUID, note string, req audit.Request) error {
	number, err := confirm(ctx, q, id, "finance", &p.UserID, note, s.now())
	if err != nil {
		return err
	}
	return audit.Write(ctx, q, req, audit.Entry{Action: "finance.payment_confirmed", EntityType: "payment", EntityID: id.String(),
		Details: map[string]any{"receipt": number}})
}

// confirm marks a pending payment confirmed and issues its receipt, in the
// caller's transaction, and returns the receipt number.
func confirm(ctx context.Context, q *dbq.Queries, id uuid.UUID, via string, by *uuid.UUID, note string, now time.Time) (string, error) {
	if n, err := q.DecidePayment(ctx, dbq.DecidePaymentParams{ID: id, Status: "confirmed", ConfirmedVia: &via, DecidedBy: by, DecisionNote: note}); err != nil || n != 1 {
		if err == nil {
			err = ErrDecided
		}
		return "", err
	}
	year := now.In(nairobi).Year()
	n, err := q.NextCounterValue(ctx, dbq.NextCounterValueParams{Name: "receipt", Period: int32(year)})
	if err != nil {
		return "", err
	}
	number := fmt.Sprintf("DTC/RCT/%d/%05d", year, n)
	if _, err := q.InsertReceipt(ctx, dbq.InsertReceiptParams{Number: number, PaymentID: id, IssuedBy: by}); err != nil {
		return "", err
	}
	return number, nil
}

// ConfirmByFinance confirms a pending payment as a finance officer who has
// checked it against the statement, in the caller's transaction.
func ConfirmByFinance(ctx context.Context, q *dbq.Queries, id uuid.UUID, by uuid.UUID, note string, now time.Time) (string, error) {
	return confirm(ctx, q, id, "finance", &by, note, now)
}

// ConfirmByProvider confirms a pending payment from the verified M-Pesa
// flow (AC-11) and issues its receipt, in the caller's transaction. by is
// the member of staff who assigned it, or nil when it matched by itself.
func ConfirmByProvider(ctx context.Context, q *dbq.Queries, id uuid.UUID, by *uuid.UUID, note string, now time.Time) (string, error) {
	return confirm(ctx, q, id, "provider", by, note, now)
}

// RecordPayment records money received for a student.
func (s *Service) RecordPayment(ctx context.Context, p *auth.Principal, key string, in PaymentInput, req audit.Request) (*Answer, error) {
	paid, err := in.check(s.today())
	if err != nil {
		return nil, err
	}
	return s.keyed(ctx, "finance.payment.record:"+p.UserID.String(), key, in, func(q *dbq.Queries) (any, error) {
		if _, err := q.FinanceStudent(ctx, in.StudentID); errors.Is(err, pgx.ErrNoRows) {
			return nil, content.Problems{{Field: "student_id", Message: "Choose a student."}}.Err()
		} else if err != nil {
			return nil, err
		}
		if in.Method == "mpesa" {
			used, err := q.MpesaReferenceUsed(ctx, in.Reference)
			if err != nil {
				return nil, err
			}
			if used {
				return nil, content.Problems{{Field: "reference", Message: "This M-Pesa code is already recorded. Search payments for it before recording it again."}}.Err()
			}
		}
		id, err := q.InsertPayment(ctx, dbq.InsertPaymentParams{StudentID: in.StudentID, Method: in.Method, AmountCents: in.AmountCents, PaidOn: paid,
			Reference: in.Reference, PayerName: in.PayerName, Note: in.Note, RecordedBy: &p.UserID})
		if isUniqueViolation(err) {
			return nil, content.Problems{{Field: "reference", Message: "This M-Pesa code is already recorded. Search payments for it before recording it again."}}.Err()
		}
		if err != nil {
			return nil, err
		}
		if err := audit.Write(ctx, q, req, audit.Entry{Action: "finance.payment_recorded", EntityType: "payment", EntityID: id.String(),
			Details: map[string]any{"student_id": in.StudentID.String(), "method": in.Method, "amount_cents": in.AmountCents}}); err != nil {
			return nil, err
		}
		// Cash counted at the office is receipted at once (owner's choice).
		if in.Method == "cash" && p.Has(PermConfirm) {
			if err := s.receipt(ctx, q, p, id, "Cash received at the finance office.", req); err != nil {
				return nil, err
			}
		}
		return getPayment(ctx, q, id)
	})
}

// ConfirmPayment confirms a pending payment after checking it against the
// bank or M-Pesa statement, and issues its receipt.
func (s *Service) ConfirmPayment(ctx context.Context, p *auth.Principal, key string, id uuid.UUID, note string, req audit.Request) (*Answer, error) {
	note = tidy(note, 1000)
	return s.keyed(ctx, "finance.payment.confirm:"+p.UserID.String(), key, map[string]any{"id": id, "note": note}, func(q *dbq.Queries) (any, error) {
		pm, err := q.LockPayment(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		if pm.Status != "pending" {
			return nil, ErrDecided
		}
		if note == "" {
			note = "Checked against the statement."
		}
		if err := s.receipt(ctx, q, p, id, note, req); err != nil {
			return nil, err
		}
		return getPayment(ctx, q, id)
	})
}

// RejectPayment refuses a pending payment that cannot be found on the
// statement. It never counts; the reason is kept.
func (s *Service) RejectPayment(ctx context.Context, p *auth.Principal, key string, id uuid.UUID, reason string, req audit.Request) (*Answer, error) {
	reason = tidy(reason, 1000)
	if len([]rune(reason)) < 5 {
		return nil, content.Problems{{Field: "reason", Message: "Say why this payment is rejected (at least 5 characters)."}}.Err()
	}
	return s.keyed(ctx, "finance.payment.reject:"+p.UserID.String(), key, map[string]any{"id": id, "reason": reason}, func(q *dbq.Queries) (any, error) {
		pm, err := q.LockPayment(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		if pm.Status != "pending" {
			return nil, ErrDecided
		}
		if n, err := q.DecidePayment(ctx, dbq.DecidePaymentParams{ID: id, Status: "rejected", DecidedBy: &p.UserID, DecisionNote: reason}); err != nil || n != 1 {
			if err == nil {
				err = ErrDecided
			}
			return nil, err
		}
		if err := audit.Write(ctx, q, req, audit.Entry{Action: "finance.payment_rejected", EntityType: "payment", EntityID: id.String(),
			Details: map[string]any{"reason": reason}}); err != nil {
			return nil, err
		}
		return getPayment(ctx, q, id)
	})
}
