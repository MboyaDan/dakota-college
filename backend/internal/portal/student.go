package portal

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/academics"
	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/finance"
)

// The student's own pages. user is always the signed-in account.

const maxOpenRequests = 5

type Profile struct {
	StudentNumber        string               `json:"student_number"`
	FirstName            string               `json:"first_name"`
	MiddleName           string               `json:"middle_name"`
	LastName             string               `json:"last_name"`
	DateOfBirth          *string              `json:"date_of_birth"`
	Email                string               `json:"email"`
	Phone                *string              `json:"phone"`
	IdentityDocumentType *string              `json:"identity_document_type"`
	IdentityDocument     string               `json:"identity_document"`
	Status               string               `json:"status"`
	EnrolledAt           time.Time            `json:"enrolled_at"`
	ProgrammeTitle       string               `json:"programme_title"`
	SchoolName           string               `json:"school_name"`
	IntakeName           *string              `json:"intake_name"`
	CohortName           *string              `json:"cohort_name"`
	Guardians            []academics.Guardian `json:"guardians"`
	ChangeRequests       []ChangeRequest      `json:"change_requests"`
}

type ChangeRequest struct {
	ID        uuid.UUID         `json:"id"`
	OldValues map[string]string `json:"old_values"`
	NewValues map[string]string `json:"new_values"`
	Reason    string            `json:"reason"`
	Status    string            `json:"status"`
	Decision  string            `json:"decision"`
	DecidedAt *time.Time        `json:"decided_at"`
	CreatedAt time.Time         `json:"created_at"`
}

func changeRequest(id uuid.UUID, oldV, newV []byte, reason, status, decision string, decided *time.Time, created time.Time) (ChangeRequest, error) {
	c := ChangeRequest{ID: id, Reason: reason, Status: status, Decision: decision, DecidedAt: decided, CreatedAt: created}
	if err := json.Unmarshal(oldV, &c.OldValues); err != nil {
		return c, err
	}
	return c, json.Unmarshal(newV, &c.NewValues)
}

// Profile is the student's own record, with their contacts and change requests.
func (s *Service) Profile(ctx context.Context, user uuid.UUID) (*Profile, error) {
	q := dbq.New(s.pool)
	st, err := s.own(ctx, q, user)
	if err != nil {
		return nil, err
	}
	p := &Profile{StudentNumber: st.StudentNumber, FirstName: st.FirstName, MiddleName: st.MiddleName, LastName: st.LastName,
		DateOfBirth: dateString(st.DateOfBirth), Email: st.Email, Phone: st.Phone, IdentityDocumentType: st.IdentityDocumentType,
		IdentityDocument: st.IdentityDocument, Status: st.Status, EnrolledAt: st.EnrolledAt, ProgrammeTitle: st.ProgrammeTitle,
		SchoolName: st.SchoolName, IntakeName: st.IntakeName, CohortName: st.CohortName,
		Guardians: []academics.Guardian{}, ChangeRequests: []ChangeRequest{}}
	gs, err := q.ListStudentGuardians(ctx, st.ID)
	if err != nil {
		return nil, err
	}
	for _, g := range gs {
		email := ""
		if g.Email != nil {
			email = *g.Email
		}
		p.Guardians = append(p.Guardians, academics.Guardian{ID: g.ID, FullName: g.FullName, Relationship: g.Relationship,
			Phone: g.Phone, Email: email, IsPrimary: g.IsPrimary})
	}
	reqs, err := q.ListOwnProfileChangeRequests(ctx, st.ID)
	if err != nil {
		return nil, err
	}
	for _, r := range reqs {
		c, err := changeRequest(r.ID, r.OldValues, r.NewValues, r.Reason, r.Status, r.Decision, r.DecidedAt, r.CreatedAt)
		if err != nil {
			return nil, err
		}
		p.ChangeRequests = append(p.ChangeRequests, c)
	}
	return p, nil
}

