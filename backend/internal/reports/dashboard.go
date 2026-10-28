package reports

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/admissions"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/enquiry"
)

// Dashboard is what one viewer may see for a period. A section the viewer
// may not see is null.
type Dashboard struct {
	From       string             `json:"from"`
	To         string             `json:"to"`
	Admissions *AdmissionsSection `json:"admissions"`
	Students   *StudentsSection   `json:"students"`
	Results    *ResultsSection    `json:"results"`
	Finance    *FinanceSection    `json:"finance"`
	Enquiries  *EnquiriesSection  `json:"enquiries"`
}

type SchoolFigures struct {
	SchoolID   uuid.UUID `json:"school_id"`
	SchoolName string    `json:"school_name"`
	admissions.Figures
}

// AdmissionsSection counts applications submitted in the period by where
// they are now (as the admissions report, D-126).
type AdmissionsSection struct {
	Totals   admissions.Figures `json:"totals"`
	BySchool []SchoolFigures    `json:"by_school"`
}

type StudentStatusCounts struct {
	Active    int64 `json:"active"`
	Deferred  int64 `json:"deferred"`
	Suspended int64 `json:"suspended"`
	Withdrawn int64 `json:"withdrawn"`
	Graduated int64 `json:"graduated"`
}

func (c *StudentStatusCounts) add(status string, n int64) {
	switch status {
	case "active":
		c.Active += n
	case "deferred":
		c.Deferred += n
	case "suspended":
		c.Suspended += n
	case "withdrawn":
		c.Withdrawn += n
	case "graduated":
		c.Graduated += n
	}
}

type SchoolStudents struct {
	SchoolID   uuid.UUID `json:"school_id"`
	SchoolName string    `json:"school_name"`
	StudentStatusCounts
	// Joined counts students enrolled in the period.
	Joined int64 `json:"joined"`
}

// StudentsSection is every student now, by status, and who joined in the
// period. Open requests and pending detail changes wait on the registrar.
type StudentsSection struct {
	Totals         StudentStatusCounts `json:"totals"`
	Joined         int64               `json:"joined"`
	BySchool       []SchoolStudents    `json:"by_school"`
	OpenRequests   int64               `json:"open_requests"`
	PendingChanges int64               `json:"pending_changes"`
}

// ResultsSection is where every class's results are now, and the classes
// released in the period with how many passed.
type ResultsSection struct {
	Draft           int64 `json:"draft"`
	Submitted       int64 `json:"submitted"`
	Approved        int64 `json:"approved"`
	Released        int64 `json:"released"`
	ReleasedClasses int64 `json:"released_classes"`
	Results         int64 `json:"results"`
	Passed          int64 `json:"passed"`
}

type MonthMoney struct {
	Month          string `json:"month"`
	BilledCents    int64  `json:"billed_cents"`
	CollectedCents int64  `json:"collected_cents"`
	ChargesCents   int64  `json:"charges_cents"`
	CreditsCents   int64  `json:"credits_cents"`
}

// FinanceSection: money in the period by month, and the position now.
type FinanceSection struct {
	BilledCents         int64        `json:"billed_cents"`
	CollectedCents      int64        `json:"collected_cents"`
	ChargesCents        int64        `json:"charges_cents"`
	CreditsCents        int64        `json:"credits_cents"`
	ByMonth             []MonthMoney `json:"by_month"`
	OutstandingCents    int64        `json:"outstanding_cents"`
	PendingPayments     int64        `json:"pending_payments"`
	PendingPaymentCents int64        `json:"pending_payment_cents"`
}

type OfficeEnquiries struct {
	Office   string `json:"office"`
	Received int64  `json:"received"`
	Closed   int64  `json:"closed"`
	NewNow   int64  `json:"new_now"`
	OpenNow  int64  `json:"open_now"`
}

type EnquiriesSection struct {
	Received int64             `json:"received"`
	Closed   int64             `json:"closed"`
	NewNow   int64             `json:"new_now"`
	OpenNow  int64             `json:"open_now"`
	ByOffice []OfficeEnquiries `json:"by_office"`
}

