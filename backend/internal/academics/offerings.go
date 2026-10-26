package academics

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// An offering is a unit taught in a term: trainers, a timetable and a class
// list. Managed with academics.manage within scope (by the unit's school);
// trainers see their own with academics.teach.

type OfferingItem struct {
	ID           uuid.UUID `json:"id"`
	TermID       uuid.UUID `json:"term_id"`
	TermName     string    `json:"term_name"`
	UnitID       uuid.UUID `json:"unit_id"`
	UnitCode     string    `json:"unit_code"`
	UnitTitle    string    `json:"unit_title"`
	TrainerNames string    `json:"trainer_names"`
	Registered   int64     `json:"registered"`
}

func (s *Service) ListOfferings(ctx context.Context, sc Scope, term, unit *uuid.UUID) ([]OfferingItem, error) {
	rows, err := dbq.New(s.pool).StaffListOfferings(ctx, dbq.StaffListOfferingsParams{AllSchools: sc.All, Schools: sc.Schools, TermID: term, UnitID: unit})
	if err != nil {
		return nil, err
	}
	out := make([]OfferingItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, OfferingItem{ID: r.ID, TermID: r.TermID, TermName: r.TermName, UnitID: r.UnitID, UnitCode: r.UnitCode,
			UnitTitle: r.UnitTitle, TrainerNames: r.TrainerNames, Registered: r.Registered})
	}
	return out, nil
}

type Person struct {
	ID       uuid.UUID `json:"id"`
	FullName string    `json:"full_name"`
}

type TimetableEntry struct {
	ID       uuid.UUID `json:"id"`
	Kind     string    `json:"kind"`
	Weekday  *int16    `json:"weekday"`
	OnDate   *string   `json:"on_date"`
	StartsAt string    `json:"starts_at"`
	EndsAt   string    `json:"ends_at"`
	Room     string    `json:"room"`
}

type ClassMember struct {
	RegistrationID uuid.UUID `json:"registration_id"`
	StudentID      uuid.UUID `json:"student_id"`
	StudentNumber  string    `json:"student_number"`
	Name           string    `json:"name"`
	CohortName     *string   `json:"cohort_name"`
	Status         string    `json:"status"`
	RegisteredAt   time.Time `json:"registered_at"`
}

type Offering struct {
	ID        uuid.UUID        `json:"id"`
	TermID    uuid.UUID        `json:"term_id"`
	TermName  string           `json:"term_name"`
	StartsOn  string           `json:"starts_on"`
	EndsOn    string           `json:"ends_on"`
	UnitID    uuid.UUID        `json:"unit_id"`
	UnitCode  string           `json:"unit_code"`
	UnitTitle string           `json:"unit_title"`
	Trainers  []Person         `json:"trainers"`
	Timetable []TimetableEntry `json:"timetable"`
	ClassList []ClassMember    `json:"class_list"`
}

// Clock formats a time of day as HH:MM.
func Clock(t pgtype.Time) string {
	m := t.Microseconds / 60_000_000
	return time.Date(0, 1, 1, int(m/60), int(m%60), 0, 0, time.UTC).Format("15:04")
}