// SetPhone changes the student's own phone number at once (D-142).
func (s *Service) SetPhone(ctx context.Context, user uuid.UUID, phone string, req audit.Request) error {
	phone = strings.TrimSpace(phone)
	var pr content.Problems
	pr.Required("phone", phone, 30)
	pr.Phone("phone", phone)
	if err := pr.Err(); err != nil {
		return err
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		st, err := s.own(ctx, q, user)
		if err != nil {
			return err
		}
		if st.Phone != nil && *st.Phone == phone {
			return ErrUnchanged
		}
		if err := q.SetStudentPhone(ctx, dbq.SetStudentPhoneParams{ID: st.ID, Phone: &phone}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "student.phone_changed", EntityType: "student", EntityID: st.ID.String(),
			Details: map[string]any{"by": "student"}})
	})
}

// Guardians: the student keeps their own next of kin up to date, with the
// same rules and audit as staff (the academics service, scoped to their record).

func (s *Service) studentID(ctx context.Context, user uuid.UUID) (dbq.GetStudentByUserRow, error) {
	return s.own(ctx, dbq.New(s.pool), user)
}

func (s *Service) AddGuardian(ctx context.Context, user uuid.UUID, in academics.GuardianInput, req audit.Request) error {
	st, err := s.studentID(ctx, user)
	if err != nil {
		return err
	}
	return s.academics.AddGuardian(ctx, ownScope(st), st.ID, in, req)
}

func (s *Service) UpdateGuardian(ctx context.Context, user, gid uuid.UUID, in academics.GuardianInput, req audit.Request) error {
	st, err := s.studentID(ctx, user)
	if err != nil {
		return err
	}
	return s.academics.UpdateGuardian(ctx, ownScope(st), st.ID, gid, in, req)
}

func (s *Service) RemoveGuardian(ctx context.Context, user, gid uuid.UUID, req audit.Request) error {
	st, err := s.studentID(ctx, user)
	if err != nil {
		return err
	}
	return s.academics.RemoveGuardian(ctx, ownScope(st), st.ID, gid, req)
}

// ChangeInput is what a student asks to change; empty fields stay as they are.
type ChangeInput struct {
	FirstName        string `json:"first_name"`
	MiddleName       string `json:"middle_name"`
	LastName         string `json:"last_name"`
	DateOfBirth      string `json:"date_of_birth"`
	IdentityDocument string `json:"identity_document"`
	Reason           string `json:"reason"`
}

