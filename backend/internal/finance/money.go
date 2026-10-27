package finance

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MboyaDan/dakota-college/backend/internal/content"
)

// Money rules that need no database: what a valid line or instalment plan
// is, how an invoice total is split, and how a statement runs. All amounts
// are integer cents of KES (NN-03); nothing here uses floating point.

// MaxAmountCents is the largest single amount: KES 1,000,000,000.00.
const MaxAmountCents = 100_000_000_000

var categories = []string{"tuition", "registration", "assessment", "caution", "other"}

// Line is one fee on an invoice.
type Line struct {
	Label       string `json:"label"`
	Category    string `json:"category"`
	AmountCents int64  `json:"amount_cents"`
}

// StructureLine is one fee in a fee structure: charged once (on the first
// invoice, at enrolment) or every term.
type StructureLine struct {
	Label       string `json:"label"`
	Category    string `json:"category"`
	Charge      string `json:"charge"`
	AmountCents int64  `json:"amount_cents"`
}

// Instalment is a share of an invoice, due some days after it is issued.
type Instalment struct {
	Percent int `json:"percent"`
	DueDays int `json:"due_days"`
}

// wholeAtIssue is the plan when none is given: everything due on issue.
var wholeAtIssue = []Instalment{{Percent: 100, DueDays: 0}}

func checkLine(pr *content.Problems, field string, label, category string, amount int64) string {
	label = strings.Join(strings.Fields(label), " ")
	switch n := len([]rune(label)); {
	case n == 0:
		pr.Add(field+".label", "Enter what the fee is for.")
	case n > 120:
		pr.Add(field+".label", "Use 120 characters or fewer.")
	}
	if !slices.Contains(categories, category) {
		pr.Add(field+".category", "Choose tuition, registration, assessment, caution or other.")
	}
	if amount < 1 || amount > MaxAmountCents {
		pr.Add(field+".amount_cents", "Enter an amount above KES 0.00 and up to KES 1,000,000,000.00.")
	}
	return label
}

// CheckLines validates invoice lines and tidies their labels.
func CheckLines(pr *content.Problems, lines []Line) []Line {
	if len(lines) > 30 {
		pr.Add("lines", "An invoice has at most 30 lines.")
		return lines
	}
	out := make([]Line, len(lines))
	for i, l := range lines {
		out[i] = l
		out[i].Label = checkLine(pr, fmt.Sprintf("lines.%d", i), l.Label, l.Category, l.AmountCents)
	}
	return out
}

// CheckStructureLines validates fee structure lines.
func CheckStructureLines(pr *content.Problems, lines []StructureLine) []StructureLine {
	switch {
	case len(lines) == 0:
		pr.Add("lines", "Add at least one fee.")
		return lines
	case len(lines) > 30:
		pr.Add("lines", "A fee structure has at most 30 lines.")
		return lines
	}
	out := make([]StructureLine, len(lines))
	for i, l := range lines {
		field := fmt.Sprintf("lines.%d", i)
		out[i] = l
		out[i].Label = checkLine(pr, field, l.Label, l.Category, l.AmountCents)
		if l.Charge != "once" && l.Charge != "per_term" {
			pr.Add(field+".charge", "Choose once (at enrolment) or every term.")
		}
	}
	return out
}

// CheckInstalments validates a plan: up to 12 parts whose shares add up
// to 100, each due later than the one before. An empty plan means the
// whole amount is due on issue.
func CheckInstalments(pr *content.Problems, plan []Instalment) {
	if len(plan) == 0 {
		return
	}
	if len(plan) > 12 {
		pr.Add("instalments", "Use 12 instalments or fewer.")
		return
	}
	total := 0
	for i, n := range plan {
		field := fmt.Sprintf("instalments.%d", i)
		if n.Percent < 1 || n.Percent > 100 {
			pr.Add(field+".percent", "Enter a share from 1 to 100.")
		}
		if n.DueDays < 0 || n.DueDays > 730 {
			pr.Add(field+".due_days", "Enter a number of days from 0 to 730.")
		} else if i > 0 && n.DueDays <= plan[i-1].DueDays {
			pr.Add(field+".due_days", "Each instalment must be due later than the one before.")
		}
		total += n.Percent
	}
	if total != 100 {
		pr.Add("instalments", fmt.Sprintf("The shares add up to %d. They must add up to 100.", total))
	}
}

