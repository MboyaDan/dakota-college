package finance

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/letters"
)

// PDFs of receipts, invoices and statements, drawn from records each time
// they are asked for. A receipt exists only for a confirmed payment, and an
// invoice PDF only once it is issued. student, when set, limits a document
// to that student's own (the student portal); nil is finance staff.

var categoryNames = map[string]string{"tuition": "Tuition", "registration": "Registration", "assessment": "Assessment", "caution": "Caution", "other": "Other"}

func fileName(prefix, number string) string {
	return prefix + " " + strings.ReplaceAll(number, "/", "-") + ".pdf"
}

func (s *Service) ownerOf(ctx context.Context, q *dbq.Queries, user *uuid.UUID) (*uuid.UUID, error) {
	if user == nil {
		return nil, nil
	}
	st, err := s.ownStudent(ctx, q, *user)
	if err != nil {
		return nil, err
	}
	return &st.ID, nil
}

// ReceiptPDF is the receipt of a confirmed payment.
func (s *Service) ReceiptPDF(ctx context.Context, payment uuid.UUID, user *uuid.UUID) ([]byte, string, error) {
	q := dbq.New(s.pool)
	owner, err := s.ownerOf(ctx, q, user)
	if err != nil {
		return nil, "", err
	}
	r, err := q.GetPayment(ctx, payment)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (r.ReceiptNumber == nil || (owner != nil && r.StudentID != *owner))) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	details := [][2]string{
		{"Receipt number", *r.ReceiptNumber},
		{"Received from", fullName(r.FirstName, r.MiddleName, r.LastName)},
		{"Student number", r.StudentNumber},
		{"Programme", r.ProgrammeTitle},
		{"Paid on", r.PaidOn.Format("2 January 2006")},
		{"Method", methodNames[r.Method]},
	}
	if r.Reference != "" {
		details = append(details, [2]string{"Reference", r.Reference})
	}
	if r.Method == "sponsor" && r.PayerName != "" {
		details = append(details, [2]string{"Paid by", r.PayerName})
	}
	doc := letters.Ledger{Title: "Receipt", Reference: *r.ReceiptNumber, Date: r.ReceiptIssuedAt.In(nairobi), Details: details,
		Totals: [][2]string{{"Amount received", letters.FormatKES(r.AmountCents)}},
		Notes:  "Received with thanks. This receipt was issued by the finance office after the payment was confirmed. Keep it for your records.",
		Sample: isSample(r.StudentNumber)}
	pdf, err := letters.LedgerDocument(doc)
	return pdf, fileName("Receipt", *r.ReceiptNumber), err
}

// InvoicePDF is an issued invoice with its lines and instalments.
func (s *Service) InvoicePDF(ctx context.Context, invoice uuid.UUID, user *uuid.UUID) ([]byte, string, error) {
	q := dbq.New(s.pool)
	owner, err := s.ownerOf(ctx, q, user)
	if err != nil {
		return nil, "", err
	}
	r, err := q.GetInvoice(ctx, invoice)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (r.Status != "issued" || (owner != nil && r.StudentID != *owner))) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	lines, plan, err := invoiceParts(ctx, q, invoice)
	if err != nil {
		return nil, "", err
	}
	details := [][2]string{
		{"Invoice number", *r.Number},
		{"Billed to", fullName(r.FirstName, r.MiddleName, r.LastName)},
		{"Student number", r.StudentNumber},
		{"Programme", r.ProgrammeTitle},
		{"For", invoiceFor(r.Kind, r.TermName)},
	}
	doc := letters.Ledger{Title: "Invoice", Reference: *r.Number, Date: r.IssuedAt.In(nairobi), Details: details,
		Columns: []letters.LedgerColumn{{Label: "Item", Width: 0.6}, {Label: "Type", Width: 0.2}, {Label: "Amount", Width: 0.2, Right: true}},
		Totals:  [][2]string{{"Total", letters.FormatKES(r.TotalCents)}},
		Sample:  isSample(r.StudentNumber)}
	for _, l := range lines {
		doc.Rows = append(doc.Rows, letters.LedgerRow{Cells: []string{l.Label, categoryNames[l.Category], letters.FormatKES(l.AmountCents)}})
	}
	var due []string
	for _, n := range plan {
		if n.DueOn != nil && n.AmountCents != nil {
			d, _ := time.Parse(time.DateOnly, *n.DueOn)
			due = append(due, letters.FormatKES(*n.AmountCents)+" by "+d.Format("2 January 2006"))
		}
	}
	doc.Notes = "Due: " + strings.Join(due, "; ") + ".\n\nYour fee statement shows what you have paid and what you still owe. A payment counts once the finance office has confirmed it."
	pdf, err := letters.LedgerDocument(doc)
	return pdf, fileName("Invoice", *r.Number), err
}