// RequestChange asks the registrar to change identity details (D-142).
func (s *Service) RequestChange(ctx context.Context, user uuid.UUID, in ChangeInput, req audit.Request) error {
	var pr content.Problems
	in.Reason = strings.TrimSpace(in.Reason)
	pr.Required("reason", in.Reason, 2000)
	return s.tx(ctx, func(q *dbq.Queries) error {
		st, err := s.own(ctx, q, user)
		if err != nil {
			return err
		}
		oldV, newV := map[string]string{}, map[string]string{}
		consider := func(field, current, wanted string, max int) {
			wanted = strings.TrimSpace(wanted)
			if wanted == "" || wanted == current {
				return
			}
			if len([]rune(wanted)) > max {
				pr.Add(field, "This is too long.")
				return
			}
			oldV[field], newV[field] = current, wanted
		}
		consider("first_name", st.FirstName, in.FirstName, 80)
		consider("middle_name", st.MiddleName, in.MiddleName, 80)
		consider("last_name", st.LastName, in.LastName, 80)
		consider("identity_document", st.IdentityDocument, in.IdentityDocument, 30)
		if v := strings.TrimSpace(in.DateOfBirth); v != "" {
			d, err := time.Parse(time.DateOnly, v)
			switch {
			case err != nil:
				pr.Add("date_of_birth", "Enter the date as YYYY-MM-DD.")
			case d.After(s.today()):
				pr.Add("date_of_birth", "The date of birth cannot be in the future.")
			default:
				consider("date_of_birth", deref(dateString(st.DateOfBirth)), v, 10)
			}
		}
		if len(newV) == 0 {
			pr.Add("first_name", "Change at least one detail.")
		}
		if err := pr.Err(); err != nil {
			return err
		}
		ob, _ := json.Marshal(oldV)
		nb, _ := json.Marshal(newV)
		id, err := q.InsertProfileChangeRequest(ctx, dbq.InsertProfileChangeRequestParams{StudentID: st.ID, OldValues: ob, NewValues: nb, Reason: in.Reason})
		if isUniqueViolation(err) {
			return ErrPending
		}
		if err != nil {
			return err
		}
		fields := make([]string, 0, len(newV))
		for k := range newV {
			fields = append(fields, k)
		}
		slices.Sort(fields)
		return audit.Write(ctx, q, req, audit.Entry{Action: "student.change_requested", EntityType: "student", EntityID: st.ID.String(),
			Details: map[string]any{"request_id": id.String(), "fields": fields}})
	})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ---------- registration ----------

type RegistrationOption struct {
	OfferingID           uuid.UUID `json:"offering_id"`
	TermName             string    `json:"term_name"`
	RegistrationClosesOn *string   `json:"registration_closes_on"`
	UnitCode             string    `json:"unit_code"`
	UnitTitle            string    `json:"unit_title"`
	Stage                int16     `json:"stage"`
	IsCore               bool      `json:"is_core"`
	Registered           bool      `json:"registered"`
}

type Registration struct {
	// CanRegister is false unless the student is active.
	CanRegister bool                     `json:"can_register"`
	Open        []RegistrationOption     `json:"open"`
	Current     []academics.Registration `json:"current"`
}

// Registration lists the classes open to the student today and what they
// are registered for.
func (s *Service) Registration(ctx context.Context, user uuid.UUID) (*Registration, error) {
	q := dbq.New(s.pool)
	st, err := s.own(ctx, q, user)
	if err != nil {
		return nil, err
	}
	out := &Registration{CanRegister: st.Status == "active", Open: []RegistrationOption{}, Current: []academics.Registration{}}
	opts, err := q.StudentRegistrationOptions(ctx, dbq.StudentRegistrationOptionsParams{ProgrammeID: st.ProgrammeID, StudentID: st.ID, Today: s.today()})
	if err != nil {
		return nil, err
	}
	for _, o := range opts {
		out.Open = append(out.Open, RegistrationOption{OfferingID: o.ID, TermName: o.TermName, RegistrationClosesOn: dateString(o.RegistrationClosesOn),
			UnitCode: o.UnitCode, UnitTitle: o.UnitTitle, Stage: o.Stage, IsCore: o.IsCore,
			Registered: o.RegistrationStatus != nil && *o.RegistrationStatus == "registered"})
	}
	regs, err := q.ListStudentRegistrations(ctx, st.ID)
	if err != nil {
		return nil, err
	}
	for _, x := range regs {
		out.Current = append(out.Current, academics.Registration{ID: x.ID, OfferingID: x.OfferingID, TermName: x.TermName,
			UnitCode: x.UnitCode, UnitTitle: x.UnitTitle, Status: x.Status, At: x.RegisteredAt, DroppedAt: x.DroppedAt})
	}
	return out, nil
}

// changeRegistration checks the rules (active, window open, unit in the
// programme) and runs fn in one transaction.
func (s *Service) changeRegistration(ctx context.Context, user, offering uuid.UUID, fn func(q *dbq.Queries, st dbq.GetStudentByUserRow) error) error {
	return s.tx(ctx, func(q *dbq.Queries) error {
		st, err := s.own(ctx, q, user)
		if err != nil {
			return err
		}
		if st.Status != "active" {
			return ErrNotActive
		}
		if _, err := q.StudentOfferingOpen(ctx, dbq.StudentOfferingOpenParams{ProgrammeID: st.ProgrammeID, OfferingID: offering, Today: s.today()}); errors.Is(err, pgx.ErrNoRows) {
			return ErrClosed
		} else if err != nil {
			return err
		}
		return fn(q, st)
	})
}

// Register puts the student on a class list, within the registration window.
func (s *Service) Register(ctx context.Context, user, offering uuid.UUID, req audit.Request) error {
	return s.changeRegistration(ctx, user, offering, func(q *dbq.Queries, st dbq.GetStudentByUserRow) error {
		n, err := q.RegisterStudent(ctx, dbq.RegisterStudentParams{OfferingID: offering, StudentID: st.ID, RegisteredBy: &user})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrUnchanged
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "student.unit_registered", EntityType: "unit_offering", EntityID: offering.String(),
			Details: map[string]any{"student_id": st.ID.String(), "by": "student"}})
	})
}

