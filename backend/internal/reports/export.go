package reports

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/MboyaDan/dakota-college/backend/internal/admissions"
	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// Exports lists the count-only exports by key.
var Exports = []string{"admissions", "enrolments", "fee-collection", "results"}

// ErrUnknownExport means there is no export with that key.
var ErrUnknownExport = errors.New("no such export")

// File is one export, ready to download.
type File struct {
	Name string
	Data []byte
	Rows int
}

// Export makes a CSV the viewer may see and audits it. Text cells that a
// spreadsheet would read as a formula are prefixed with an apostrophe.
func (s *Service) Export(ctx context.Context, p *auth.Principal, req audit.Request, kind string, period Period) (*File, error) {
	var (
		header []string
		rows   [][]string
		err    error
	)
	switch kind {
	case "admissions":
		if !canAdmissions(p) {
			return nil, ErrForbidden
		}
		header, rows, err = s.admissionsCSV(ctx, period)
	case "enrolments":
		if !collegeWide(p) {
			return nil, ErrForbidden
		}
		header, rows, err = s.enrolmentsCSV(ctx, period)
	case "fee-collection":
		if !canFinance(p) {
			return nil, ErrForbidden
		}
		header, rows, err = s.feeCollectionCSV(ctx, period)
	case "results":
		if !collegeWide(p) {
			return nil, ErrForbidden
		}
		header, rows, err = s.resultsCSV(ctx, period)
	default:
		return nil, ErrUnknownExport
	}
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	// A byte order mark so Excel reads UTF-8.
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	w.UseCRLF = true
	if err := w.Write(header); err != nil {
		return nil, err
	}
	if err := w.WriteAll(rows); err != nil {
		return nil, err
	}

	err = audit.Write(ctx, dbq.New(s.pool), req, audit.Entry{
		Action: "reports.exported", EntityType: "report", EntityID: kind,
		Details: map[string]any{"from": period.FromText(), "to": period.ToText(), "rows": len(rows)},
	})
	if err != nil {
		return nil, err
	}
	return &File{
		Name: fmt.Sprintf("dakota-%s-%s-to-%s.csv", kind, period.FromText(), period.ToText()),
		Data: buf.Bytes(), Rows: len(rows),
	}, nil
}

func (s *Service) admissionsCSV(ctx context.Context, period Period) ([]string, [][]string, error) {
	r, err := s.admissions.Report(ctx, period.FromText(), period.ToText())
	if err != nil {
		return nil, nil, err
	}
	header := []string{"Programme", "Intake", "Submitted", "In review", "Information requested", "Offered", "Accepted",
		"Enrolled", "Rejected", "Withdrawn", "Expired"}
	rows := make([][]string, 0, len(r.Rows)+1)
	for _, row := range r.Rows {
		intake := ""
		if row.IntakeName != nil {
			intake = *row.IntakeName
		}
		rows = append(rows, append([]string{text(row.ProgrammeTitle), text(intake)}, figures(row.Figures)...))
	}
	rows = append(rows, append([]string{"Total", ""}, figures(r.Totals)...))
	return header, rows, nil
}

func (s *Service) enrolmentsCSV(ctx context.Context, period Period) ([]string, [][]string, error) {
	list, err := dbq.New(s.pool).StudentCounts(ctx, dbq.StudentCountsParams{FromDate: period.From, ToDate: period.To})
	if err != nil {
		return nil, nil, err
	}
	header := []string{"School", "Programme", "Status", "Students", "Joined in period"}
	rows := make([][]string, 0, len(list)+1)
	var students, joined int64
	for _, r := range list {
		rows = append(rows, []string{text(r.SchoolName), text(r.ProgrammeTitle), r.Status, num(r.Students), num(r.Joined)})
		students += r.Students
		joined += r.Joined
	}
	rows = append(rows, []string{"Total", "", "", num(students), num(joined)})
	return header, rows, nil
}

func (s *Service) feeCollectionCSV(ctx context.Context, period Period) ([]string, [][]string, error) {
	list, err := dbq.New(s.pool).FinanceByMonth(ctx, dbq.FinanceByMonthParams{FromDate: period.From, ToDate: period.To})
	if err != nil {
		return nil, nil, err
	}
	header := []string{"Month", "Billed (KES)", "Collected (KES)", "Charges (KES)", "Credits (KES)"}
	rows := make([][]string, 0, len(list)+1)
	var t MonthMoney
	for _, r := range list {
		rows = append(rows, []string{r.Month, KES(r.BilledCents), KES(r.CollectedCents), KES(r.ChargesCents), KES(r.CreditsCents)})
		t.BilledCents += r.BilledCents
		t.CollectedCents += r.CollectedCents
		t.ChargesCents += r.ChargesCents
		t.CreditsCents += r.CreditsCents
	}
	rows = append(rows, []string{"Total", KES(t.BilledCents), KES(t.CollectedCents), KES(t.ChargesCents), KES(t.CreditsCents)})
	return header, rows, nil
}

func (s *Service) resultsCSV(ctx context.Context, period Period) ([]string, [][]string, error) {
	list, err := dbq.New(s.pool).ReleasedResults(ctx, dbq.ReleasedResultsParams{FromDate: period.From, ToDate: period.To})
	if err != nil {
		return nil, nil, err
	}
	header := []string{"Term", "Unit code", "Unit", "School", "Release version", "Released on", "Results", "Passed", "Pass rate (%)"}
	rows := make([][]string, 0, len(list)+1)
	var results, passed int64
	for _, r := range list {
		rows = append(rows, []string{text(r.TermName), text(r.UnitCode), text(r.UnitTitle), text(r.SchoolName), num(int64(r.Version)),
			r.ReleasedAt.In(nairobi).Format("2006-01-02"), num(r.Results), num(r.Passed), Percent(r.Passed, r.Results)})
		results += r.Results
		passed += r.Passed
	}
	rows = append(rows, []string{"Total", "", "", "", "", "", num(results), num(passed), Percent(passed, results)})
	return header, rows, nil
}

func figures(f admissions.Figures) []string {
	v := []int64{f.Submitted, f.InReview, f.InfoRequested, f.Offered, f.Accepted, f.Enrolled, f.Rejected, f.Withdrawn, f.Expired}
	out := make([]string, len(v))
	for i, n := range v {
		out[i] = num(n)
	}
	return out
}

func num(n int64) string { return strconv.FormatInt(n, 10) }

// KES formats cents as shillings with two decimals, without floating point.
func KES(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

// Percent is part of whole as a percentage with one decimal, rounded half
// up, without floating point. Empty when whole is zero.
func Percent(part, whole int64) string {
	if whole <= 0 {
		return ""
	}
	tenths := (part*1000 + whole/2) / whole
	return fmt.Sprintf("%d.%d", tenths/10, tenths%10)
}

// text guards a free-text cell against spreadsheet formula injection.
func text(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}