// StatementPDF is a student's fee statement as of now.
func (s *Service) StatementPDF(ctx context.Context, student uuid.UUID, user *uuid.UUID) ([]byte, string, error) {
	var acc *Account
	err := s.snapshot(ctx, func(q *dbq.Queries) error {
		var st dbq.FinanceStudentRow
		var err error
		if user != nil {
			st, err = s.ownStudent(ctx, q, *user)
		} else {
			st, err = q.FinanceStudent(ctx, student)
			if errors.Is(err, pgx.ErrNoRows) {
				err = ErrNotFound
			}
		}
		if err != nil {
			return err
		}
		acc, err = s.account(ctx, q, refOf(st), false)
		return err
	})
	if err != nil {
		return nil, "", err
	}
	now := s.now().In(nairobi)
	doc := letters.Ledger{Title: "Fee statement", Reference: acc.Student.StudentNumber, Date: now,
		Details: [][2]string{{"Name", acc.Student.Name}, {"Student number", acc.Student.StudentNumber}, {"Programme", acc.Student.ProgrammeTitle},
			{"As of", now.Format("2 January 2006, 15:04")}},
		// Details come first because the first column is the one that wraps.
		Columns: []letters.LedgerColumn{{Label: "Details", Width: 0.4}, {Label: "Date", Width: 0.15}, {Label: "Charged", Width: 0.15, Right: true},
			{Label: "Paid", Width: 0.15, Right: true}, {Label: "Balance", Width: 0.15, Right: true}},
		Empty:  "Nothing has been charged or paid yet.",
		Sample: isSample(acc.Student.StudentNumber)}
	money := func(c int64) string {
		if c == 0 {
			return ""
		}
		return letters.FormatKES(c)
	}
	for _, e := range acc.Statement {
		d, _ := time.Parse(time.DateOnly, e.Date)
		details := e.Description
		if e.Reference != "" && !strings.Contains(details, e.Reference) {
			details += " (" + e.Reference + ")"
		}
		doc.Rows = append(doc.Rows, letters.LedgerRow{Cells: []string{details, d.Format("2 Jan 2006"), money(e.DebitCents), money(e.CreditCents), letters.FormatKES(e.BalanceCents)}})
	}
	sm := acc.Summary
	doc.Totals = [][2]string{{"Total charged", letters.FormatKES(sm.InvoicedCents + sm.ChargesCents)}, {"Total paid and credited", letters.FormatKES(sm.PaidCents + sm.CreditsCents)}}
	switch {
	case sm.BalanceCents > 0:
		doc.Totals = append(doc.Totals, [2]string{"Balance owed", letters.FormatKES(sm.BalanceCents)})
	case sm.BalanceCents < 0:
		doc.Totals = append(doc.Totals, [2]string{"In credit", letters.FormatKES(-sm.BalanceCents)})
	default:
		doc.Totals = append(doc.Totals, [2]string{"Balance", letters.FormatKES(0)})
	}
	notes := []string{}
	if sm.OverdueCents > 0 {
		notes = append(notes, "Overdue now: "+letters.FormatKES(sm.OverdueCents)+".")
	}
	if sm.NextDueOn != nil {
		d, _ := time.Parse(time.DateOnly, *sm.NextDueOn)
		notes = append(notes, "Next due: "+letters.FormatKES(sm.NextDueCents)+" by "+d.Format("2 January 2006")+".")
	}
	if sm.PendingCents > 0 {
		notes = append(notes, letters.FormatKES(sm.PendingCents)+" in payments is waiting for the finance office to confirm it and is not counted yet.")
	}
	notes = append(notes, "This statement is made from the college's records at the time shown.")
	doc.Notes = strings.Join(notes, "\n")
	pdf, err := letters.LedgerDocument(doc)
	return pdf, fileName("Fee statement", acc.Student.StudentNumber), err
}