// offering builds the full view of an offering read with q.
func offering(ctx context.Context, q *dbq.Queries, o dbq.GetOfferingRow) (*Offering, error) {
	out := &Offering{ID: o.ID, TermID: o.TermID, TermName: o.TermName, StartsOn: o.StartsOn.Format(time.DateOnly),
		EndsOn: o.EndsOn.Format(time.DateOnly), UnitID: o.UnitID, UnitCode: o.UnitCode, UnitTitle: o.UnitTitle,
		Trainers: []Person{}, Timetable: []TimetableEntry{}, ClassList: []ClassMember{}}
	trainers, err := q.ListOfferingTrainers(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	for _, t := range trainers {
		out.Trainers = append(out.Trainers, Person{ID: t.ID, FullName: t.FullName})
	}
	entries, err := q.ListTimetable(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		out.Timetable = append(out.Timetable, TimetableEntry{ID: e.ID, Kind: e.Kind, Weekday: e.Weekday, OnDate: dateString(e.OnDate),
			StartsAt: Clock(e.StartsAt), EndsAt: Clock(e.EndsAt), Room: e.Room})
	}
	class, err := q.ListClassList(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	for _, c := range class {
		out.ClassList = append(out.ClassList, ClassMember{RegistrationID: c.ID, StudentID: c.StudentID, StudentNumber: c.StudentNumber,
			Name: fullName(c.FirstName, c.MiddleName, c.LastName), CohortName: c.CohortName, Status: c.Status, RegisteredAt: c.RegisteredAt})
	}
	return out, nil
}

// loadOffering reads an offering in scope (by its unit's school).
func loadOffering(ctx context.Context, q *dbq.Queries, sc Scope, id uuid.UUID) (dbq.GetOfferingRow, error) {
	o, err := q.GetOffering(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !sc.Covers(o.SchoolID)) {
		return o, ErrNotFound
	}
	return o, err
}

func (s *Service) GetOffering(ctx context.Context, sc Scope, id uuid.UUID) (*Offering, error) {
	q := dbq.New(s.pool)
	o, err := loadOffering(ctx, q, sc, id)
	if err != nil {
		return nil, err
	}
	return offering(ctx, q, o)
}

type OfferingInput struct {
	TermID uuid.UUID `json:"term_id"`
	UnitID uuid.UUID `json:"unit_id"`
}

// CreateOffering schedules a unit in a term.
func (s *Service) CreateOffering(ctx context.Context, sc Scope, in OfferingInput, req audit.Request) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.tx(ctx, func(q *dbq.Queries) error {
		u, err := q.GetUnit(ctx, in.UnitID)
		if errors.Is(err, pgx.ErrNoRows) {
			return content.Problems{{Field: "unit_id", Message: "Choose a unit from the list."}}.Err()
		}
		if err != nil {
			return err
		}
		if !sc.Covers(u.SchoolID) {
			return ErrOutsideScope
		}
		if !u.Active {
			return content.Problems{{Field: "unit_id", Message: "This unit is no longer taught. Make it active first."}}.Err()
		}
		if _, err := q.GetTerm(ctx, in.TermID); errors.Is(err, pgx.ErrNoRows) {
			return content.Problems{{Field: "term_id", Message: "Choose a term from the list."}}.Err()
		} else if err != nil {
			return err
		}
		if id, err = q.InsertOffering(ctx, dbq.InsertOfferingParams{TermID: in.TermID, UnitID: in.UnitID}); err != nil {
			if isUniqueViolation(err) {
				return ErrDuplicate
			}
			return err
		}
		return audit.Write(ctx, q, req, entry("offering.created", "unit_offering", id, map[string]any{"unit": u.Code}))
	})
	return id, err
}

// Trainers lists active staff who can teach.
func (s *Service) Trainers(ctx context.Context) ([]Person, error) {
	rows, err := dbq.New(s.pool).ListTrainers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Person, 0, len(rows))
	for _, r := range rows {
		out = append(out, Person{ID: r.ID, FullName: r.FullName})
	}
	return out, nil
}

// SetTrainers replaces who teaches an offering. Each must hold academics.teach.
func (s *Service) SetTrainers(ctx context.Context, p *auth.Principal, sc Scope, id uuid.UUID, users []uuid.UUID, req audit.Request) error {
	if len(users) > 10 {
		return content.Problems{{Field: "trainer_ids", Message: "Assign at most 10 trainers."}}.Err()
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		if _, err := loadOffering(ctx, q, sc, id); err != nil {
			return err
		}
		allowed, err := q.ListTrainers(ctx)
		if err != nil {
			return err
		}
		for _, u := range users {
			if !slices.ContainsFunc(allowed, func(t dbq.ListTrainersRow) bool { return t.ID == u }) {
				return content.Problems{{Field: "trainer_ids", Message: "Choose trainers from the list."}}.Err()
			}
		}
		if err := q.DeleteOfferingTrainers(ctx, id); err != nil {
			return err
		}
		seen := map[uuid.UUID]bool{}
		names := []string{}
		for _, u := range users {
			if seen[u] {
				continue
			}
			seen[u] = true
			names = append(names, u.String())
			if err := q.InsertTrainerAssignment(ctx, dbq.InsertTrainerAssignmentParams{OfferingID: id, UserID: u, AssignedBy: &p.UserID}); err != nil {
				return err
			}
		}
		return audit.Write(ctx, q, req, entry("offering.trainers_set", "unit_offering", id, map[string]any{"trainers": names}))
	})
}

