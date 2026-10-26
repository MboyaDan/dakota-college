package results

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/academics"
	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/media"
)

const (
	PermEnter   = "academics.marks.enter"
	PermApprove = "academics.results.approve"
	PermRelease = "academics.results.release"
)

var (
	ErrNotFound = errors.New("not found")
	// ErrWrongStatus: the sheet is not in a status that allows this.
	ErrWrongStatus = errors.New("not allowed at this status")
	// ErrWaiting: marks cannot change while the sheet waits for approval.
	ErrWaiting    = errors.New("waiting for approval")
	ErrNoStudent  = errors.New("no student record")
	ErrStorageOff = errors.New("storage not configured")
)

var nairobi = time.FixedZone("EAT", 3*60*60)

type Service struct {
	pool    *pgxpool.Pool
	logger  *slog.Logger
	now     func() time.Time
	storage media.Storage
	bucket  string
}

type Options struct {
	Pool          *pgxpool.Pool
	Logger        *slog.Logger
	Storage       media.Storage
	PrivateBucket string
	Now           func() time.Time
}

func NewService(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Service{pool: o.Pool, logger: o.Logger, now: now, storage: o.Storage, bucket: o.PrivateBucket}
}

func (s *Service) tx(ctx context.Context, fn func(q *dbq.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(dbq.New(tx)) })
}

// ---------- schemes ----------

func schemeOf(ctx context.Context, q *dbq.Queries, programme uuid.UUID) (Scheme, error) {
	r, err := q.GetAssessmentScheme(ctx, programme)
	if errors.Is(err, pgx.ErrNoRows) {
		return Default(), nil
	}
	if err != nil {
		return Scheme{}, err
	}
	sc := Scheme{Kind: r.Kind, PassMark: int(r.PassMark)}
	if err := json.Unmarshal(r.Components, &sc.Components); err != nil {
		return sc, err
	}
	return sc, json.Unmarshal(r.GradeBands, &sc.Bands)
}

// SchemeFor is a programme's scheme (the default until one is set).
func (s *Service) SchemeFor(ctx context.Context, programme uuid.UUID) (*Scheme, error) {
	q := dbq.New(s.pool)
	if _, err := q.GetProgrammeSchool(ctx, programme); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	sc, err := schemeOf(ctx, q, programme)
	return &sc, err
}

// SetScheme sets a programme's scheme. Sheets already holding marks keep the
// scheme they started with.
func (s *Service) SetScheme(ctx context.Context, p *auth.Principal, sc academics.Scope, programme uuid.UUID, in Scheme, req audit.Request) error {
	if err := in.Normalise(); err != nil {
		return err
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		prog, err := q.GetProgrammeSchool(ctx, programme)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !sc.Covers(prog.SchoolID)) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		comps, _ := json.Marshal(in.Components)
		bands, _ := json.Marshal(in.Bands)
		if err := q.UpsertAssessmentScheme(ctx, dbq.UpsertAssessmentSchemeParams{ProgrammeID: programme, Kind: in.Kind, Components: comps,
			PassMark: int16(in.PassMark), GradeBands: bands, UpdatedBy: &p.UserID}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "results.scheme_set", EntityType: "programme", EntityID: programme.String(),
			Details: map[string]any{"kind": in.Kind}})
	})
}

// ---------- sheets ----------

type Event struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Reason    string    `json:"reason"`
	At        time.Time `json:"at"`
	ActorName string    `json:"actor_name"`
}

type Row struct {
	StudentID     uuid.UUID       `json:"student_id"`
	StudentNumber string          `json:"student_number"`
	Name          string          `json:"name"`
	Marks         map[string]Mark `json:"marks"`
	Result        Result          `json:"result"`
}

