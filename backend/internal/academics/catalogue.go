package academics

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// ---------- terms (college-wide: changes need schools.all) ----------

type Term struct {
	ID                   uuid.UUID `json:"id"`
	Name                 string    `json:"name"`
	StartsOn             string    `json:"starts_on"`
	EndsOn               string    `json:"ends_on"`
	RegistrationOpensOn  *string   `json:"registration_opens_on"`
	RegistrationClosesOn *string   `json:"registration_closes_on"`
	IsSample             bool      `json:"is_sample"`
}

type TermInput struct {
	Name                 string `json:"name"`
	StartsOn             string `json:"starts_on"`
	EndsOn               string `json:"ends_on"`
	RegistrationOpensOn  string `json:"registration_opens_on"`
	RegistrationClosesOn string `json:"registration_closes_on"`
}

func termFrom(id uuid.UUID, name string, starts, ends time.Time, opens, closes *time.Time, sample bool) Term {
	return Term{ID: id, Name: name, StartsOn: starts.Format(time.DateOnly), EndsOn: ends.Format(time.DateOnly),
		RegistrationOpensOn: dateString(opens), RegistrationClosesOn: dateString(closes), IsSample: sample}
}

func (s *Service) ListTerms(ctx context.Context) ([]Term, error) {
	rows, err := dbq.New(s.pool).ListTerms(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Term, 0, len(rows))
	for _, r := range rows {
		out = append(out, termFrom(r.ID, r.Name, r.StartsOn, r.EndsOn, r.RegistrationOpensOn, r.RegistrationClosesOn, r.IsSample))
	}
	return out, nil
}

func parseDate(pr *content.Problems, field, v string, required bool) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		if required {
			pr.Add(field, "Enter the date.")
		}
		return nil
	}
	d, err := time.Parse(time.DateOnly, v)
	if err != nil {
		pr.Add(field, "Enter the date as YYYY-MM-DD.")
		return nil
	}
	return &d
}

func (in *TermInput) check() (dbq.InsertTermParams, error) {
	var pr content.Problems
	in.Name = strings.TrimSpace(in.Name)
	pr.Required("name", in.Name, 80)
	starts := parseDate(&pr, "starts_on", in.StartsOn, true)
	ends := parseDate(&pr, "ends_on", in.EndsOn, true)
	opens := parseDate(&pr, "registration_opens_on", in.RegistrationOpensOn, false)
	closes := parseDate(&pr, "registration_closes_on", in.RegistrationClosesOn, false)
	if starts != nil && ends != nil && !ends.After(*starts) {
		pr.Add("ends_on", "The term must end after it starts.")
	}
	if opens != nil && closes != nil && closes.Before(*opens) {
		pr.Add("registration_closes_on", "Registration must close on or after the day it opens.")
	}
	if err := pr.Err(); err != nil {
		return dbq.InsertTermParams{}, err
	}
	return dbq.InsertTermParams{Name: in.Name, StartsOn: *starts, EndsOn: *ends, RegistrationOpensOn: opens, RegistrationClosesOn: closes}, nil
}

func termNameTaken(err error) error {
	if isUniqueViolation(err) {
		return content.Problems{{Field: "name", Message: "A term with this name exists. Choose another name."}}.Err()
	}
	return err
}

// CreateTerm adds an academic period.
func (s *Service) CreateTerm(ctx context.Context, sc Scope, in TermInput, req audit.Request) (uuid.UUID, error) {
	if !sc.All {
		return uuid.Nil, ErrCollegeWide
	}
	params, err := in.check()
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err = s.tx(ctx, func(q *dbq.Queries) error {
		if id, err = q.InsertTerm(ctx, params); err != nil {
			return termNameTaken(err)
		}
		return audit.Write(ctx, q, req, entry("term.created", "academic_term", id, map[string]any{"name": params.Name}))
	})
	return id, err
}

