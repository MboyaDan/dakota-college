package finance

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
)

// Invoices (brief 9). A draft is made at enrolment (Stage 10a), by a
// billing run for a term, or by hand. While a draft, its lines and plan can
// change; issuing numbers it DTC/INV/YYYY/NNNNN (gapless) and fixes each
// instalment's amount and due date. An issued invoice never changes: a
// mistake is corrected with an adjustment.

type InvoiceInstalment struct {
	Seq         int     `json:"seq"`
	Percent     int     `json:"percent"`
	DueDays     int     `json:"due_days"`
	AmountCents *int64  `json:"amount_cents"`
	DueOn       *string `json:"due_on"`
}

type InvoiceSummary struct {
	ID            uuid.UUID  `json:"id"`
	StudentID     uuid.UUID  `json:"student_id"`
	StudentNumber string     `json:"student_number,omitempty"`
	StudentName   string     `json:"student_name,omitempty"`
	Kind          string     `json:"kind"`
	Status        string     `json:"status"`
	Number        *string    `json:"number"`
	TermName      *string    `json:"term_name"`
	TotalCents    int64      `json:"total_cents"`
	CreatedAt     time.Time  `json:"created_at"`
	IssuedAt      *time.Time `json:"issued_at"`
}

type Invoice struct {
	InvoiceSummary
	ProgrammeTitle string              `json:"programme_title"`
	Note           string              `json:"note"`
	VoidReason     string              `json:"void_reason"`
	CreatedByName  string              `json:"created_by_name"`
	IssuedByName   *string             `json:"issued_by_name"`
	Lines          []Line              `json:"lines"`
	Instalments    []InvoiceInstalment `json:"instalments"`
	// HasStructure: a fee structure exists to fill this draft from.
	HasStructure bool `json:"has_structure"`
}

func dateString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.DateOnly)
	return &s
}