type Sheet struct {
	ID         uuid.UUID `json:"id"`
	OfferingID uuid.UUID `json:"offering_id"`
	Status     string    `json:"status"`
	Scheme     Scheme    `json:"scheme"`
	UnitCode   string    `json:"unit_code"`
	UnitTitle  string    `json:"unit_title"`
	TermName   string    `json:"term_name"`
	Rows       []Row     `json:"rows"`
	// Complete: every registered student has every mark.
	Complete        bool      `json:"complete"`
	Events          []Event   `json:"events"`
	ReleasedVersion int32     `json:"released_version"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ensureSheet makes the sheet of a class, with the scheme of its programme.
// A sheet without marks follows changes to the scheme.
func ensureSheet(ctx context.Context, q *dbq.Queries, offering uuid.UUID) (uuid.UUID, error) {
	sc := Default()
	if prog, err := q.ProgrammeForOffering(ctx, offering); err == nil {
		if sc, err = schemeOf(ctx, q, prog); err != nil {
			return uuid.Nil, err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	raw, _ := json.Marshal(sc)
	if err := q.InsertSheet(ctx, dbq.InsertSheetParams{OfferingID: offering, Scheme: raw}); err != nil {
		return uuid.Nil, err
	}
	sh, err := q.GetSheetByOffering(ctx, offering)
	if err != nil {
		return uuid.Nil, err
	}
	if n, err := q.CountMarks(ctx, sh.ID); err != nil {
		return uuid.Nil, err
	} else if n == 0 && sh.Status == "draft" && string(sh.Scheme) != string(raw) {
		if err := q.SetSheetScheme(ctx, dbq.SetSheetSchemeParams{ID: sh.ID, Scheme: raw}); err != nil {
			return uuid.Nil, err
		}
	}
	return sh.ID, nil
}

func fullName(first, middle, last string) string {
	return strings.Join(strings.Fields(first+" "+middle+" "+last), " ")
}

// sheet builds the view of a sheet: one row per registered student with
// their current marks and computed result.
func sheet(ctx context.Context, q *dbq.Queries, id uuid.UUID, offering uuid.UUID) (*Sheet, error) {
	r, err := q.GetSheetOffering(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &Sheet{ID: r.ID, OfferingID: r.OfferingID, Status: r.Status, UnitCode: r.UnitCode, UnitTitle: r.UnitTitle, TermName: r.TermName,
		Rows: []Row{}, Events: []Event{}, UpdatedAt: r.UpdatedAt, Complete: true}
	if err := json.Unmarshal(r.Scheme, &out.Scheme); err != nil {
		return nil, err
	}
	students, err := q.ListClassStudents(ctx, offering)
	if err != nil {
		return nil, err
	}
	marks, err := q.CurrentMarks(ctx, id)
	if err != nil {
		return nil, err
	}
	byStudent := map[uuid.UUID]map[string]Mark{}
	for _, m := range marks {
		if byStudent[m.StudentID] == nil {
			byStudent[m.StudentID] = map[string]Mark{}
		}
		mk := Mark{Outcome: m.Outcome}
		if m.Score != nil {
			v := int(*m.Score)
			mk.Score = &v
		}
		byStudent[m.StudentID][m.Component] = mk
	}
	for _, st := range students {
		ms := byStudent[st.ID]
		if ms == nil {
			ms = map[string]Mark{}
		}
		row := Row{StudentID: st.ID, StudentNumber: st.StudentNumber, Name: fullName(st.FirstName, st.MiddleName, st.LastName),
			Marks: ms, Result: Compute(out.Scheme, ms)}
		out.Complete = out.Complete && row.Result.Complete
		out.Rows = append(out.Rows, row)
	}
	evs, err := q.ListSheetEvents(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, e := range evs {
		out.Events = append(out.Events, Event{From: e.FromStatus, To: e.ToStatus, Reason: e.Reason, At: e.CreatedAt, ActorName: e.ActorName})
	}
	if v, err := q.NextReleaseVersion(ctx, id); err != nil {
		return nil, err
	} else {
		out.ReleasedVersion = v - 1
	}
	return out, nil
}

func (s *Service) assigned(ctx context.Context, q *dbq.Queries, user, offering uuid.UUID) error {
	ok, err := q.IsAssignedTrainer(ctx, dbq.IsAssignedTrainerParams{OfferingID: offering, UserID: user})
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// TrainerSheet is the result sheet of a class the trainer teaches.
func (s *Service) TrainerSheet(ctx context.Context, user, offering uuid.UUID) (*Sheet, error) {
	var out *Sheet
	err := s.tx(ctx, func(q *dbq.Queries) error {
		if err := s.assigned(ctx, q, user, offering); err != nil {
			return err
		}
		id, err := ensureSheet(ctx, q, offering)
		if err != nil {
			return err
		}
		out, err = sheet(ctx, q, id, offering)
		return err
	})
	return out, err
}

func move(ctx context.Context, q *dbq.Queries, id uuid.UUID, from, to, reason string, actor uuid.UUID) error {
	if err := q.SetSheetStatus(ctx, dbq.SetSheetStatusParams{ID: id, Status: to}); err != nil {
		return err
	}
	return q.InsertSheetEvent(ctx, dbq.InsertSheetEventParams{SheetID: id, FromStatus: from, ToStatus: to, Reason: reason, ActorID: actor})
}

// Entry is one mark the trainer enters.
type Entry struct {
	StudentID uuid.UUID `json:"student_id"`
	Component string    `json:"component"`
	Score     *int      `json:"score"`
	Outcome   *string   `json:"outcome"`
}

// EnterMarks records marks for a class the trainer teaches. While the sheet
// waits for approval nothing changes; after approval (or release) a
// correction needs a reason and sends the sheet back to draft.
func (s *Service) EnterMarks(ctx context.Context, user, offering uuid.UUID, entries []Entry, reason string, req audit.Request) error {
	reason = strings.TrimSpace(reason)
	if len(entries) == 0 || len(entries) > 2000 {
		return content.Problems{{Field: "entries", Message: "Enter at least one mark."}}.Err()
	}
	if len([]rune(reason)) > 500 {
		return content.Problems{{Field: "reason", Message: "Keep the reason under 500 characters."}}.Err()
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		if err := s.assigned(ctx, q, user, offering); err != nil {
			return err
		}
		id, err := ensureSheet(ctx, q, offering)
		if err != nil {
			return err
		}
		sh, err := q.LockSheet(ctx, id)
		if err != nil {
			return err
		}
		var sc Scheme
		if err := json.Unmarshal(sh.Scheme, &sc); err != nil {
			return err
		}
		switch sh.Status {
		case "submitted":
			return ErrWaiting
		case "approved", "released":
			if reason == "" {
				return content.Problems{{Field: "reason", Message: "These results are approved. Say why you are correcting them."}}.Err()
			}
			if err := move(ctx, q, id, sh.Status, "draft", "Correction: "+reason, user); err != nil {
				return err
			}
		}
		students, err := q.ListClassStudents(ctx, offering)
		if err != nil {
			return err
		}
		var pr content.Problems
		for i, e := range entries {
			field := fmt.Sprintf("entries.%d", i)
			if !slices.ContainsFunc(students, func(st dbq.ListClassStudentsRow) bool { return st.ID == e.StudentID }) {
				pr.Add(field, "This student is not on the class list.")
				continue
			}
			if !slices.ContainsFunc(sc.Components, func(c Component) bool { return c.Key == e.Component }) {
				pr.Add(field, "This part is not in the assessment.")
				continue
			}
			p := dbq.InsertMarkParams{SheetID: id, StudentID: e.StudentID, Component: e.Component, Reason: reason, EnteredBy: user}
			if sc.Kind == "competency" {
				if e.Outcome == nil || (*e.Outcome != "competent" && *e.Outcome != "not_yet_competent") {
					pr.Add(field, "Choose Competent or Not yet competent.")
					continue
				}
				p.Outcome = e.Outcome
			} else {
				if e.Score == nil || *e.Score < 0 || *e.Score > 100 {
					pr.Add(field, "Enter a whole number from 0 to 100.")
					continue
				}
				v := int16(*e.Score)
				p.Score = &v
			}
			if err := q.InsertMark(ctx, p); err != nil {
				return err
			}
		}
		if err := pr.Err(); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "results.marks_entered", EntityType: "result_sheet", EntityID: id.String(),
			Details: map[string]any{"marks": len(entries), "correction": reason != "", "unit": sh.UnitCode}})
	})
}

// Submit sends a complete sheet to the Head of School.
func (s *Service) Submit(ctx context.Context, user, offering uuid.UUID, req audit.Request) error {
	return s.tx(ctx, func(q *dbq.Queries) error {
		if err := s.assigned(ctx, q, user, offering); err != nil {
			return err
		}
		id, err := ensureSheet(ctx, q, offering)
		if err != nil {
			return err
		}
		sh, err := q.LockSheet(ctx, id)
		if err != nil {
			return err
		}
		if sh.Status != "draft" {
			return ErrWrongStatus
		}
		view, err := sheet(ctx, q, id, offering)
		if err != nil {
			return err
		}
		if len(view.Rows) == 0 {
			return content.Problems{{Field: "entries", Message: "Nobody is registered for this class."}}.Err()
		}
		var missing []string
		for _, r := range view.Rows {
			if !r.Result.Complete {
				missing = append(missing, r.StudentNumber)
			}
		}
		if len(missing) > 0 {
			return content.Problems{{Field: "entries", Message: "Marks are missing for " + strings.Join(missing, ", ") + "."}}.Err()
		}
		if err := move(ctx, q, id, "draft", "submitted", "", user); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "results.submitted", EntityType: "result_sheet", EntityID: id.String(),
			Details: map[string]any{"unit": sh.UnitCode, "students": len(view.Rows)}})
	})
}

// ---------- approval and release ----------

type SheetItem struct {
	ID              uuid.UUID `json:"id"`
	OfferingID      uuid.UUID `json:"offering_id"`
	Status          string    `json:"status"`
	TermName        string    `json:"term_name"`
	UnitCode        string    `json:"unit_code"`
	UnitTitle       string    `json:"unit_title"`
	Students        int64     `json:"students"`
	ReleasedVersion int32     `json:"released_version"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ListSheets lists sheets in scope with one of the statuses.
func (s *Service) ListSheets(ctx context.Context, sc academics.Scope, statuses []string) ([]SheetItem, error) {
	rows, err := dbq.New(s.pool).StaffListSheets(ctx, dbq.StaffListSheetsParams{AllSchools: sc.All, Schools: sc.Schools, Statuses: statuses})
	if err != nil {
		return nil, err
	}
	out := make([]SheetItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, SheetItem{ID: r.ID, OfferingID: r.OfferingID, Status: r.Status, TermName: r.TermName, UnitCode: r.UnitCode,
			UnitTitle: r.UnitTitle, Students: r.Students, ReleasedVersion: r.ReleasedVersion, UpdatedAt: r.UpdatedAt})
	}
	return out, nil
}

