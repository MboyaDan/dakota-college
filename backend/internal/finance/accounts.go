package finance

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
)

// A student's account (AC-10): invoices, payments and adjustments, and the
// statement and balance derived from them. Nothing here is stored.

type StudentRef struct {
	ID             uuid.UUID `json:"id"`
	StudentNumber  string    `json:"student_number"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	ProgrammeTitle string    `json:"programme_title"`
	IntakeName     *string   `json:"intake_name"`
}

// Summary is where an account stands. Balance is positive when the
// student owes and negative when they are in credit. Pending payments are
// shown but do not count until confirmed.
type Summary struct {
	BalanceCents  int64   `json:"balance_cents"`
	InvoicedCents int64   `json:"invoiced_cents"`
	ChargesCents  int64   `json:"charges_cents"`
	PaidCents     int64   `json:"paid_cents"`
	CreditsCents  int64   `json:"credits_cents"`
	PendingCents  int64   `json:"pending_cents"`
	OverdueCents  int64   `json:"overdue_cents"`
	NextDueOn     *string `json:"next_due_on"`
	NextDueCents  int64   `json:"next_due_cents"`
}

type Adjustment struct {
	ID            uuid.UUID  `json:"id"`
	Kind          string     `json:"kind"`
	AmountCents   int64      `json:"amount_cents"`
	Reason        string     `json:"reason"`
	InvoiceID     *uuid.UUID `json:"invoice_id"`
	InvoiceNumber *string    `json:"invoice_number"`
	PaymentID     *uuid.UUID `json:"payment_id"`
	ReceiptNumber *string    `json:"receipt_number"`
	CreatedAt     time.Time  `json:"created_at"`
	CreatedByName string     `json:"created_by_name"`
}

type Account struct {
	Student     StudentRef       `json:"student"`
	Summary     Summary          `json:"summary"`
	Statement   []Entry          `json:"statement"`
	Invoices    []InvoiceSummary `json:"invoices"`
	Payments    []Payment        `json:"payments"`
	Adjustments []Adjustment     `json:"adjustments"`
}

var methodNames = map[string]string{"cash": "Cash", "bank": "Bank", "mpesa": "M-Pesa", "sponsor": "Sponsor"}

// invoiceFor says what an invoice is for.
func invoiceFor(kind string, term *string) string {
	switch {
	case kind == "enrolment":
		return "First fees, at enrolment"
	case term != nil:
		return *term
	default:
		return "Other fees"
	}
}

func invoiceDescription(kind string, term *string) string {
	return "Invoice: " + invoiceFor(kind, term)
}

func paymentDescription(method, reference, payer string) string {
	d := "Payment, " + methodNames[method]
	if reference != "" {
		d += " " + reference
	}
	if method == "sponsor" && payer != "" {
		d += " from " + payer
	}
	return d
}

// account reads everything about one student's fees, in one snapshot.
func (s *Service) account(ctx context.Context, q *dbq.Queries, st StudentRef, staff bool) (*Account, error) {
	out := &Account{Student: st, Statement: []Entry{}, Invoices: []InvoiceSummary{}, Payments: []Payment{}, Adjustments: []Adjustment{}}

	invoices, err := q.StudentInvoices(ctx, dbq.StudentInvoicesParams{StudentID: st.ID, IssuedOnly: !staff})
	if err != nil {
		return nil, err
	}
	for _, r := range invoices {
		out.Invoices = append(out.Invoices, InvoiceSummary{ID: r.ID, StudentID: st.ID, Kind: r.Kind, Status: r.Status, Number: r.Number,
			TermName: r.TermName, TotalCents: r.TotalCents, CreatedAt: r.CreatedAt, IssuedAt: r.IssuedAt})
		if r.Status == "issued" {
			out.Statement = append(out.Statement, NewEntry("invoice", invoiceDescription(r.Kind, r.TermName), *r.Number, r.TotalCents, 0, dateOf(*r.IssuedAt), *r.IssuedAt))
		}
	}

	payments, err := q.StudentPayments(ctx, st.ID)
	if err != nil {
		return nil, err
	}
	for _, r := range payments {
		if !staff && r.Status == "rejected" {
			continue
		}
		pm := Payment{ID: r.ID, StudentID: st.ID, Method: r.Method, AmountCents: r.AmountCents, PaidOn: r.PaidOn.Format(time.DateOnly),
			Reference: r.Reference, PayerName: r.PayerName, Status: r.Status, RecordedAt: r.RecordedAt, DecidedAt: r.DecidedAt,
			ReceiptID: r.ReceiptID, ReceiptNumber: r.ReceiptNumber}
		if staff {
			pm.DecisionNote = r.DecisionNote
		}
		out.Payments = append(out.Payments, pm)
		if r.Status == "confirmed" {
			ref := ""
			if r.ReceiptNumber != nil {
				ref = *r.ReceiptNumber
			}
			at := r.RecordedAt
			if r.DecidedAt != nil {
				at = *r.DecidedAt
			}
			out.Statement = append(out.Statement, NewEntry("payment", paymentDescription(r.Method, r.Reference, r.PayerName), ref, 0, r.AmountCents, r.PaidOn, at))
		}
	}

	adjustments, err := q.StudentAdjustments(ctx, st.ID)
	if err != nil {
		return nil, err
	}
	for _, r := range adjustments {
		out.Adjustments = append(out.Adjustments, Adjustment{ID: r.ID, Kind: r.Kind, AmountCents: r.AmountCents, Reason: r.Reason,
			InvoiceID: r.InvoiceID, InvoiceNumber: r.InvoiceNumber, PaymentID: r.PaymentID, ReceiptNumber: r.ReceiptNumber,
			CreatedAt: r.CreatedAt, CreatedByName: r.CreatedByName})
		debit, credit := r.AmountCents, int64(0)
		desc := "Charge: " + r.Reason
		if r.Kind == "credit" {
			debit, credit = 0, r.AmountCents
			desc = "Credit: " + r.Reason
		}
		ref := ""
		if r.InvoiceNumber != nil {
			ref = *r.InvoiceNumber
		} else if r.ReceiptNumber != nil {
			ref = *r.ReceiptNumber
		}
		out.Statement = append(out.Statement, NewEntry(r.Kind, desc, ref, debit, credit, dateOf(r.CreatedAt), r.CreatedAt))
	}

	var closing int64
	out.Statement, closing = Run(out.Statement)

	// The balance again, summed by the database from the same records.
	b, err := q.StudentBalance(ctx, st.ID)
	if err != nil {
		return nil, err
	}
	balance := b.InvoicedCents + b.ChargesCents - b.PaidCents - b.CreditsCents
	if balance != closing {
		// Cannot happen while both read one snapshot; refuse to show a wrong statement.
		return nil, errors.New("statement does not reconcile with the balance")
	}
	dues, err := q.StudentDueInstalments(ctx, st.ID)
	if err != nil {
		return nil, err
	}
	schedule := make([]Due, 0, len(dues))
	for _, d := range dues {
		if d.DueOn != nil && d.AmountCents != nil {
			schedule = append(schedule, Due{On: *d.DueOn, Amount: *d.AmountCents})
		}
	}
	overdue, nextOn, nextAmount := Schedule(schedule, b.ChargesCents, b.PaidCents+b.CreditsCents, s.today())
	out.Summary = Summary{BalanceCents: balance, InvoicedCents: b.InvoicedCents, ChargesCents: b.ChargesCents, PaidCents: b.PaidCents,
		CreditsCents: b.CreditsCents, PendingCents: b.PendingCents, OverdueCents: overdue, NextDueOn: dateString(nextOn), NextDueCents: nextAmount}
	return out, nil
}

// snapshot runs fn in a read-only, repeatable-read transaction, so every
// sum on a statement comes from the same moment.
func (s *Service) snapshot(ctx context.Context, fn func(q *dbq.Queries) error) error {
	return pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		return fn(dbq.New(tx))
	})
}

func refOf(r dbq.FinanceStudentRow) StudentRef {
	return StudentRef{ID: r.ID, StudentNumber: r.StudentNumber, Name: fullName(r.FirstName, r.MiddleName, r.LastName), Status: r.Status,
		ProgrammeTitle: r.ProgrammeTitle, IntakeName: r.IntakeName}
}

// Account is a student's account for finance staff.
func (s *Service) Account(ctx context.Context, student uuid.UUID) (*Account, error) {
	var out *Account
	err := s.snapshot(ctx, func(q *dbq.Queries) error {
		st, err := q.FinanceStudent(ctx, student)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		out, err = s.account(ctx, q, refOf(st), true)
		return err
	})
	return out, err
}

type AdjustmentInput struct {
	Kind        string     `json:"kind"`
	AmountCents int64      `json:"amount_cents"`
	Reason      string     `json:"reason"`
	InvoiceID   *uuid.UUID `json:"invoice_id"`
	PaymentID   *uuid.UUID `json:"payment_id"`
}

// Adjust records a compensating entry: a charge (owes more) or a credit
// (owes less), always with a reason. It never edits what it corrects.
func (s *Service) Adjust(ctx context.Context, p *auth.Principal, key string, student uuid.UUID, in AdjustmentInput, req audit.Request) (*Answer, error) {
	in.Reason = strings.Join(strings.Fields(in.Reason), " ")
	var pr content.Problems
	if in.Kind != "charge" && in.Kind != "credit" {
		pr.Add("kind", "Choose a charge (the student owes more) or a credit (the student owes less).")
	}
	if in.AmountCents < 1 || in.AmountCents > MaxAmountCents {
		pr.Add("amount_cents", "Enter an amount above KES 0.00 and up to KES 1,000,000,000.00.")
	}
	if n := len([]rune(in.Reason)); n < 5 || n > 1000 {
		pr.Add("reason", "Say why, in 5 to 1,000 characters. The student sees this on their statement.")
	}
	if in.InvoiceID != nil && in.PaymentID != nil {
		pr.Add("invoice_id", "Link the adjustment to an invoice or a payment, not both.")
	}
	if err := pr.Err(); err != nil {
		return nil, err
	}
	request := map[string]any{"student": student, "input": in}
	return s.keyed(ctx, "finance.adjust:"+p.UserID.String(), key, request, func(q *dbq.Queries) (any, error) {
		if _, err := q.FinanceStudent(ctx, student); errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		} else if err != nil {
			return nil, err
		}
		if in.InvoiceID != nil {
			inv, err := q.LockInvoice(ctx, *in.InvoiceID)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && (inv.StudentID != student || inv.Status != "issued")) {
				return nil, content.Problems{{Field: "invoice_id", Message: "Choose an issued invoice of this student."}}.Err()
			}
			if err != nil {
				return nil, err
			}
		}
		if in.PaymentID != nil {
			pm, err := q.LockPayment(ctx, *in.PaymentID)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && (pm.StudentID != student || pm.Status != "confirmed")) {
				return nil, content.Problems{{Field: "payment_id", Message: "Choose a confirmed payment of this student."}}.Err()
			}
			if err != nil {
				return nil, err
			}
		}
		id, err := q.InsertAdjustment(ctx, dbq.InsertAdjustmentParams{StudentID: student, Kind: in.Kind, AmountCents: in.AmountCents, Reason: in.Reason,
			InvoiceID: in.InvoiceID, PaymentID: in.PaymentID, CreatedBy: p.UserID})
		if err != nil {
			return nil, err
		}
		if err := audit.Write(ctx, q, req, audit.Entry{Action: "finance.adjustment", EntityType: "student", EntityID: student.String(),
			Details: map[string]any{"adjustment_id": id.String(), "kind": in.Kind, "amount_cents": in.AmountCents}}); err != nil {
			return nil, err
		}
		// The account itself is read afterwards, from a snapshot.
		return Adjustment{ID: id, Kind: in.Kind, AmountCents: in.AmountCents, Reason: in.Reason, InvoiceID: in.InvoiceID,
			PaymentID: in.PaymentID, CreatedAt: s.now(), CreatedByName: p.FullName}, nil
	})
}

type ArrearsRow struct {
	Student      StudentRef `json:"student"`
	CohortName   *string    `json:"cohort_name"`
	BalanceCents int64      `json:"balance_cents"`
	OverdueCents int64      `json:"overdue_cents"`
}

// Arrears lists students who owe money, most overdue first.
func (s *Service) Arrears(ctx context.Context, cohort *uuid.UUID, overdueOnly bool) ([]ArrearsRow, error) {
	rows, err := dbq.New(s.pool).ArrearsReport(ctx, dbq.ArrearsReportParams{Today: s.today(), CohortID: cohort, OverdueOnly: overdueOnly})
	if err != nil {
		return nil, err
	}
	out := make([]ArrearsRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, ArrearsRow{Student: StudentRef{ID: r.ID, StudentNumber: r.StudentNumber, Name: fullName(r.FirstName, r.MiddleName, r.LastName),
			Status: r.Status, ProgrammeTitle: r.ProgrammeTitle}, CohortName: r.CohortName, BalanceCents: r.BalanceCents, OverdueCents: r.OverdueCents})
	}
	return out, nil
}

type FinanceSummary struct {
	DraftInvoices       int64 `json:"draft_invoices"`
	PendingPayments     int64 `json:"pending_payments"`
	PendingCents        int64 `json:"pending_cents"`
	NetOutstandingCents int64 `json:"net_outstanding_cents"`
}

func (s *Service) Summary(ctx context.Context) (*FinanceSummary, error) {
	r, err := dbq.New(s.pool).FinanceSummary(ctx)
	if err != nil {
		return nil, err
	}
	return &FinanceSummary{DraftInvoices: r.DraftInvoices, PendingPayments: r.PendingPayments, PendingCents: r.PendingCents, NetOutstandingCents: r.NetOutstandingCents}, nil
}

// ---------- the student's own fees ----------

type StudentInvoice struct {
	InvoiceSummary
	Lines       []Line              `json:"lines"`
	Instalments []InvoiceInstalment `json:"instalments"`
}

type StudentFees struct {
	Summary   Summary          `json:"summary"`
	Statement []Entry          `json:"statement"`
	Invoices  []StudentInvoice `json:"invoices"`
	Payments  []Payment        `json:"payments"`
	// The fee structure for the student's programme and intake, if set.
	Structure *Structure `json:"structure"`
	// How to pay, as the college wrote it; empty when not set (DQ-09).
	PaymentInstructions string `json:"payment_instructions"`
}

func (s *Service) ownStudent(ctx context.Context, q *dbq.Queries, user uuid.UUID) (dbq.FinanceStudentRow, error) {
	r, err := q.FinanceStudentByUser(ctx, user)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbq.FinanceStudentRow{}, ErrNoStudent
	}
	return dbq.FinanceStudentRow(r), err
}

// MyFees is the signed-in student's own account: issued invoices, their
// payments (not rejected ones), statement and the fee structure.
func (s *Service) MyFees(ctx context.Context, user uuid.UUID) (*StudentFees, error) {
	var out *StudentFees
	err := s.snapshot(ctx, func(q *dbq.Queries) error {
		st, err := s.ownStudent(ctx, q, user)
		if err != nil {
			return err
		}
		acc, err := s.account(ctx, q, refOf(st), false)
		if err != nil {
			return err
		}
		out = &StudentFees{Summary: acc.Summary, Statement: acc.Statement, Payments: acc.Payments, Invoices: []StudentInvoice{}}
		for _, inv := range acc.Invoices {
			si := StudentInvoice{InvoiceSummary: inv}
			if si.Lines, si.Instalments, err = invoiceParts(ctx, q, inv.ID); err != nil {
				return err
			}
			out.Invoices = append(out.Invoices, si)
		}
		if id, ok, err := structureFor(ctx, q, st.ProgrammeID, st.IntakeID); err != nil {
			return err
		} else if ok {
			if out.Structure, err = getStructure(ctx, q, id); err != nil {
				return err
			}
			out.Structure.UpdatedByName, out.Structure.Note = nil, ""
		}
		settings, err := q.GetSiteSettings(ctx, []string{"finance.payment_instructions"})
		if err != nil {
			return err
		}
		if len(settings) == 1 {
			out.PaymentInstructions = strings.TrimSpace(settings[0].Value)
		}
		return nil
	})
	return out, err
}