func invoiceParts(ctx context.Context, q *dbq.Queries, id uuid.UUID) ([]Line, []InvoiceInstalment, error) {
	rows, err := q.InvoiceLines(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	lines := make([]Line, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, Line{Label: r.Label, Category: r.Category, AmountCents: r.AmountCents})
	}
	ins, err := q.InvoiceInstalments(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	plan := make([]InvoiceInstalment, 0, len(ins))
	for _, n := range ins {
		plan = append(plan, InvoiceInstalment{Seq: int(n.Seq), Percent: int(n.Percent), DueDays: int(n.DueDays), AmountCents: n.AmountCents, DueOn: dateString(n.DueOn)})
	}
	return lines, plan, nil
}

func getInvoice(ctx context.Context, q *dbq.Queries, id uuid.UUID) (*Invoice, error) {
	r, err := q.GetInvoice(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out := &Invoice{InvoiceSummary: InvoiceSummary{ID: r.ID, StudentID: r.StudentID, StudentNumber: r.StudentNumber,
		StudentName: fullName(r.FirstName, r.MiddleName, r.LastName), Kind: r.Kind, Status: r.Status, Number: r.Number,
		TermName: r.TermName, TotalCents: r.TotalCents, CreatedAt: r.CreatedAt, IssuedAt: r.IssuedAt},
		ProgrammeTitle: r.ProgrammeTitle, Note: r.Note, VoidReason: r.VoidReason, CreatedByName: r.CreatedByName, IssuedByName: r.IssuedByName}
	if out.Lines, out.Instalments, err = invoiceParts(ctx, q, id); err != nil {
		return nil, err
	}
	if r.Status == "draft" {
		st, err := q.FinanceStudent(ctx, r.StudentID)
		if err != nil {
			return nil, err
		}
		if _, out.HasStructure, err = structureFor(ctx, q, st.ProgrammeID, st.IntakeID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Service) GetInvoice(ctx context.Context, id uuid.UUID) (*Invoice, error) {
	return getInvoice(ctx, dbq.New(s.pool), id)
}

type InvoiceFilter struct {
	Status *string
	TermID *uuid.UUID
	Cohort *uuid.UUID
	Query  string
	Page   int
}

const pageSize = 50

// likePattern escapes LIKE wildcards in a search box entry.
func likePattern(q string) *string {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}
	q = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(tidy(q, 80))
	return &q
}

func offset(page int) int32 {
	if page < 1 {
		page = 1
	}
	return int32((page - 1) * pageSize)
}

func (s *Service) ListInvoices(ctx context.Context, f InvoiceFilter) ([]InvoiceSummary, int64, error) {
	rows, err := dbq.New(s.pool).ListInvoices(ctx, dbq.ListInvoicesParams{Status: f.Status, TermID: f.TermID, CohortID: f.Cohort,
		Q: likePattern(f.Query), Lim: pageSize, Off: offset(f.Page)})
	if err != nil {
		return nil, 0, err
	}
	out := make([]InvoiceSummary, 0, len(rows))
	var total int64
	for _, r := range rows {
		total = r.TotalCount
		out = append(out, InvoiceSummary{ID: r.ID, StudentID: r.StudentID, StudentNumber: r.StudentNumber, StudentName: fullName(r.FirstName, r.MiddleName, r.LastName),
			Kind: r.Kind, Status: r.Status, Number: r.Number, TermName: r.TermName, TotalCents: r.TotalCents, CreatedAt: r.CreatedAt, IssuedAt: r.IssuedAt})
	}
	return out, total, nil
}

func lockDraft(ctx context.Context, q *dbq.Queries, id uuid.UUID) (dbq.LockInvoiceRow, error) {
	inv, err := q.LockInvoice(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return inv, ErrNotFound
	}
	if err == nil && inv.Status != "draft" {
		err = ErrNotDraft
	}
	return inv, err
}

func writeInvoiceParts(ctx context.Context, q *dbq.Queries, id uuid.UUID, lines []Line, plan []Instalment) error {
	if err := q.DeleteInvoiceLines(ctx, id); err != nil {
		return err
	}
	if err := q.DeleteInvoiceInstalments(ctx, id); err != nil {
		return err
	}
	for i, l := range lines {
		if err := q.InsertInvoiceLine(ctx, dbq.InsertInvoiceLineParams{InvoiceID: id, Label: l.Label, Category: l.Category, AmountCents: l.AmountCents, SortOrder: int32(i)}); err != nil {
			return err
		}
	}
	for i, n := range plan {
		if err := q.InsertInvoiceInstalment(ctx, dbq.InsertInvoiceInstalmentParams{InvoiceID: id, Seq: int16(i + 1), Percent: int16(n.Percent), DueDays: int16(n.DueDays)}); err != nil {
			return err
		}
	}
	return nil
}

// fill writes a draft's lines from a structure: every line for the first
// (enrolment) invoice, the per-term lines for a term invoice.
func fill(ctx context.Context, q *dbq.Queries, invoice uuid.UUID, kind string, structure uuid.UUID) (int, error) {
	lines, plan, err := structureLines(ctx, q, structure)
	if err != nil {
		return 0, err
	}
	var chosen []Line
	for _, l := range lines {
		if kind == "enrolment" || l.Charge == "per_term" {
			chosen = append(chosen, Line{Label: l.Label, Category: l.Category, AmountCents: l.AmountCents})
		}
	}
	return len(chosen), writeInvoiceParts(ctx, q, invoice, chosen, plan)
}

// FillFromStructure fills a draft from the student's fee structure, inside
// the caller's transaction (enrolment uses it). It reports false, and
// changes nothing, when there is no structure for the student.
func FillFromStructure(ctx context.Context, q *dbq.Queries, invoice uuid.UUID) (bool, error) {
	inv, err := lockDraft(ctx, q, invoice)
	if err != nil {
		return false, err
	}
	st, err := q.FinanceStudent(ctx, inv.StudentID)
	if err != nil {
		return false, err
	}
	structure, ok, err := structureFor(ctx, q, st.ProgrammeID, st.IntakeID)
	if err != nil || !ok {
		return false, err
	}
	_, err = fill(ctx, q, invoice, inv.Kind, structure)
	return err == nil, err
}

// Fill replaces a draft's lines with those of the student's fee structure.
func (s *Service) Fill(ctx context.Context, id uuid.UUID, req audit.Request) (*Invoice, error) {
	var out *Invoice
	err := s.tx(ctx, func(q *dbq.Queries) error {
		inv, err := lockDraft(ctx, q, id)
		if err != nil {
			return err
		}
		if inv.Kind == "other" {
			return content.Problems{{Field: "invoice", Message: "Add the lines of this invoice by hand: it is not a term or enrolment invoice."}}.Err()
		}
		ok, err := FillFromStructure(ctx, q, id)
		if err != nil {
			return err
		}
		if !ok {
			return content.Problems{{Field: "invoice", Message: "There is no fee structure for this student's programme and intake. Set one up under Fee structures first."}}.Err()
		}
		if out, err = getInvoice(ctx, q, id); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "finance.invoice_filled", EntityType: "invoice", EntityID: id.String(),
			Details: map[string]any{"lines": len(out.Lines), "total_cents": out.TotalCents}})
	})
	return out, err
}

type DraftInput struct {
	Note        string       `json:"note"`
	Lines       []Line       `json:"lines"`
	Instalments []Instalment `json:"instalments"`
}

// UpdateDraft replaces a draft's note, lines and plan.
func (s *Service) UpdateDraft(ctx context.Context, id uuid.UUID, in DraftInput, req audit.Request) (*Invoice, error) {
	var pr content.Problems
	lines := CheckLines(&pr, in.Lines)
	CheckInstalments(&pr, in.Instalments)
	if len([]rune(in.Note)) > 1000 {
		pr.Add("note", "Use 1,000 characters or fewer.")
	}
	if err := pr.Err(); err != nil {
		return nil, err
	}
	var out *Invoice
	err := s.tx(ctx, func(q *dbq.Queries) error {
		if _, err := lockDraft(ctx, q, id); err != nil {
			return err
		}
		if err := q.UpdateInvoiceNote(ctx, dbq.UpdateInvoiceNoteParams{ID: id, Note: tidy(in.Note, 1000)}); err != nil {
			return err
		}
		if err := writeInvoiceParts(ctx, q, id, lines, in.Instalments); err != nil {
			return err
		}
		var err error
		if out, err = getInvoice(ctx, q, id); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "finance.invoice_edited", EntityType: "invoice", EntityID: id.String(),
			Details: map[string]any{"lines": len(lines), "total_cents": out.TotalCents}})
	})
	return out, err
}

