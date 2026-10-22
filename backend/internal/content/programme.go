package content

import (
	"context"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// ProgrammeDoc is a programme with everything a visitor needs to decide
// (CLAUDE.md "five questions"): what they learn, how long, what they need,
// what it costs, when it starts. Requirements, intakes and fees are part of
// the document, so editing fees on a published programme also waits for
// approval.
type ProgrammeDoc struct {
	SchoolID        uuid.UUID     `json:"school_id"`
	Slug            string        `json:"slug"`
	Title           string        `json:"title"`
	Level           string        `json:"level"`
	Mode            string        `json:"mode"`
	DurationMonths  *int32        `json:"duration_months"`
	DurationNote    string        `json:"duration_note"`
	MinKCSEGrade    *string       `json:"min_kcse_grade"`
	Summary         string        `json:"summary"`
	Overview        string        `json:"overview"`
	WhatYouLearn    string        `json:"what_you_learn"`
	CareerOutcomes  string        `json:"career_outcomes"`
	AssessmentModel string        `json:"assessment_model"`
	SEOTitle        string        `json:"seo_title"`
	SEODescription  string        `json:"seo_description"`
	SortOrder       int32         `json:"sort_order"`
	Requirements    []Requirement `json:"requirements"`
	Intakes         []Intake      `json:"intakes"`
	Fees            []Fee         `json:"fees"`
}

type Requirement struct {
	Kind        string `json:"kind"`
	Requirement string `json:"requirement"`
}

type Intake struct {
	// ID is set for existing intakes; applications refer to it.
	ID                  *uuid.UUID `json:"id"`
	Name                string     `json:"name"`
	StartsOn            Date       `json:"starts_on"`
	ApplicationsOpenOn  *Date      `json:"applications_open_on"`
	ApplicationDeadline *Date      `json:"application_deadline"`
	Capacity            *int32     `json:"capacity"`
	Status              string     `json:"status"`
}

type Fee struct {
	Label    string `json:"label"`
	Category string `json:"category"`
	// AmountCents is KES cents: 1,000.00 KES is 100000 (ADR-05).
	AmountCents int64  `json:"amount_cents"`
	Frequency   string `json:"frequency"`
	Notes       string `json:"notes"`
}

var kcseGrades = []string{"A", "A-", "B+", "B", "B-", "C+", "C", "C-", "D+", "D", "D-", "E"}

// maxFeeCents is KES 10 million, far above any real line, to catch typos.
const maxFeeCents = 10_000_000 * 100

func (d ProgrammeDoc) Validate() error {
	var p Problems
	if d.SchoolID == uuid.Nil {
		p.add("school_id", "Choose the school this programme belongs to.")
	}
	p.slug("slug", d.Slug)
	p.required("title", d.Title, 160)
	p.oneOf("level", d.Level, "short_course", "artisan", "certificate", "diploma", "higher_diploma")
	p.oneOf("mode", d.Mode, "full_time", "part_time", "blended")
	p.oneOf("assessment_model", d.AssessmentModel, "marks", "competency", "both")
	if d.DurationMonths != nil && (*d.DurationMonths < 1 || *d.DurationMonths > 72) {
		p.add("duration_months", "Enter a duration between 1 and 72 months.")
	}
	if d.MinKCSEGrade != nil && !slices.Contains(kcseGrades, *d.MinKCSEGrade) {
		p.add("min_kcse_grade", "Choose a KCSE grade from A to E.")
	}
	p.optional("duration_note", d.DurationNote, 200)
	p.optional("summary", d.Summary, 400)
	p.optional("overview", d.Overview, 20000)
	p.optional("what_you_learn", d.WhatYouLearn, 20000)
	p.optional("career_outcomes", d.CareerOutcomes, 10000)
	p.optional("seo_title", d.SEOTitle, 70)
	p.optional("seo_description", d.SEODescription, 170)

	for i, r := range d.Requirements {
		f := fieldf("requirements", i)
		p.oneOf(f+".kind", r.Kind, "academic", "document", "other")
		p.required(f+".requirement", r.Requirement, 500)
	}
	seen := map[string]bool{}
	for i, in := range d.Intakes {
		f := fieldf("intakes", i)
		p.required(f+".name", in.Name, 80)
		p.oneOf(f+".status", in.Status, "planned", "open", "closed")
		if in.StartsOn.IsZero() {
			p.add(f+".starts_on", "Choose the start date.")
		}
		if seen[in.StartsOn.Format(time.DateOnly)] {
			p.add(f+".starts_on", "Two intakes start on the same day.")
		}
		seen[in.StartsOn.Format(time.DateOnly)] = true
		if in.ApplicationDeadline != nil && in.ApplicationDeadline.After(in.StartsOn.Time) {
			p.add(f+".application_deadline", "The application deadline must be on or before the start date.")
		}
		if in.ApplicationsOpenOn != nil && in.ApplicationDeadline != nil && in.ApplicationsOpenOn.After(in.ApplicationDeadline.Time) {
			p.add(f+".applications_open_on", "Applications must open before the deadline.")
		}
		if in.Capacity != nil && *in.Capacity < 1 {
			p.add(f+".capacity", "Capacity must be at least 1, or left empty.")
		}
	}
	for i, fee := range d.Fees {
		f := fieldf("fees", i)
		p.required(f+".label", fee.Label, 120)
		p.oneOf(f+".category", fee.Category, "tuition", "registration", "assessment", "caution", "other")
		p.oneOf(f+".frequency", fee.Frequency, "once", "per_term", "per_year", "whole_programme")
		if fee.AmountCents < 0 || fee.AmountCents > maxFeeCents {
			p.add(f+".amount_cents", "Enter an amount between KES 0 and KES 10,000,000.")
		}
		p.optional(f+".notes", fee.Notes, 300)
	}
	return p.Err()
}

func fieldf(list string, i int) string { return list + "[" + strconv.Itoa(i) + "]" }

func loadProgramme(ctx context.Context, q *dbq.Queries, id uuid.UUID) (ProgrammeDoc, State, error) {
	r, err := q.StaffGetProgramme(ctx, id)
	if err != nil {
		return ProgrammeDoc{}, State{}, notFound(err)
	}
	d := ProgrammeDoc{
		SchoolID: r.SchoolID, Slug: r.Slug, Title: r.Title, Level: r.Level, Mode: r.Mode, DurationMonths: r.DurationMonths,
		DurationNote: r.DurationNote, MinKCSEGrade: r.MinKcseGrade, Summary: r.Summary, Overview: r.Overview,
		WhatYouLearn: r.WhatYouLearn, CareerOutcomes: r.CareerOutcomes, AssessmentModel: r.AssessmentModel,
		SEOTitle: r.SeoTitle, SEODescription: r.SeoDescription, SortOrder: r.SortOrder,
		Requirements: []Requirement{}, Intakes: []Intake{}, Fees: []Fee{},
	}
	reqs, err := q.ListProgrammeRequirements(ctx, id)
	if err != nil {
		return d, State{}, err
	}
	for _, rq := range reqs {
		d.Requirements = append(d.Requirements, Requirement{Kind: rq.Kind, Requirement: rq.Requirement})
	}
	intakes, err := q.ListProgrammeIntakes(ctx, id)
	if err != nil {
		return d, State{}, err
	}
	for _, in := range intakes {
		inID := in.ID
		d.Intakes = append(d.Intakes, Intake{ID: &inID, Name: in.Name, StartsOn: Date{in.StartsOn},
			ApplicationsOpenOn: datePtr(in.ApplicationsOpenOn), ApplicationDeadline: datePtr(in.ApplicationDeadline),
			Capacity: in.Capacity, Status: in.Status})
	}
	fees, err := q.ListProgrammeFeeItems(ctx, id)
	if err != nil {
		return d, State{}, err
	}
	for _, f := range fees {
		d.Fees = append(d.Fees, Fee{Label: f.Label, Category: f.Category, AmountCents: f.AmountCents, Frequency: f.Frequency, Notes: f.Notes})
	}
	return d, State{Status: r.Status, PublishedAt: r.PublishedAt, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}, nil
}

// saveProgrammeChildren replaces requirements and fees, and updates intakes
// in place (keeping their IDs, which applications use).
func saveProgrammeChildren(ctx context.Context, q *dbq.Queries, id uuid.UUID, d ProgrammeDoc) error {
	if err := q.DeleteProgrammeRequirements(ctx, id); err != nil {
		return err
	}
	for i, r := range d.Requirements {
		if err := q.InsertProgrammeRequirement(ctx, dbq.InsertProgrammeRequirementParams{ProgrammeID: id, Kind: r.Kind, Requirement: r.Requirement, SortOrder: int32(i)}); err != nil {
			return err
		}
	}

	keep := []uuid.UUID{}
	for _, in := range d.Intakes {
		if in.ID != nil {
			keep = append(keep, *in.ID)
		}
	}
	// Remove dropped intakes first so a re-added start date does not clash.
	if err := q.DeleteProgrammeIntakesExcept(ctx, dbq.DeleteProgrammeIntakesExceptParams{ProgrammeID: id, KeepIds: keep}); err != nil {
		return err
	}
	for _, in := range d.Intakes {
		if in.ID != nil {
			n, err := q.UpdateProgrammeIntake(ctx, dbq.UpdateProgrammeIntakeParams{ID: *in.ID, ProgrammeID: id, Name: in.Name, StartsOn: in.StartsOn.Time,
				ApplicationsOpenOn: timePtr(in.ApplicationsOpenOn), ApplicationDeadline: timePtr(in.ApplicationDeadline), Capacity: in.Capacity, Status: in.Status})
			if err != nil {
				return err
			}
			if n == 0 {
				return &ValidationError{Problems: Problems{{Field: "intakes", Message: "An intake in this form no longer exists. Reload and try again."}}}
			}
			continue
		}
		if _, err := q.InsertProgrammeIntake(ctx, dbq.InsertProgrammeIntakeParams{ProgrammeID: id, Name: in.Name, StartsOn: in.StartsOn.Time,
			ApplicationsOpenOn: timePtr(in.ApplicationsOpenOn), ApplicationDeadline: timePtr(in.ApplicationDeadline), Capacity: in.Capacity, Status: in.Status}); err != nil {
			return err
		}
	}

	if err := q.DeleteFeeItems(ctx, id); err != nil {
		return err
	}
	for i, f := range d.Fees {
		if err := q.InsertFeeItem(ctx, dbq.InsertFeeItemParams{ProgrammeID: id, Label: f.Label, Category: f.Category, AmountCents: f.AmountCents,
			Frequency: f.Frequency, Notes: f.Notes, SortOrder: int32(i)}); err != nil {
			return err
		}
	}
	return nil
}

var programmeKind = def[ProgrammeDoc]{
	name: "programmes", workflow: true,
	list: func(ctx context.Context, q *dbq.Queries) ([]Summary, error) {
		rows, err := q.StaffListProgrammes(ctx)
		return summaries(rows, err, func(r dbq.StaffListProgrammesRow) Summary {
			return Summary{ID: r.ID, Label: r.Label, Context: r.SchoolName, Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}
		})
	},
	load: loadProgramme,
	create: func(ctx context.Context, q *dbq.Queries, d ProgrammeDoc, actor uuid.UUID) (uuid.UUID, error) {
		id, err := q.StaffInsertProgramme(ctx, dbq.StaffInsertProgrammeParams{
			SchoolID: d.SchoolID, Slug: d.Slug, Title: d.Title, Level: d.Level, Mode: d.Mode, DurationMonths: d.DurationMonths,
			DurationNote: d.DurationNote, MinKcseGrade: d.MinKCSEGrade, Summary: d.Summary, Overview: d.Overview,
			WhatYouLearn: d.WhatYouLearn, CareerOutcomes: d.CareerOutcomes, AssessmentModel: d.AssessmentModel,
			SeoTitle: d.SEOTitle, SeoDescription: d.SEODescription, SortOrder: d.SortOrder, Actor: &actor,
		})
		if err != nil {
			return uuid.Nil, err
		}
		return id, saveProgrammeChildren(ctx, q, id, d)
	},
	save: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, d ProgrammeDoc, actor uuid.UUID) error {
		if err := q.StaffUpdateProgramme(ctx, dbq.StaffUpdateProgrammeParams{
			ID: id, SchoolID: d.SchoolID, Slug: d.Slug, Title: d.Title, Level: d.Level, Mode: d.Mode, DurationMonths: d.DurationMonths,
			DurationNote: d.DurationNote, MinKcseGrade: d.MinKCSEGrade, Summary: d.Summary, Overview: d.Overview,
			WhatYouLearn: d.WhatYouLearn, CareerOutcomes: d.CareerOutcomes, AssessmentModel: d.AssessmentModel,
			SeoTitle: d.SEOTitle, SeoDescription: d.SEODescription, SortOrder: d.SortOrder, Actor: &actor,
		}); err != nil {
			return err
		}
		return saveProgrammeChildren(ctx, q, id, d)
	},
	setStatus: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, at *time.Time, actor uuid.UUID) error {
		return q.StaffSetProgrammeStatus(ctx, dbq.StaffSetProgrammeStatusParams{ID: id, Status: status, PublishedAt: at, Actor: &actor})
	},
	// Intakes go first; if an application uses one, the foreign key refuses
	// and the whole delete is rolled back (ErrInUse).
	remove: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) error {
		if err := q.DeleteProgrammeIntakesExcept(ctx, dbq.DeleteProgrammeIntakesExceptParams{ProgrammeID: id, KeepIds: []uuid.UUID{}}); err != nil {
			return err
		}
		return q.StaffDeleteProgramme(ctx, id)
	},
	// A published programme answers the five questions (CLAUDE.md section 2).
	readyToPublish: func(ctx context.Context, q *dbq.Queries, d ProgrammeDoc) error {
		var p Problems
		if d.Summary == "" {
			p.add("summary", "Add a short summary: it appears in the course finder.")
		}
		if d.WhatYouLearn == "" {
			p.add("what_you_learn", "Say what students will learn.")
		}
		if d.DurationMonths == nil {
			p.add("duration_months", "Say how long the programme takes.")
		}
		if len(d.Requirements) == 0 {
			p.add("requirements", "List at least one entry requirement.")
		}
		if len(d.Fees) == 0 {
			p.add("fees", "List the fees, so applicants know what it costs.")
		}
		school, err := q.StaffGetSchool(ctx, d.SchoolID)
		if err != nil {
			return notFound(err)
		}
		if school.Status != "published" {
			p.add("school_id", "Publish the school first: programmes of an unpublished school are not shown.")
		}
		return p.Err()
	},
}