// Drop takes the student off a class list while the window is open.
func (s *Service) Drop(ctx context.Context, user, offering uuid.UUID, req audit.Request) error {
	return s.changeRegistration(ctx, user, offering, func(q *dbq.Queries, st dbq.GetStudentByUserRow) error {
		r, err := q.GetOwnRegistration(ctx, dbq.GetOwnRegistrationParams{OfferingID: offering, StudentID: st.ID})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && r.Status != "registered") {
			return ErrUnchanged
		}
		if err != nil {
			return err
		}
		if _, err := q.DropRegistration(ctx, dbq.DropRegistrationParams{ID: r.ID, OfferingID: offering, DroppedBy: &user}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "student.unit_dropped", EntityType: "unit_offering", EntityID: offering.String(),
			Details: map[string]any{"student_id": st.ID.String(), "by": "student"}})
	})
}

// ---------- timetable and notices ----------

type TimetableEntry struct {
	academics.TimetableEntry
	OfferingID uuid.UUID `json:"offering_id"`
	UnitCode   string    `json:"unit_code"`
	UnitTitle  string    `json:"unit_title"`
	TermName   string    `json:"term_name"`
}

func (s *Service) Timetable(ctx context.Context, user uuid.UUID) ([]TimetableEntry, error) {
	q := dbq.New(s.pool)
	st, err := s.own(ctx, q, user)
	if err != nil {
		return nil, err
	}
	rows, err := q.StudentTimetable(ctx, dbq.StudentTimetableParams{StudentID: st.ID, Today: s.today()})
	if err != nil {
		return nil, err
	}
	out := make([]TimetableEntry, 0, len(rows))
	for _, e := range rows {
		out = append(out, TimetableEntry{
			TimetableEntry: academics.TimetableEntry{ID: e.ID, Kind: e.Kind, Weekday: e.Weekday, OnDate: dateString(e.OnDate),
				StartsAt: academics.Clock(e.StartsAt), EndsAt: academics.Clock(e.EndsAt), Room: e.Room},
			OfferingID: e.OfferingID, UnitCode: e.UnitCode, UnitTitle: e.UnitTitle, TermName: e.TermName})
	}
	return out, nil
}

type Notice struct {
	ID           uuid.UUID `json:"id"`
	Audience     string    `json:"audience"`
	AudienceName string    `json:"audience_name"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`
	EndsOn       *string   `json:"ends_on"`
	AuthorName   string    `json:"author_name"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *Service) Notices(ctx context.Context, user uuid.UUID, limit int32) ([]Notice, error) {
	q := dbq.New(s.pool)
	st, err := s.own(ctx, q, user)
	if err != nil {
		return nil, err
	}
	return s.notices(ctx, q, st, limit)
}

func (s *Service) notices(ctx context.Context, q *dbq.Queries, st dbq.GetStudentByUserRow, limit int32) ([]Notice, error) {
	rows, err := q.StudentNotices(ctx, dbq.StudentNoticesParams{Today: s.today(), SchoolID: &st.SchoolID, CohortID: st.CohortID, StudentID: st.ID, MaxItems: limit})
	if err != nil {
		return nil, err
	}
	out := make([]Notice, 0, len(rows))
	for _, n := range rows {
		out = append(out, Notice{ID: n.ID, Audience: n.Audience, AudienceName: n.AudienceName, Title: n.Title, Body: n.Body,
			EndsOn: dateString(n.EndsOn), AuthorName: n.AuthorName, CreatedAt: n.CreatedAt})
	}
	return out, nil
}

// ---------- service requests ----------

var RequestKinds = []string{"letter", "transcript", "deferment", "change_of_course", "other"}

type ServiceRequest struct {
	ID        uuid.UUID `json:"id"`
	Kind      string    `json:"kind"`
	Details   string    `json:"details"`
	Status    string    `json:"status"`
	Response  string    `json:"response"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Service) Requests(ctx context.Context, user uuid.UUID) ([]ServiceRequest, error) {
	q := dbq.New(s.pool)
	st, err := s.own(ctx, q, user)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListOwnServiceRequests(ctx, st.ID)
	if err != nil {
		return nil, err
	}
	out := make([]ServiceRequest, 0, len(rows))
	for _, r := range rows {
		out = append(out, ServiceRequest{ID: r.ID, Kind: r.Kind, Details: r.Details, Status: r.Status, Response: r.Response,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt})
	}
	return out, nil
}