// CreateInvoice opens an empty draft for a student: a term invoice (one per
// term) or an "other" invoice for anything else.
func (s *Service) CreateInvoice(ctx context.Context, p *auth.Principal, student uuid.UUID, kind string, term *uuid.UUID, req audit.Request) (*Invoice, error) {
	var pr content.Problems
	switch kind {
	case "term":
		if term == nil {
			pr.Add("term_id", "Choose the term this invoice is for.")
		}
	case "other":
		term = nil
	default:
		pr.Add("kind", "Choose a term invoice or another invoice.")
	}
	if err := pr.Err(); err != nil {
		return nil, err
	}
	var out *Invoice
	err := s.tx(ctx, func(q *dbq.Queries) error {
		st, err := q.FinanceStudent(ctx, student)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		note := ""
		if term != nil {
			t, err := q.FinanceTerm(ctx, *term)
			if errors.Is(err, pgx.ErrNoRows) {
				return content.Problems{{Field: "term_id", Message: "Choose a term from the list."}}.Err()
			}
			if err != nil {
				return err
			}
			note = "Fees for " + t.Name + "."
		}
		id, err := q.InsertInvoice(ctx, dbq.InsertInvoiceParams{StudentID: student, IntakeID: st.IntakeID, Kind: kind, TermID: term, Note: note, CreatedBy: p.UserID})
		if isUniqueViolation(err) {
			return content.Problems{{Field: "term_id", Message: "This student already has an invoice for that term. Open it from their account."}}.Err()
		}
		if err != nil {
			return err
		}
		if out, err = getInvoice(ctx, q, id); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "finance.invoice_created", EntityType: "invoice", EntityID: id.String(),
			Details: map[string]any{"student_id": student.String(), "kind": kind}})
	})
	return out, err
}

