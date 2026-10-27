package finance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// Fee structures (D-157): what each programme charges, per intake or for
// every intake. Invoices copy the lines when they are made, so editing a
// structure never changes an invoice that exists.

type StructureSummary struct {
	ID             uuid.UUID  `json:"id"`
	ProgrammeID    uuid.UUID  `json:"programme_id"`
	ProgrammeTitle string     `json:"programme_title"`
	IntakeID       *uuid.UUID `json:"intake_id"`
	IntakeName     *string    `json:"intake_name"`
	OnceCents      int64      `json:"once_cents"`
	PerTermCents   int64      `json:"per_term_cents"`
	IsSample       bool       `json:"is_sample"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type Structure struct {
	StructureSummary
	Note          string          `json:"note"`
	UpdatedByName *string         `json:"updated_by_name"`
	Lines         []StructureLine `json:"lines"`
	Instalments   []Instalment    `json:"instalments"`
	// Warnings from copying the published fees, shown once after creation.
	Warnings []string `json:"warnings,omitempty"`
}

func (s *Service) ListStructures(ctx context.Context) ([]StructureSummary, error) {
	rows, err := dbq.New(s.pool).ListFeeStructures(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]StructureSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, StructureSummary{ID: r.ID, ProgrammeID: r.ProgrammeID, ProgrammeTitle: r.ProgrammeTitle, IntakeID: r.IntakeID,
			IntakeName: r.IntakeName, OnceCents: r.OnceCents, PerTermCents: r.PerTermCents, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt})
	}
	return out, nil
}

func structureLines(ctx context.Context, q *dbq.Queries, id uuid.UUID) ([]StructureLine, []Instalment, error) {
	rows, err := q.FeeStructureLines(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	lines := make([]StructureLine, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, StructureLine{Label: r.Label, Category: r.Category, Charge: r.Charge, AmountCents: r.AmountCents})
	}
	ins, err := q.FeeStructureInstalments(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	plan := make([]Instalment, 0, len(ins))
	for _, n := range ins {
		plan = append(plan, Instalment{Percent: int(n.Percent), DueDays: int(n.DueDays)})
	}
	return lines, plan, nil
}

func getStructure(ctx context.Context, q *dbq.Queries, id uuid.UUID) (*Structure, error) {
	r, err := q.GetFeeStructure(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out := &Structure{StructureSummary: StructureSummary{ID: r.ID, ProgrammeID: r.ProgrammeID, ProgrammeTitle: r.ProgrammeTitle,
		IntakeID: r.IntakeID, IntakeName: r.IntakeName, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt},
		Note: r.Note, UpdatedByName: r.UpdatedByName}
	if out.Lines, out.Instalments, err = structureLines(ctx, q, id); err != nil {
		return nil, err
	}
	for _, l := range out.Lines {
		if l.Charge == "once" {
			out.OnceCents += l.AmountCents
		} else {
			out.PerTermCents += l.AmountCents
		}
	}
	return out, nil
}

func (s *Service) GetStructure(ctx context.Context, id uuid.UUID) (*Structure, error) {
	return getStructure(ctx, dbq.New(s.pool), id)
}

func writeStructureLines(ctx context.Context, q *dbq.Queries, id uuid.UUID, lines []StructureLine, plan []Instalment) error {
	if err := q.DeleteFeeStructureLines(ctx, id); err != nil {
		return err
	}
	if err := q.DeleteFeeStructureInstalments(ctx, id); err != nil {
		return err
	}
	for i, l := range lines {
		if err := q.InsertFeeStructureLine(ctx, dbq.InsertFeeStructureLineParams{StructureID: id, Label: l.Label, Category: l.Category,
			Charge: l.Charge, AmountCents: l.AmountCents, SortOrder: int32(i)}); err != nil {
			return err
		}
	}
	for i, n := range plan {
		if err := q.InsertFeeStructureInstalment(ctx, dbq.InsertFeeStructureInstalmentParams{StructureID: id, Seq: int16(i + 1),
			Percent: int16(n.Percent), DueDays: int16(n.DueDays)}); err != nil {
			return err
		}
	}
	return nil
}

// fromPublished turns the website's fee lines into structure lines. Lines
// charged per year or for the whole programme have no exact match, so they
// are copied as charged once, with a warning to check them.
func fromPublished(rows []dbq.PublishedFeeLinesRow) ([]StructureLine, []string) {
	var lines []StructureLine
	var warnings []string
	for _, r := range rows {
		charge := "once"
		switch r.Frequency {
		case "per_term":
			charge = "per_term"
		case "per_year", "whole_programme":
			label := map[string]string{"per_year": "per year", "whole_programme": "for the whole programme"}[r.Frequency]
			warnings = append(warnings, fmt.Sprintf("“%s” is published as charged %s. It was copied as charged once, at enrolment; change it if the college bills it differently.", r.Label, label))
		}
		lines = append(lines, StructureLine{Label: r.Label, Category: r.Category, Charge: charge, AmountCents: r.AmountCents})
		if len(lines) == 30 {
			break
		}
	}
	return lines, warnings
}

// CreateStructure makes the structure for a programme and intake (nil:
// every intake). With fromPublished, it starts from the fees published on
// the programme's page; otherwise it starts empty.
func (s *Service) CreateStructure(ctx context.Context, p *auth.Principal, programme uuid.UUID, intake *uuid.UUID, copyPublished bool, req audit.Request) (*Structure, error) {
	var out *Structure
	err := s.tx(ctx, func(q *dbq.Queries) error {
		chk, err := q.ProgrammeIntakeValid(ctx, dbq.ProgrammeIntakeValidParams{ProgrammeID: programme, IntakeID: intake})
		if errors.Is(err, pgx.ErrNoRows) {
			return content.Problems{{Field: "programme_id", Message: "Choose a programme."}}.Err()
		}
		if err != nil {
			return err
		}
		if !chk.IntakeOk {
			return content.Problems{{Field: "intake_id", Message: "Choose an intake of this programme, or every intake."}}.Err()
		}
		id, err := q.InsertFeeStructure(ctx, dbq.InsertFeeStructureParams{ProgrammeID: programme, IntakeID: intake, IsSample: chk.IsSample, UpdatedBy: &p.UserID})
		if isUniqueViolation(err) {
			return content.Problems{{Field: "intake_id", Message: "This programme already has a fee structure for that intake. Open it from the list."}}.Err()
		}
		if err != nil {
			return err
		}
		var warnings []string
		if copyPublished {
			rows, err := q.PublishedFeeLines(ctx, dbq.PublishedFeeLinesParams{ProgrammeID: programme, IntakeID: intake})
			if err != nil {
				return err
			}
			var lines []StructureLine
			lines, warnings = fromPublished(rows)
			if err := writeStructureLines(ctx, q, id, lines, nil); err != nil {
				return err
			}
		}
		if out, err = getStructure(ctx, q, id); err != nil {
			return err
		}
		out.Warnings = warnings
		return audit.Write(ctx, q, req, audit.Entry{Action: "finance.structure_created", EntityType: "fee_structure", EntityID: id.String(),
			Details: map[string]any{"programme_id": programme.String(), "from_published": copyPublished, "lines": len(out.Lines)}})
	})
	return out, err
}

type StructureInput struct {
	Note        string          `json:"note"`
	Lines       []StructureLine `json:"lines"`
	Instalments []Instalment    `json:"instalments"`
}

// UpdateStructure replaces a structure's lines and instalment plan.
func (s *Service) UpdateStructure(ctx context.Context, p *auth.Principal, id uuid.UUID, in StructureInput, req audit.Request) (*Structure, error) {
	var pr content.Problems
	lines := CheckStructureLines(&pr, in.Lines)
	CheckInstalments(&pr, in.Instalments)
	if len([]rune(in.Note)) > 1000 {
		pr.Add("note", "Use 1,000 characters or fewer.")
	}
	if err := pr.Err(); err != nil {
		return nil, err
	}
	var out *Structure
	err := s.tx(ctx, func(q *dbq.Queries) error {
		if _, err := q.LockFeeStructure(ctx, id); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if err := q.UpdateFeeStructure(ctx, dbq.UpdateFeeStructureParams{ID: id, Note: tidy(in.Note, 1000), UpdatedBy: &p.UserID}); err != nil {
			return err
		}
		if err := writeStructureLines(ctx, q, id, lines, in.Instalments); err != nil {
			return err
		}
		var err error
		if out, err = getStructure(ctx, q, id); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "finance.structure_updated", EntityType: "fee_structure", EntityID: id.String(),
			Details: map[string]any{"lines": len(lines), "once_cents": out.OnceCents, "per_term_cents": out.PerTermCents, "instalments": len(in.Instalments)}})
	})
	return out, err
}

// DeleteStructure removes a structure. Invoices made from it keep their lines.
func (s *Service) DeleteStructure(ctx context.Context, id uuid.UUID, req audit.Request) error {
	return s.tx(ctx, func(q *dbq.Queries) error {
		n, err := q.DeleteFeeStructure(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "finance.structure_deleted", EntityType: "fee_structure", EntityID: id.String()})
	})
}

// structureFor is the structure that bills a student: their intake's own,
// else their programme's structure for every intake.
func structureFor(ctx context.Context, q *dbq.Queries, programme uuid.UUID, intake *uuid.UUID) (uuid.UUID, bool, error) {
	id, err := q.FeeStructureFor(ctx, dbq.FeeStructureForParams{ProgrammeID: programme, IntakeID: intake})
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	return id, err == nil, err
}