// GetSheet is one sheet in scope.
func (s *Service) GetSheet(ctx context.Context, sc academics.Scope, id uuid.UUID) (*Sheet, error) {
	q := dbq.New(s.pool)
	r, err := q.GetSheetOffering(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !sc.Covers(r.SchoolID)) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return sheet(ctx, q, id, r.OfferingID)
}

// decide locks a sheet in scope in status from and moves it.
func (s *Service) decide(ctx context.Context, p *auth.Principal, sc academics.Scope, id uuid.UUID, from, to, reason, action string, req audit.Request, extra func(q *dbq.Queries, sh dbq.LockSheetRow) error) error {
	return s.tx(ctx, func(q *dbq.Queries) error {
		sh, err := q.LockSheet(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !sc.Covers(sh.SchoolID)) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if sh.Status != from {
			return ErrWrongStatus
		}
		if extra != nil {
			if err := extra(q, sh); err != nil {
				return err
			}
		}
		if err := move(ctx, q, id, from, to, reason, p.UserID); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: action, EntityType: "result_sheet", EntityID: id.String(),
			Details: map[string]any{"unit": sh.UnitCode}})
	})
}

// Approve is the Head of School's approval of submitted results.
func (s *Service) Approve(ctx context.Context, p *auth.Principal, sc academics.Scope, id uuid.UUID, req audit.Request) error {
	return s.decide(ctx, p, sc, id, "submitted", "approved", "", "results.approved", req, nil)
}