// issue numbers one draft and fixes its instalments, in the caller's
// transaction.
func (s *Service) issue(ctx context.Context, q *dbq.Queries, p *auth.Principal, id uuid.UUID, req audit.Request) (string, error) {
	if _, err := lockDraft(ctx, q, id); err != nil {
		return "", err
	}
	lines, plan, err := invoiceParts(ctx, q, id)
	if err != nil {
		return "", err
	}
	var total int64
	for _, l := range lines {
		total += l.AmountCents
	}
	if len(lines) == 0 || total <= 0 {
		return "", content.Problems{{Field: "lines", Message: "Add at least one fee line before issuing."}}.Err()
	}
	if len(plan) == 0 {
		if err := q.InsertInvoiceInstalment(ctx, dbq.InsertInvoiceInstalmentParams{InvoiceID: id, Seq: 1, Percent: 100, DueDays: 0}); err != nil {
			return "", err
		}
		plan = []InvoiceInstalment{{Seq: 1, Percent: 100, DueDays: 0}}
	}
	shares := make([]Instalment, len(plan))
	for i, n := range plan {
		shares[i] = Instalment{Percent: n.Percent, DueDays: n.DueDays}
	}
	var check content.Problems
	CheckInstalments(&check, shares)
	if err := check.Err(); err != nil {
		return "", err
	}
	today := s.today()
	for i, amount := range Split(total, shares) {
		due := today.AddDate(0, 0, plan[i].DueDays)
		if err := q.FixInvoiceInstalment(ctx, dbq.FixInvoiceInstalmentParams{InvoiceID: id, Seq: int16(plan[i].Seq), AmountCents: &amount, DueOn: &due}); err != nil {
			return "", err
		}
	}
	year := s.now().In(nairobi).Year()
	n, err := q.NextCounterValue(ctx, dbq.NextCounterValueParams{Name: "invoice", Period: int32(year)})
	if err != nil {
		return "", err
	}
	number := fmt.Sprintf("DTC/INV/%d/%05d", year, n)
	if rows, err := q.IssueInvoice(ctx, dbq.IssueInvoiceParams{ID: id, Number: &number, IssuedBy: &p.UserID}); err != nil || rows != 1 {
		if err == nil {
			err = ErrNotDraft
		}
		return "", err
	}
	return number, audit.Write(ctx, q, req, audit.Entry{Action: "finance.invoice_issued", EntityType: "invoice", EntityID: id.String(),
		Details: map[string]any{"number": number, "total_cents": total, "instalments": len(plan)}})
}

// Issue issues drafts, all or none: if one cannot be issued, nothing is.
func (s *Service) Issue(ctx context.Context, p *auth.Principal, ids []uuid.UUID, req audit.Request) ([]string, error) {
	if len(ids) == 0 || len(ids) > 500 {
		return nil, content.Problems{{Field: "ids", Message: "Choose from 1 to 500 draft invoices."}}.Err()
	}
	var numbers []string
	err := s.tx(ctx, func(q *dbq.Queries) error {
		numbers = numbers[:0]
		seen := map[uuid.UUID]bool{}
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			n, err := s.issue(ctx, q, p, id, req)
			if err != nil {
				var ve *content.ValidationError
				if errors.As(err, &ve) && len(ids) > 1 {
					// Say whose draft stopped the batch. A validation problem is
					// not a database error, so the transaction can still read.
					who := "one student"
					if inv, gerr := q.GetInvoice(ctx, id); gerr == nil {
						who = inv.StudentNumber
					}
					return content.Problems{{Field: "ids", Message: "Nothing was issued. The draft for " + who + " is not ready: " + ve.Problems[0].Message}}.Err()
				}
				return err
			}
			numbers = append(numbers, n)
		}
		return nil
	})
	return numbers, err
}