// UpdateTerm changes a term's name or dates.
func (s *Service) UpdateTerm(ctx context.Context, sc Scope, id uuid.UUID, in TermInput, req audit.Request) error {
	if !sc.All {
		return ErrCollegeWide
	}
	p, err := in.check()
	if err != nil {
		return err
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		n, err := q.UpdateTerm(ctx, dbq.UpdateTermParams{ID: id, Name: p.Name, StartsOn: p.StartsOn, EndsOn: p.EndsOn,
			RegistrationOpensOn: p.RegistrationOpensOn, RegistrationClosesOn: p.RegistrationClosesOn})
		if err != nil {
			return termNameTaken(err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return audit.Write(ctx, q, req, entry("term.updated", "academic_term", id, map[string]any{"name": p.Name}))
	})
}

// ---------- units ----------

type Unit struct {
	ID          uuid.UUID `json:"id"`
	SchoolID    uuid.UUID `json:"school_id"`
	SchoolName  string    `json:"school_name"`
	Code        string    `json:"code"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Hours       *int32    `json:"hours"`
	Active      bool      `json:"active"`
	IsSample    bool      `json:"is_sample"`
}

type UnitInput struct {
	SchoolID    uuid.UUID `json:"school_id"`
	Code        string    `json:"code"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Hours       *int32    `json:"hours"`
	Active      *bool     `json:"active"`
}

var unitCode = regexp.MustCompile(`^[A-Z0-9][A-Z0-9 /-]{1,19}$`)

func (in *UnitInput) check() error {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Title, in.Description = strings.TrimSpace(in.Title), strings.TrimSpace(in.Description)
	var pr content.Problems
	if !unitCode.MatchString(in.Code) {
		pr.Add("code", "Use 2 to 20 capital letters, digits, spaces, slashes or dashes, like ICT 101.")
	}
	pr.Required("title", in.Title, 160)
	pr.Optional("description", in.Description, 2000)
	if in.Hours != nil && (*in.Hours < 1 || *in.Hours > 2000) {
		pr.Add("hours", "Enter the hours as a whole number from 1 to 2000, or leave it empty.")
	}
	return pr.Err()
}

func unitCodeTaken(err error) error {
	if isUniqueViolation(err) {
		return content.Problems{{Field: "code", Message: "Another unit has this code."}}.Err()
	}
	return err
}

// ListUnits is the unit catalogue; any member of staff may read it.
func (s *Service) ListUnits(ctx context.Context, school *uuid.UUID, search string) ([]Unit, error) {
	rows, err := dbq.New(s.pool).ListUnits(ctx, dbq.ListUnitsParams{SchoolID: school, Search: optionalText(search)})
	if err != nil {
		return nil, err
	}
	out := make([]Unit, 0, len(rows))
	for _, r := range rows {
		out = append(out, Unit{ID: r.ID, SchoolID: r.SchoolID, SchoolName: r.SchoolName, Code: r.Code, Title: r.Title,
			Description: r.Description, Hours: r.Hours, Active: r.Active, IsSample: r.IsSample})
	}
	return out, nil
}

func (s *Service) GetUnit(ctx context.Context, id uuid.UUID) (*Unit, error) {
	r, err := dbq.New(s.pool).GetUnit(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &Unit{ID: r.ID, SchoolID: r.SchoolID, SchoolName: r.SchoolName, Code: r.Code, Title: r.Title,
		Description: r.Description, Hours: r.Hours, Active: r.Active, IsSample: r.IsSample}, nil
}

// CreateUnit adds a unit to a school in scope.
func (s *Service) CreateUnit(ctx context.Context, sc Scope, in UnitInput, req audit.Request) (uuid.UUID, error) {
	if err := in.check(); err != nil {
		return uuid.Nil, err
	}
	if !sc.Covers(in.SchoolID) {
		return uuid.Nil, ErrOutsideScope
	}
	var id uuid.UUID
	err := s.tx(ctx, func(q *dbq.Queries) error {
		var err error
		if id, err = q.InsertUnit(ctx, dbq.InsertUnitParams{SchoolID: in.SchoolID, Code: in.Code, Title: in.Title,
			Description: in.Description, Hours: in.Hours}); err != nil {
			if isForeignKey(err) {
				return content.Problems{{Field: "school_id", Message: "Choose a school from the list."}}.Err()
			}
			return unitCodeTaken(err)
		}
		return audit.Write(ctx, q, req, entry("unit.created", "unit", id, map[string]any{"code": in.Code}))
	})
	return id, err
}

// UpdateUnit edits a unit of a school in scope. Its school never changes.
func (s *Service) UpdateUnit(ctx context.Context, sc Scope, id uuid.UUID, in UnitInput, req audit.Request) error {
	cur, err := s.GetUnit(ctx, id)
	if err != nil {
		return err
	}
	if !sc.Covers(cur.SchoolID) {
		return ErrNotFound
	}
	in.SchoolID = cur.SchoolID
	if err := in.check(); err != nil {
		return err
	}
	active := cur.Active
	if in.Active != nil {
		active = *in.Active
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		if _, err := q.UpdateUnit(ctx, dbq.UpdateUnitParams{ID: id, Code: in.Code, Title: in.Title, Description: in.Description,
			Hours: in.Hours, Active: active}); err != nil {
			return unitCodeTaken(err)
		}
		return audit.Write(ctx, q, req, entry("unit.updated", "unit", id, map[string]any{"code": in.Code, "active": active}))
	})
}

// ---------- programme units ----------

type ProgrammeUnit struct {
	UnitID uuid.UUID `json:"unit_id"`
	Code   string    `json:"code"`
	Title  string    `json:"title"`
	Hours  *int32    `json:"hours"`
	Stage  int16     `json:"stage"`
	IsCore bool      `json:"is_core"`
}

type ProgrammeUnitInput struct {
	UnitID uuid.UUID `json:"unit_id"`
	Stage  int16     `json:"stage"`
	IsCore bool      `json:"is_core"`
}

func (s *Service) ProgrammeUnits(ctx context.Context, programmeID uuid.UUID) ([]ProgrammeUnit, error) {
	q := dbq.New(s.pool)
	if _, err := q.GetProgrammeSchool(ctx, programmeID); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	rows, err := q.ListProgrammeUnits(ctx, programmeID)
	if err != nil {
		return nil, err
	}
	out := make([]ProgrammeUnit, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProgrammeUnit{UnitID: r.UnitID, Code: r.Code, Title: r.Title, Hours: r.Hours, Stage: r.Stage, IsCore: r.IsCore})
	}
	return out, nil
}