// Return sends submitted results back to the trainer with a reason.
func (s *Service) Return(ctx context.Context, p *auth.Principal, sc academics.Scope, id uuid.UUID, reason string, req audit.Request) error {
	reason = strings.TrimSpace(reason)
	var pr content.Problems
	pr.Required("reason", reason, 500)
	if err := pr.Err(); err != nil {
		return err
	}
	return s.decide(ctx, p, sc, id, "submitted", "draft", reason, "results.returned", req, nil)
}

// Released is one student's row in a release.
type Released struct {
	StudentID     uuid.UUID       `json:"student_id"`
	StudentNumber string          `json:"student_number"`
	Marks         map[string]Mark `json:"marks"`
	Result
}

// Release publishes approved results to students: the computed results are
// frozen as a new release version.
func (s *Service) Release(ctx context.Context, p *auth.Principal, sc academics.Scope, id uuid.UUID, req audit.Request) error {
	return s.decide(ctx, p, sc, id, "approved", "released", "", "results.released", req, func(q *dbq.Queries, sh dbq.LockSheetRow) error {
		view, err := sheet(ctx, q, id, sh.OfferingID)
		if err != nil {
			return err
		}
		rows := make([]Released, 0, len(view.Rows))
		for _, r := range view.Rows {
			rows = append(rows, Released{StudentID: r.StudentID, StudentNumber: r.StudentNumber, Marks: r.Marks, Result: r.Result})
		}
		raw, err := json.Marshal(rows)
		if err != nil {
			return err
		}
		v, err := q.NextReleaseVersion(ctx, id)
		if err != nil {
			return err
		}
		return q.InsertRelease(ctx, dbq.InsertReleaseParams{SheetID: id, Version: v, Results: raw, ReleasedBy: p.UserID})
	})
}