// Dashboard builds the sections the viewer may see.
func (s *Service) Dashboard(ctx context.Context, p *auth.Principal, period Period) (*Dashboard, error) {
	d := &Dashboard{From: period.FromText(), To: period.ToText()}
	var err error
	if canAdmissions(p) {
		if d.Admissions, err = s.admissionsSection(ctx, period); err != nil {
			return nil, err
		}
	}
	if collegeWide(p) {
		if d.Students, err = s.studentsSection(ctx, period); err != nil {
			return nil, err
		}
		if d.Results, err = s.resultsSection(ctx, period); err != nil {
			return nil, err
		}
	}
	if canFinance(p) {
		if d.Finance, err = s.financeSection(ctx, period); err != nil {
			return nil, err
		}
	}
	if canEnquiries(p) {
		if d.Enquiries, err = s.enquiriesSection(ctx, period); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func (s *Service) admissionsSection(ctx context.Context, period Period) (*AdmissionsSection, error) {
	r, err := s.admissions.Report(ctx, period.FromText(), period.ToText())
	if err != nil {
		return nil, err
	}
	schools, err := dbq.New(s.pool).ProgrammeSchools(ctx)
	if err != nil {
		return nil, err
	}
	schoolOf := make(map[uuid.UUID]dbq.ProgrammeSchoolsRow, len(schools))
	for _, ps := range schools {
		schoolOf[ps.ProgrammeID] = ps
	}
	out := &AdmissionsSection{Totals: r.Totals, BySchool: []SchoolFigures{}}
	index := map[uuid.UUID]int{}
	for _, row := range r.Rows {
		ps := schoolOf[row.ProgrammeID]
		i, ok := index[ps.SchoolID]
		if !ok {
			i = len(out.BySchool)
			index[ps.SchoolID] = i
			out.BySchool = append(out.BySchool, SchoolFigures{SchoolID: ps.SchoolID, SchoolName: ps.SchoolName})
		}
		f := &out.BySchool[i].Figures
		f.Submitted += row.Submitted
		f.InReview += row.InReview
		f.InfoRequested += row.InfoRequested
		f.Offered += row.Offered
		f.Accepted += row.Accepted
		f.Enrolled += row.Enrolled
		f.Rejected += row.Rejected
		f.Withdrawn += row.Withdrawn
		f.Expired += row.Expired
	}
	sort.Slice(out.BySchool, func(a, b int) bool { return out.BySchool[a].SchoolName < out.BySchool[b].SchoolName })
	return out, nil
}

func (s *Service) studentsSection(ctx context.Context, period Period) (*StudentsSection, error) {
	q := dbq.New(s.pool)
	rows, err := q.StudentCounts(ctx, dbq.StudentCountsParams{FromDate: period.From, ToDate: period.To})
	if err != nil {
		return nil, err
	}
	out := &StudentsSection{BySchool: []SchoolStudents{}}
	index := map[uuid.UUID]int{}
	for _, r := range rows {
		i, ok := index[r.SchoolID]
		if !ok {
			i = len(out.BySchool)
			index[r.SchoolID] = i
			out.BySchool = append(out.BySchool, SchoolStudents{SchoolID: r.SchoolID, SchoolName: r.SchoolName})
		}
		out.BySchool[i].add(r.Status, r.Students)
		out.BySchool[i].Joined += r.Joined
		out.Totals.add(r.Status, r.Students)
		out.Joined += r.Joined
	}
	c, err := q.StudentServiceCounts(ctx)
	if err != nil {
		return nil, err
	}
	out.OpenRequests, out.PendingChanges = c.OpenRequests, c.PendingChanges
	return out, nil
}

func (s *Service) resultsSection(ctx context.Context, period Period) (*ResultsSection, error) {
	q := dbq.New(s.pool)
	c, err := q.ResultSheetCounts(ctx)
	if err != nil {
		return nil, err
	}
	out := &ResultsSection{Draft: c.Draft, Submitted: c.Submitted, Approved: c.Approved, Released: c.Released}
	rows, err := q.ReleasedResults(ctx, dbq.ReleasedResultsParams{FromDate: period.From, ToDate: period.To})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out.ReleasedClasses++
		out.Results += r.Results
		out.Passed += r.Passed
	}
	return out, nil
}

func (s *Service) financeSection(ctx context.Context, period Period) (*FinanceSection, error) {
	q := dbq.New(s.pool)
	rows, err := q.FinanceByMonth(ctx, dbq.FinanceByMonthParams{FromDate: period.From, ToDate: period.To})
	if err != nil {
		return nil, err
	}
	out := &FinanceSection{ByMonth: make([]MonthMoney, 0, len(rows))}
	for _, r := range rows {
		out.ByMonth = append(out.ByMonth, MonthMoney{Month: r.Month, BilledCents: r.BilledCents, CollectedCents: r.CollectedCents,
			ChargesCents: r.ChargesCents, CreditsCents: r.CreditsCents})
		out.BilledCents += r.BilledCents
		out.CollectedCents += r.CollectedCents
		out.ChargesCents += r.ChargesCents
		out.CreditsCents += r.CreditsCents
	}
	sum, err := q.FinanceSummary(ctx)
	if err != nil {
		return nil, err
	}
	out.OutstandingCents, out.PendingPayments, out.PendingPaymentCents = sum.NetOutstandingCents, sum.PendingPayments, sum.PendingCents
	return out, nil
}

func (s *Service) enquiriesSection(ctx context.Context, period Period) (*EnquiriesSection, error) {
	rows, err := dbq.New(s.pool).EnquiryCounts(ctx, dbq.EnquiryCountsParams{FromDate: period.From, ToDate: period.To})
	if err != nil {
		return nil, err
	}
	out := &EnquiriesSection{ByOffice: []OfficeEnquiries{}}
	index := map[string]int{}
	for _, r := range rows {
		office := enquiry.Office(r.QueryType)
		i, ok := index[office]
		if !ok {
			i = len(out.ByOffice)
			index[office] = i
			out.ByOffice = append(out.ByOffice, OfficeEnquiries{Office: office})
		}
		o := &out.ByOffice[i]
		o.Received += r.Received
		o.Closed += r.Closed
		o.NewNow += r.NewNow
		o.OpenNow += r.OpenNow
		out.Received += r.Received
		out.Closed += r.Closed
		out.NewNow += r.NewNow
		out.OpenNow += r.OpenNow
	}
	sort.Slice(out.ByOffice, func(a, b int) bool { return out.ByOffice[a].Office < out.ByOffice[b].Office })
	return out, nil
}