// SetProgrammeUnits replaces the unit list of a programme in scope. A unit
// may belong to another school (shared units).
func (s *Service) SetProgrammeUnits(ctx context.Context, sc Scope, programmeID uuid.UUID, items []ProgrammeUnitInput, req audit.Request) error {
	var pr content.Problems
	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(items))
	for _, it := range items {
		if seen[it.UnitID] {
			pr.Add("items", "Each unit can appear once.")
			break
		}
		seen[it.UnitID] = true
		ids = append(ids, it.UnitID)
		if it.Stage < 1 || it.Stage > 10 {
			pr.Add("items", "The stage (year or level) must be from 1 to 10.")
			break
		}
	}
	if err := pr.Err(); err != nil {
		return err
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		prog, err := q.GetProgrammeSchool(ctx, programmeID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !sc.Covers(prog.SchoolID)) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if n, err := q.CountUnits(ctx, ids); err != nil {
			return err
		} else if int(n) != len(ids) {
			return content.Problems{{Field: "items", Message: "Choose units from the list."}}.Err()
		}
		if err := q.DeleteProgrammeUnits(ctx, programmeID); err != nil {
			return err
		}
		for i, it := range items {
			if err := q.InsertProgrammeUnit(ctx, dbq.InsertProgrammeUnitParams{ProgrammeID: programmeID, UnitID: it.UnitID,
				Stage: it.Stage, IsCore: it.IsCore, SortOrder: int32(i)}); err != nil {
				return err
			}
		}
		return audit.Write(ctx, q, req, entry("programme.units_set", "programme", programmeID, map[string]any{"units": len(items)}))
	})
}

// ---------- cohorts ----------

type Cohort struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	ProgrammeID    uuid.UUID  `json:"programme_id"`
	ProgrammeTitle string     `json:"programme_title"`
	IntakeID       *uuid.UUID `json:"intake_id"`
	IntakeName     *string    `json:"intake_name"`
	Students       int64      `json:"students"`
	ActiveStudents int64      `json:"active_students"`
}

func (s *Service) ListCohorts(ctx context.Context, sc Scope, programme *uuid.UUID) ([]Cohort, error) {
	rows, err := dbq.New(s.pool).StaffListCohorts(ctx, dbq.StaffListCohortsParams{AllSchools: sc.All, Schools: sc.Schools, ProgrammeID: programme})
	if err != nil {
		return nil, err
	}
	out := make([]Cohort, 0, len(rows))
	for _, r := range rows {
		out = append(out, Cohort{ID: r.ID, Name: r.Name, ProgrammeID: r.ProgrammeID, ProgrammeTitle: r.ProgrammeTitle,
			IntakeID: r.IntakeID, IntakeName: r.IntakeName, Students: r.Students, ActiveStudents: r.ActiveStudents})
	}
	return out, nil
}

type CohortInput struct {
	ProgrammeID uuid.UUID `json:"programme_id"`
	Name        string    `json:"name"`
}

// CreateCohort adds a cohort without an intake (for example a repeat
// group); intake cohorts are made by enrolment.
func (s *Service) CreateCohort(ctx context.Context, sc Scope, in CohortInput, req audit.Request) (uuid.UUID, error) {
	in.Name = strings.TrimSpace(in.Name)
	var pr content.Problems
	pr.Required("name", in.Name, 120)
	if err := pr.Err(); err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err := s.tx(ctx, func(q *dbq.Queries) error {
		prog, err := q.GetProgrammeSchool(ctx, in.ProgrammeID)
		if errors.Is(err, pgx.ErrNoRows) {
			return content.Problems{{Field: "programme_id", Message: "Choose a programme from the list."}}.Err()
		}
		if err != nil {
			return err
		}
		if !sc.Covers(prog.SchoolID) {
			return ErrOutsideScope
		}
		if id, err = q.InsertCohort(ctx, dbq.InsertCohortParams{ProgrammeID: in.ProgrammeID, Name: in.Name}); err != nil {
			if isUniqueViolation(err) {
				return content.Problems{{Field: "name", Message: "This programme already has a cohort with that name."}}.Err()
			}
			return err
		}
		return audit.Write(ctx, q, req, entry("cohort.created", "cohort", id, map[string]any{"name": in.Name}))
	})
	return id, err
}
