// Package reports serves the staff dashboards and count-only CSV exports
// (Stage 16b, brief 7.2). Every section is shown only to a viewer who may
// see its data already: admissions with admissions.view, finance with
// finance.view, enquiries with enquiries.view, and students, results and
// student services with students.view across every school (schools.all), so
// a Head of School never sees college-wide figures for other schools.
//
// Exports carry counts and sums only, never a name or contact detail, so
// they need reports.export plus the data permission but no approval (D-184).
package reports

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/academics"
	"github.com/MboyaDan/dakota-college/backend/internal/admissions"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/enquiry"
	"github.com/MboyaDan/dakota-college/backend/internal/finance"
)

const (
	PermView   = "reports.view"
	PermExport = "reports.export"
	// MaxDays is the longest period one dashboard or export covers.
	MaxDays = 731
)

var nairobi = time.FixedZone("EAT", 3*60*60)

type Options struct {
	Pool       *pgxpool.Pool
	Admissions *admissions.Service
	// Now is for tests; defaults to time.Now.
	Now func() time.Time
}

type Service struct {
	pool       *pgxpool.Pool
	admissions *admissions.Service
	now        func() time.Time
}

func NewService(o Options) *Service {
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Service{pool: o.Pool, admissions: o.Admissions, now: o.Now}
}

// ErrForbidden means the viewer may not see the data an export is made of.
var ErrForbidden = errors.New("not allowed to see this data")

// Period is a range of Nairobi dates, both included.
type Period struct {
	From time.Time
	To   time.Time
}

func (p Period) FromText() string { return p.From.Format(time.DateOnly) }
func (p Period) ToText() string   { return p.To.Format(time.DateOnly) }

// ParsePeriod reads from and to (YYYY-MM-DD). Empty means 1 January this
// year to today. Problems come back as a *content.ValidationError.
func (s *Service) ParsePeriod(from, to string) (Period, error) {
	y, m, d := s.now().In(nairobi).Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	p := Period{From: time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC), To: today}
	var pr content.Problems
	parse := func(field, v string, into *time.Time) {
		if v == "" {
			return
		}
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			pr.Add(field, "Enter the date as YYYY-MM-DD.")
			return
		}
		*into = t
	}
	parse("from", from, &p.From)
	parse("to", to, &p.To)
	if err := pr.Err(); err != nil {
		return p, err
	}
	if p.To.Before(p.From) {
		pr.Add("to", "The end date must be on or after the start date.")
	} else if p.To.Sub(p.From) > MaxDays*24*time.Hour {
		pr.Add("to", "Choose a period of at most two years.")
	}
	return p, pr.Err()
}

// collegeWide is students.view without a school limit.
func collegeWide(p *auth.Principal) bool {
	return p.Has(academics.PermStudentsView) && p.Has(academics.PermAllSchools)
}

func canAdmissions(p *auth.Principal) bool { return p.Has(admissions.PermView) }
func canFinance(p *auth.Principal) bool    { return p.Has(finance.PermView) }
func canEnquiries(p *auth.Principal) bool  { return p.Has(enquiry.PermView) }
