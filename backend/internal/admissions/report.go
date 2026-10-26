package admissions

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// MaxReportDays is the longest period one report covers.
const MaxReportDays = 731

// Figures counts applications submitted in the period by where they are now.
// Offered counts every application that received an offer, whatever
// happened next; Accepted counts those accepted (including enrolled).
type Figures struct {
	Submitted     int64 `json:"submitted"`
	InReview      int64 `json:"in_review"`
	InfoRequested int64 `json:"info_requested"`
	Offered       int64 `json:"offered"`
	Accepted      int64 `json:"accepted"`
	Enrolled      int64 `json:"enrolled"`
	Rejected      int64 `json:"rejected"`
	Withdrawn     int64 `json:"withdrawn"`
	Expired       int64 `json:"expired"`
}

func (f *Figures) add(o Figures) {
	f.Submitted += o.Submitted
	f.InReview += o.InReview
	f.InfoRequested += o.InfoRequested
	f.Offered += o.Offered
	f.Accepted += o.Accepted
	f.Enrolled += o.Enrolled
	f.Rejected += o.Rejected
	f.Withdrawn += o.Withdrawn
	f.Expired += o.Expired
}

type ReportRow struct {
	ProgrammeID    uuid.UUID  `json:"programme_id"`
	ProgrammeTitle string     `json:"programme_title"`
	IntakeID       *uuid.UUID `json:"intake_id"`
	IntakeName     *string    `json:"intake_name"`
	Figures
}

type Report struct {
	From   string      `json:"from"`
	To     string      `json:"to"`
	Rows   []ReportRow `json:"rows"`
	Totals Figures     `json:"totals"`
}

// Report covers applications submitted from from to to (Nairobi dates,
// inclusive), by programme and intake. Empty dates mean 1 January this
// year to today.
func (s *Service) Report(ctx context.Context, from, to string) (*Report, error) {
	if err := s.expireOffers(ctx); err != nil {
		return nil, err
	}
	today := s.today()
	start, end := time.Date(today.Year(), 1, 1, 0, 0, 0, 0, time.UTC), today
	var pr content.Problems
	parse := func(field, v string, into *time.Time) {
		if v == "" {
			return
		}
		d, err := time.Parse(time.DateOnly, v)
		if err != nil {
			pr.Add(field, "Enter the date as YYYY-MM-DD.")
			return
		}
		*into = d
	}
	parse("from", from, &start)
	parse("to", to, &end)
	if err := pr.Err(); err != nil {
		return nil, err
	}
	if end.Before(start) {
		pr.Add("to", "The end date must be on or after the start date.")
	} else if end.Sub(start) > MaxReportDays*24*time.Hour {
		pr.Add("to", "Choose a period of at most two years.")
	}
	if err := pr.Err(); err != nil {
		return nil, err
	}
	rows, err := dbq.New(s.pool).AdmissionsReport(ctx, dbq.AdmissionsReportParams{FromDate: start, ToDate: end})
	if err != nil {
		return nil, err
	}
	out := &Report{From: start.Format(time.DateOnly), To: end.Format(time.DateOnly), Rows: make([]ReportRow, 0, len(rows))}
	for _, r := range rows {
		f := Figures{Submitted: r.Submitted, InReview: r.InReview, InfoRequested: r.InfoRequested, Offered: r.Offered,
			Accepted: r.Accepted, Enrolled: r.Enrolled, Rejected: r.Rejected, Withdrawn: r.Withdrawn, Expired: r.Expired}
		out.Rows = append(out.Rows, ReportRow{ProgrammeID: r.ProgrammeID, ProgrammeTitle: r.ProgrammeTitle, IntakeID: r.IntakeID, IntakeName: r.IntakeName, Figures: f})
		out.Totals.add(f)
	}
	return out, nil
}