// Void sets aside a draft that is not needed. It keeps no number.
func (s *Service) Void(ctx context.Context, id uuid.UUID, reason string, req audit.Request) (*Invoice, error) {
	reason = tidy(reason, 500)
	if len([]rune(reason)) < 5 {
		return nil, content.Problems{{Field: "reason", Message: "Say why this draft is not needed (at least 5 characters)."}}.Err()
	}
	var out *Invoice
	err := s.tx(ctx, func(q *dbq.Queries) error {
		if _, err := lockDraft(ctx, q, id); err != nil {
			return err
		}
		if n, err := q.VoidInvoice(ctx, dbq.VoidInvoiceParams{ID: id, VoidReason: reason}); err != nil || n != 1 {
			if err == nil {
				err = ErrNotDraft
			}
			return err
		}
		var err error
		if out, err = getInvoice(ctx, q, id); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "finance.invoice_voided", EntityType: "invoice", EntityID: id.String(),
			Details: map[string]any{"reason": reason}})
	})
	return out, err
}

type BillingResult struct {
	Created    int         `json:"created"`
	Skipped    int         `json:"skipped"`
	InvoiceIDs []uuid.UUID `json:"invoice_ids"`
}

// Bill makes a draft term invoice for every active student of a cohort who
// does not have one for the term yet, from their fee structure. Students
// already billed for the term are skipped, so running it twice is safe.
func (s *Service) Bill(ctx context.Context, p *auth.Principal, cohort, term uuid.UUID, req audit.Request) (*BillingResult, error) {
	out := &BillingResult{InvoiceIDs: []uuid.UUID{}}
	err := s.tx(ctx, func(q *dbq.Queries) error {
		c, err := q.FinanceCohort(ctx, cohort)
		if errors.Is(err, pgx.ErrNoRows) {
			return content.Problems{{Field: "cohort_id", Message: "Choose a class from the list."}}.Err()
		}
		if err != nil {
			return err
		}
		t, err := q.FinanceTerm(ctx, term)
		if errors.Is(err, pgx.ErrNoRows) {
			return content.Problems{{Field: "term_id", Message: "Choose a term from the list."}}.Err()
		}
		if err != nil {
			return err
		}
		students, err := q.CohortStudentsForBilling(ctx, dbq.CohortStudentsForBillingParams{CohortID: &cohort, TermID: &term})
		if err != nil {
			return err
		}
		if len(students) == 0 {
			return content.Problems{{Field: "cohort_id", Message: "This class has no active students."}}.Err()
		}
		var missing int
		for _, st := range students {
			if st.Billed {
				out.Skipped++
				continue
			}
			structure, ok, err := structureFor(ctx, q, st.ProgrammeID, st.IntakeID)
			if err != nil {
				return err
			}
			if !ok {
				missing++
				continue
			}
			id, err := q.InsertInvoice(ctx, dbq.InsertInvoiceParams{StudentID: st.ID, IntakeID: st.IntakeID, Kind: "term", TermID: &term,
				Note: "Fees for " + t.Name + ".", CreatedBy: p.UserID})
			if err != nil {
				return err
			}
			n, err := fill(ctx, q, id, "term", structure)
			if err != nil {
				return err
			}
			if n == 0 {
				return content.Problems{{Field: "cohort_id", Message: "The fee structure for this class has no fees charged every term. Add them under Fee structures first."}}.Err()
			}
			out.Created++
			out.InvoiceIDs = append(out.InvoiceIDs, id)
		}
		if missing > 0 {
			return content.Problems{{Field: "cohort_id", Message: fmt.Sprintf("%d student(s) in this class have no fee structure for their programme and intake. Set one up under Fee structures first.", missing)}}.Err()
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "finance.billing_run", EntityType: "cohort", EntityID: c.ID.String(),
			Details: map[string]any{"term_id": term.String(), "created": out.Created, "skipped": out.Skipped}})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