type TimetableInput struct {
	Kind     string `json:"kind"`
	Weekday  *int16 `json:"weekday"`
	OnDate   string `json:"on_date"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
	Room     string `json:"room"`
}

func parseClock(pr *content.Problems, field, v string) (pgtype.Time, bool) {
	t, err := time.Parse("15:04", strings.TrimSpace(v))
	if err != nil {
		pr.Add(field, "Enter the time as HH:MM, like 08:30.")
		return pgtype.Time{}, false
	}
	return pgtype.Time{Microseconds: int64(t.Hour()*60+t.Minute()) * 60_000_000, Valid: true}, true
}

// AddTimetableEntry adds a weekly class or an exam to an offering.
func (s *Service) AddTimetableEntry(ctx context.Context, sc Scope, id uuid.UUID, in TimetableInput, req audit.Request) error {
	var pr content.Problems
	in.Room = strings.TrimSpace(in.Room)
	pr.OneOf("kind", in.Kind, "class", "exam")
	pr.Optional("room", in.Room, 60)
	starts, okS := parseClock(&pr, "starts_at", in.StartsAt)
	ends, okE := parseClock(&pr, "ends_at", in.EndsAt)
	if okS && okE && ends.Microseconds <= starts.Microseconds {
		pr.Add("ends_at", "It must end after it starts.")
	}
	var weekday *int16
	var on *time.Time
	switch in.Kind {
	case "class":
		if in.Weekday == nil || *in.Weekday < 1 || *in.Weekday > 7 {
			pr.Add("weekday", "Choose the day of the week.")
		}
		weekday = in.Weekday
	case "exam":
		on = parseDate(&pr, "on_date", in.OnDate, true)
	}
	if err := pr.Err(); err != nil {
		return err
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		o, err := loadOffering(ctx, q, sc, id)
		if err != nil {
			return err
		}
		if on != nil && (on.Before(o.StartsOn) || on.After(o.EndsOn)) {
			return content.Problems{{Field: "on_date", Message: "The exam must be within the term."}}.Err()
		}
		eid, err := q.InsertTimetableEntry(ctx, dbq.InsertTimetableEntryParams{OfferingID: id, Kind: in.Kind, Weekday: weekday,
			OnDate: on, StartsAt: starts, EndsAt: ends, Room: in.Room})
		if err != nil {
			return err
		}
		return audit.Write(ctx, q, req, entry("timetable.entry_added", "unit_offering", id, map[string]any{"entry_id": eid.String(), "kind": in.Kind}))
	})
}

func (s *Service) RemoveTimetableEntry(ctx context.Context, sc Scope, id, entryID uuid.UUID, req audit.Request) error {
	return s.tx(ctx, func(q *dbq.Queries) error {
		if _, err := loadOffering(ctx, q, sc, id); err != nil {
			return err
		}
		n, err := q.DeleteTimetableEntry(ctx, dbq.DeleteTimetableEntryParams{ID: entryID, OfferingID: id})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return audit.Write(ctx, q, req, entry("timetable.entry_removed", "unit_offering", id, map[string]any{"entry_id": entryID.String()}))
	})
}

type RegisterInput struct {
	StudentIDs []uuid.UUID `json:"student_ids"`
	CohortID   *uuid.UUID  `json:"cohort_id"`
}

// RegisterResult says what happened to each student asked for.
type RegisterResult struct {
	Registered        int `json:"registered"`
	AlreadyRegistered int `json:"already_registered"`
	// NotEligible: not active, or the unit is not in their programme.
	NotEligible int `json:"not_eligible"`
}

// Register puts students (or a whole cohort's active students) on an
// offering's class list. Only active students whose programme includes the
// unit are registered; the rest are counted, not refused one by one.
func (s *Service) Register(ctx context.Context, p *auth.Principal, sc Scope, id uuid.UUID, in RegisterInput, req audit.Request) (*RegisterResult, error) {
	if (in.CohortID == nil) == (len(in.StudentIDs) == 0) {
		return nil, content.Problems{{Field: "student_ids", Message: "Choose students or one cohort."}}.Err()
	}
	if len(in.StudentIDs) > 500 {
		return nil, content.Problems{{Field: "student_ids", Message: "Register at most 500 students at a time."}}.Err()
	}
	var res RegisterResult
	err := s.tx(ctx, func(q *dbq.Queries) error {
		o, err := loadOffering(ctx, q, sc, id)
		if err != nil {
			return err
		}
		if o.EndsOn.Before(s.today()) {
			return content.Problems{{Field: "term", Message: "This term has ended. Registration is closed."}}.Err()
		}
		ids := in.StudentIDs
		if in.CohortID != nil {
			c, err := q.StaffGetCohort(ctx, dbq.StaffGetCohortParams{ID: *in.CohortID, AllSchools: sc.All, Schools: sc.Schools})
			if errors.Is(err, pgx.ErrNoRows) {
				return content.Problems{{Field: "cohort_id", Message: "Choose a cohort from the list."}}.Err()
			}
			if err != nil {
				return err
			}
			if ids, err = q.ListCohortStudentIDs(ctx, &c.ID); err != nil {
				return err
			}
		}
		eligible, err := q.EligibleStudents(ctx, dbq.EligibleStudentsParams{UnitID: o.UnitID, Ids: ids})
		if err != nil {
			return err
		}
		// Students outside the scope are not eligible either.
		var inScope []uuid.UUID
		for _, sid := range eligible {
			st, err := q.LockStudent(ctx, sid)
			if err != nil {
				return err
			}
			if sc.Covers(st.SchoolID) {
				inScope = append(inScope, sid)
			}
		}
		res.NotEligible = len(dedupe(ids)) - len(inScope)
		for _, sid := range inScope {
			n, err := q.RegisterStudent(ctx, dbq.RegisterStudentParams{OfferingID: id, StudentID: sid, RegisteredBy: &p.UserID})
			if err != nil {
				return err
			}
			if n == 1 {
				res.Registered++
			} else {
				res.AlreadyRegistered++
			}
		}
		return audit.Write(ctx, q, req, entry("offering.students_registered", "unit_offering", id, map[string]any{
			"registered": res.Registered, "already": res.AlreadyRegistered, "not_eligible": res.NotEligible}))
	})
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func dedupe(ids []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	out := ids[:0:0]
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// Drop takes a student off a class list; the registration row is kept.
func (s *Service) Drop(ctx context.Context, p *auth.Principal, sc Scope, id, registrationID uuid.UUID, req audit.Request) error {
	return s.tx(ctx, func(q *dbq.Queries) error {
		if _, err := loadOffering(ctx, q, sc, id); err != nil {
			return err
		}
		n, err := q.DropRegistration(ctx, dbq.DropRegistrationParams{ID: registrationID, OfferingID: id, DroppedBy: &p.UserID})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return audit.Write(ctx, q, req, entry("offering.registration_dropped", "unit_offering", id, map[string]any{"registration_id": registrationID.String()}))
	})
}

// ---------- teaching (academics.teach): only the trainer's own ----------

type MyOffering struct {
	ID         uuid.UUID `json:"id"`
	TermName   string    `json:"term_name"`
	StartsOn   string    `json:"starts_on"`
	EndsOn     string    `json:"ends_on"`
	UnitCode   string    `json:"unit_code"`
	UnitTitle  string    `json:"unit_title"`
	Registered int64     `json:"registered"`
}

type MyTimetableEntry struct {
	TimetableEntry
	OfferingID uuid.UUID `json:"offering_id"`
	UnitCode   string    `json:"unit_code"`
	UnitTitle  string    `json:"unit_title"`
	TermName   string    `json:"term_name"`
}

type Teaching struct {
	Offerings []MyOffering       `json:"offerings"`
	Timetable []MyTimetableEntry `json:"timetable"`
}

// MyTeaching lists the offerings a trainer is assigned to, and the timetable
// of those not yet over.
func (s *Service) MyTeaching(ctx context.Context, user uuid.UUID) (*Teaching, error) {
	q := dbq.New(s.pool)
	rows, err := q.ListMyOfferings(ctx, user)
	if err != nil {
		return nil, err
	}
	out := &Teaching{Offerings: make([]MyOffering, 0, len(rows)), Timetable: []MyTimetableEntry{}}
	for _, r := range rows {
		out.Offerings = append(out.Offerings, MyOffering{ID: r.ID, TermName: r.TermName, StartsOn: r.StartsOn.Format(time.DateOnly),
			EndsOn: r.EndsOn.Format(time.DateOnly), UnitCode: r.UnitCode, UnitTitle: r.UnitTitle, Registered: r.Registered})
	}
	tt, err := q.ListMyTimetable(ctx, dbq.ListMyTimetableParams{UserID: user, Today: s.today()})
	if err != nil {
		return nil, err
	}
	for _, e := range tt {
		out.Timetable = append(out.Timetable, MyTimetableEntry{
			TimetableEntry: TimetableEntry{ID: e.ID, Kind: e.Kind, Weekday: e.Weekday, OnDate: dateString(e.OnDate),
				StartsAt: Clock(e.StartsAt), EndsAt: Clock(e.EndsAt), Room: e.Room},
			OfferingID: e.OfferingID, UnitCode: e.UnitCode, UnitTitle: e.UnitTitle, TermName: e.TermName})
	}
	return out, nil
}

// MyOffering is one of the trainer's own offerings with its class list. Any
// other offering answers ErrNotFound (AC-08).
func (s *Service) MyOffering(ctx context.Context, user, id uuid.UUID) (*Offering, error) {
	q := dbq.New(s.pool)
	ok, err := q.IsAssignedTrainer(ctx, dbq.IsAssignedTrainerParams{OfferingID: id, UserID: user})
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	o, err := q.GetOffering(ctx, id)
	if err != nil {
		return nil, err
	}
	return offering(ctx, q, o)
}

// ---------- school assignments (users.manage) ----------

type School struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (s *Service) UserSchools(ctx context.Context, user uuid.UUID) ([]School, error) {
	rows, err := dbq.New(s.pool).ListUserSchools(ctx, user)
	if err != nil {
		return nil, err
	}
	out := make([]School, 0, len(rows))
	for _, r := range rows {
		out = append(out, School{ID: r.ID, Name: r.Name})
	}
	return out, nil
}

// SetUserSchools replaces the schools a member of staff works in.
func (s *Service) SetUserSchools(ctx context.Context, user uuid.UUID, schools []uuid.UUID, req audit.Request) error {
	schools = dedupe(schools)
	return s.tx(ctx, func(q *dbq.Queries) error {
		u, err := q.GetUserByID(ctx, user)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && u.Kind != "staff") {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if n, err := q.CountSchools(ctx, schools); err != nil {
			return err
		} else if int(n) != len(schools) {
			return content.Problems{{Field: "school_ids", Message: "Choose schools from the list."}}.Err()
		}
		if err := q.DeleteUserSchools(ctx, user); err != nil {
			return err
		}
		ids := []string{}
		for _, sc := range schools {
			ids = append(ids, sc.String())
			if err := q.InsertUserSchool(ctx, dbq.InsertUserSchoolParams{UserID: user, SchoolID: sc}); err != nil {
				return err
			}
		}
		return audit.Write(ctx, q, req, entry("user.schools_set", "user", user, map[string]any{"schools": ids}))
	})
}