// CreateRequest records a service request. At most five may be open at once.
func (s *Service) CreateRequest(ctx context.Context, user uuid.UUID, kind, details string, req audit.Request) error {
	details = strings.TrimSpace(details)
	var pr content.Problems
	pr.OneOf("kind", kind, RequestKinds...)
	pr.Required("details", details, 2000)
	if err := pr.Err(); err != nil {
		return err
	}
	return s.tx(ctx, func(q *dbq.Queries) error {
		st, err := s.own(ctx, q, user)
		if err != nil {
			return err
		}
		if n, err := q.CountOpenServiceRequests(ctx, st.ID); err != nil {
			return err
		} else if n >= maxOpenRequests {
			return ErrTooMany
		}
		id, err := q.InsertServiceRequest(ctx, dbq.InsertServiceRequestParams{StudentID: st.ID, Kind: kind, Details: details})
		if err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "student.request_created", EntityType: "service_request", EntityID: id.String(),
			Details: map[string]any{"kind": kind}})
	})
}

// ---------- dashboard ----------

type Dashboard struct {
	Name           string           `json:"name"`
	StudentNumber  string           `json:"student_number"`
	Status         string           `json:"status"`
	ProgrammeTitle string           `json:"programme_title"`
	CohortName     *string          `json:"cohort_name"`
	Timetable      []TimetableEntry `json:"timetable"`
	Notices        []Notice         `json:"notices"`
	// Pending things for the student to see to.
	OpenRequests         int64 `json:"open_requests"`
	ChangeRequestPending bool  `json:"change_request_pending"`
	// RegistrationOpen counts classes open to register for and not yet registered.
	RegistrationOpen int `json:"registration_open"`
	// FeeBalanceCents is null while nothing has been billed or paid;
	// positive owes, negative is credit (Stage 14).
	FeeBalanceCents *int64 `json:"fee_balance_cents"`
	FeeOverdueCents int64  `json:"fee_overdue_cents"`
}

func (s *Service) Dashboard(ctx context.Context, user uuid.UUID) (*Dashboard, error) {
	q := dbq.New(s.pool)
	st, err := s.own(ctx, q, user)
	if err != nil {
		return nil, err
	}
	d := &Dashboard{Name: fullName(st.FirstName, st.MiddleName, st.LastName), StudentNumber: st.StudentNumber, Status: st.Status,
		ProgrammeTitle: st.ProgrammeTitle, CohortName: st.CohortName}
	if d.Timetable, err = s.Timetable(ctx, user); err != nil {
		return nil, err
	}
	if d.Notices, err = s.notices(ctx, q, st, 3); err != nil {
		return nil, err
	}
	if d.OpenRequests, err = q.CountOpenServiceRequests(ctx, st.ID); err != nil {
		return nil, err
	}
	reqs, err := q.ListOwnProfileChangeRequests(ctx, st.ID)
	if err != nil {
		return nil, err
	}
	d.ChangeRequestPending = slices.ContainsFunc(reqs, func(r dbq.ListOwnProfileChangeRequestsRow) bool { return r.Status == "pending" })
	if st.Status == "active" {
		opts, err := q.StudentRegistrationOptions(ctx, dbq.StudentRegistrationOptionsParams{ProgrammeID: st.ProgrammeID, StudentID: st.ID, Today: s.today()})
		if err != nil {
			return nil, err
		}
		for _, o := range opts {
			if o.RegistrationStatus == nil || *o.RegistrationStatus != "registered" {
				d.RegistrationOpen++
			}
		}
	}
	balance, overdue, billed, err := finance.BalanceOf(ctx, q, st.ID, s.now())
	if err != nil {
		return nil, err
	}
	if billed {
		d.FeeBalanceCents, d.FeeOverdueCents = &balance, overdue
	}
	return d, nil
}