// Split divides a total by a plan. Each part is rounded down to the cent
// and the last part takes what is left, so the parts always add up to the
// total exactly.
func Split(total int64, plan []Instalment) []int64 {
	if len(plan) == 0 {
		plan = wholeAtIssue
	}
	out := make([]int64, len(plan))
	var used int64
	for i, n := range plan {
		if i == len(plan)-1 {
			out[i] = total - used
			break
		}
		out[i] = total * int64(n.Percent) / 100
		used += out[i]
	}
	return out
}

// Due is one amount that falls due on a date.
type Due struct {
	On     time.Time
	Amount int64
}

// Schedule says what is overdue by today and what falls due next, given
// the instalments of issued invoices, the extra charges (due at once) and
// what has been settled (confirmed payments and credits). Payments settle
// the earliest dues first.
func Schedule(dues []Due, charges, settled int64, today time.Time) (overdue int64, nextOn *time.Time, nextAmount int64) {
	dues = slices.Clone(dues)
	slices.SortStableFunc(dues, func(a, b Due) int { return a.On.Compare(b.On) })
	owedByToday := charges
	cumulative := charges
	for _, d := range dues {
		cumulative += d.Amount
		if !d.On.After(today) {
			owedByToday += d.Amount
			continue
		}
		if nextOn == nil && cumulative > settled {
			on := d.On
			nextOn = &on
			nextAmount = min(d.Amount, cumulative-settled)
		}
	}
	return max(owedByToday-settled, 0), nextOn, nextAmount
}

// Entry is one row of a fee statement. Debits raise the balance (invoices,
// charges); credits lower it (payments, credits).
type Entry struct {
	Date         string `json:"date"`
	Kind         string `json:"kind"` // invoice, payment, charge, credit
	Description  string `json:"description"`
	Reference    string `json:"reference"`
	DebitCents   int64  `json:"debit_cents"`
	CreditCents  int64  `json:"credit_cents"`
	BalanceCents int64  `json:"balance_cents"`
	day          time.Time
	at           time.Time
}

// Run orders entries by date (then by when they were recorded) and fills
// in the running balance. It returns the closing balance.
func Run(entries []Entry) ([]Entry, int64) {
	slices.SortStableFunc(entries, func(a, b Entry) int {
		if c := a.day.Compare(b.day); c != 0 {
			return c
		}
		return a.at.Compare(b.at)
	})
	var balance int64
	for i := range entries {
		balance += entries[i].DebitCents - entries[i].CreditCents
		entries[i].BalanceCents = balance
	}
	return entries, balance
}

// NewEntry makes a statement entry for a calendar day (a date column, or
// dateOf a timestamp), recorded at at.
func NewEntry(kind, description, reference string, debit, credit int64, day, at time.Time) Entry {
	return Entry{Date: day.Format(time.DateOnly), Kind: kind, Description: description, Reference: reference,
		DebitCents: debit, CreditCents: credit, day: day, at: at}
}

var nairobi = time.FixedZone("EAT", 3*60*60)

// dateOf is the calendar date of a moment in Nairobi, as midnight UTC (the
// way date columns are read).
func dateOf(t time.Time) time.Time {
	y, m, d := t.In(nairobi).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

var amountText = regexp.MustCompile(`^(\d{1,10})(?:\.(\d{1,2}))?$`)

// ParseAmount reads "1234", "1234.5" or "1234.50" (as a provider sends it)
// into cents, on the digits, never through floating point. ok is false for
// anything else or for zero.
func ParseAmount(text string) (cents int64, ok bool) {
	m := amountText.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return 0, false
	}
	whole, _ := strconv.ParseInt(m[1], 10, 64)
	frac := int64(0)
	if m[2] != "" {
		frac, _ = strconv.ParseInt((m[2] + "0")[:2], 10, 64)
	}
	cents = whole*100 + frac
	return cents, cents > 0
}
