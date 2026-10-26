package academics

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// Student statuses. Graduated is final; every change needs a reason and is
// kept in student_status_history.
var StudentStatuses = []string{"active", "deferred", "suspended", "withdrawn", "graduated"}

var Relationships = []string{"mother", "father", "guardian", "other_relative", "spouse", "other"}

type StudentFilter struct {
	Status      string
	ProgrammeID *uuid.UUID
	CohortID    *uuid.UUID
	Search      string
	Page        int
}

type StudentItem struct {
	ID             uuid.UUID `json:"id"`
	StudentNumber  string    `json:"student_number"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	Status         string    `json:"status"`
	ProgrammeTitle string    `json:"programme_title"`
	CohortName     *string   `json:"cohort_name"`
	EnrolledAt     time.Time `json:"enrolled_at"`
}

type StudentPage struct {
	Items      []StudentItem `json:"items"`
	Page       int           `json:"page"`
	TotalPages int           `json:"total_pages"`
	Total      int64         `json:"total"`
}

func (s *Service) ListStudents(ctx context.Context, sc Scope, f StudentFilter) (*StudentPage, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	q := dbq.New(s.pool)
	status, search := optionalText(f.Status), optionalText(f.Search)
	total, err := q.StaffCountStudents(ctx, dbq.StaffCountStudentsParams{AllSchools: sc.All, Schools: sc.Schools,
		Status: status, ProgrammeID: f.ProgrammeID, CohortID: f.CohortID, Search: search})
	if err != nil {
		return nil, err
	}
	rows, err := q.StaffListStudents(ctx, dbq.StaffListStudentsParams{AllSchools: sc.All, Schools: sc.Schools,
		Status: status, ProgrammeID: f.ProgrammeID, CohortID: f.CohortID, Search: search,
		MaxItems: pageSize, Skip: int32((f.Page - 1) * pageSize)})
	if err != nil {
		return nil, err
	}
	out := &StudentPage{Items: make([]StudentItem, 0, len(rows)), Page: f.Page, Total: total, TotalPages: pages(total)}
	for _, r := range rows {
		out.Items = append(out.Items, StudentItem{ID: r.ID, StudentNumber: r.StudentNumber, Name: fullName(r.FirstName, r.MiddleName, r.LastName),
			Email: r.Email, Status: r.Status, ProgrammeTitle: r.ProgrammeTitle, CohortName: r.CohortName, EnrolledAt: r.EnrolledAt})
	}
	return out, nil
}

type Guardian struct {
	ID           uuid.UUID `json:"id"`
	FullName     string    `json:"full_name"`
	Relationship string    `json:"relationship"`
	Phone        string    `json:"phone"`
	Email        string    `json:"email"`
	IsPrimary    bool      `json:"is_primary"`
}

type StatusChange struct {
	From          *string   `json:"from"`
	To            string    `json:"to"`
	Reason        string    `json:"reason"`
	At            time.Time `json:"at"`
	ChangedByName *string   `json:"changed_by_name"`
}

type Registration struct {
	ID         uuid.UUID  `json:"id"`
	OfferingID uuid.UUID  `json:"offering_id"`
	TermName   string     `json:"term_name"`
	UnitCode   string     `json:"unit_code"`
	UnitTitle  string     `json:"unit_title"`
	Status     string     `json:"status"`
	At         time.Time  `json:"registered_at"`
	DroppedAt  *time.Time `json:"dropped_at"`
}

type Student struct {
	ID                   uuid.UUID      `json:"id"`
	StudentNumber        string         `json:"student_number"`
	FirstName            string         `json:"first_name"`
	MiddleName           string         `json:"middle_name"`
	LastName             string         `json:"last_name"`
	DateOfBirth          *string        `json:"date_of_birth"`
	Email                string         `json:"email"`
	Phone                *string        `json:"phone"`
	IdentityDocumentType *string        `json:"identity_document_type"`
	IdentityDocument     string         `json:"identity_document"`
	Status               string         `json:"status"`
	EnrolledAt           time.Time      `json:"enrolled_at"`
	ApplicationID        uuid.UUID      `json:"application_id"`
	ProgrammeID          uuid.UUID      `json:"programme_id"`
	ProgrammeTitle       string         `json:"programme_title"`
	IntakeName           *string        `json:"intake_name"`
	CohortID             *uuid.UUID     `json:"cohort_id"`
	CohortName           *string        `json:"cohort_name"`
	Guardians            []Guardian     `json:"guardians"`
	History              []StatusChange `json:"history"`
	Registrations        []Registration `json:"registrations"`
}

// GetStudent returns a student record in scope, or ErrNotFound.
func (s *Service) GetStudent(ctx context.Context, sc Scope, id uuid.UUID) (*Student, error) {
	q := dbq.New(s.pool)
	r, err := q.StaffGetStudent(ctx, dbq.StaffGetStudentParams{ID: id, AllSchools: sc.All, Schools: sc.Schools})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	st := &Student{ID: r.ID, StudentNumber: r.StudentNumber, FirstName: r.FirstName, MiddleName: r.MiddleName, LastName: r.LastName,
		DateOfBirth: dateString(r.DateOfBirth), Email: r.Email, Phone: r.Phone, IdentityDocumentType: r.IdentityDocumentType,
		IdentityDocument: r.IdentityDocument, Status: r.Status, EnrolledAt: r.EnrolledAt, ApplicationID: r.ApplicationID,
		ProgrammeID: r.ProgrammeID, ProgrammeTitle: r.ProgrammeTitle, IntakeName: r.IntakeName, CohortID: r.CohortID, CohortName: r.CohortName,
		Guardians: []Guardian{}, History: []StatusChange{}, Registrations: []Registration{}}
	gs, err := q.ListStudentGuardians(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, g := range gs {
		st.Guardians = append(st.Guardians, Guardian{ID: g.ID, FullName: g.FullName, Relationship: g.Relationship, Phone: g.Phone,
			Email: deref(g.Email), IsPrimary: g.IsPrimary})
	}
	hs, err := q.ListStudentStatusHistory(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, h := range hs {
		st.History = append(st.History, StatusChange{From: h.FromStatus, To: h.ToStatus, Reason: h.Reason, At: h.CreatedAt, ChangedByName: h.ChangedByName})
	}
	regs, err := q.ListStudentRegistrations(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, x := range regs {
		st.Registrations = append(st.Registrations, Registration{ID: x.ID, OfferingID: x.OfferingID, TermName: x.TermName,
			UnitCode: x.UnitCode, UnitTitle: x.UnitTitle, Status: x.Status, At: x.RegisteredAt, DroppedAt: x.DroppedAt})
	}
	return st, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// changeStudent locks a student in scope and runs fn in one transaction.
func (s *Service) changeStudent(ctx context.Context, sc Scope, id uuid.UUID, fn func(q *dbq.Queries, st dbq.LockStudentRow) error) error {
	return s.tx(ctx, func(q *dbq.Queries) error {
		st, err := q.LockStudent(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !sc.Covers(st.SchoolID)) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		return fn(q, st)
	})
}

// SetStatus changes a student's status with a reason. Graduated is final.
func (s *Service) SetStatus(ctx context.Context, p *auth.Principal, sc Scope, id uuid.UUID, status, reason string, req audit.Request) error {
	reason = strings.TrimSpace(reason)
	var pr content.Problems
	pr.OneOf("status", status, StudentStatuses...)
	pr.Required("reason", reason, 2000)
	if err := pr.Err(); err != nil {
		return err
	}
	return s.changeStudent(ctx, sc, id, func(q *dbq.Queries, st dbq.LockStudentRow) error {
		if st.Status == status {
			return ErrUnchanged
		}
		if st.Status == "graduated" {
			return content.Problems{{Field: "status", Message: "A graduated student's status cannot change."}}.Err()
		}
		if err := q.SetStudentStatus(ctx, dbq.SetStudentStatusParams{ID: id, Status: status}); err != nil {
			return err
		}
		from := st.Status
		if err := q.InsertStudentStatusHistory(ctx, dbq.InsertStudentStatusHistoryParams{StudentID: id, FromStatus: &from, ToStatus: status,
			Reason: reason, ChangedBy: &p.UserID}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, entry("student.status_changed", "student", id, map[string]any{"from": from, "to": status}))
	})
}

// SetCohort moves a student to another cohort of their own programme.
func (s *Service) SetCohort(ctx context.Context, sc Scope, id, cohortID uuid.UUID, req audit.Request) error {
	return s.changeStudent(ctx, sc, id, func(q *dbq.Queries, st dbq.LockStudentRow) error {
		c, err := q.StaffGetCohort(ctx, dbq.StaffGetCohortParams{ID: cohortID, AllSchools: true})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && c.ProgrammeID != st.ProgrammeID) {
			return content.Problems{{Field: "cohort_id", Message: "Choose a cohort of the student's own programme."}}.Err()
		}
		if err != nil {
			return err
		}
		if st.CohortID != nil && *st.CohortID == cohortID {
			return ErrUnchanged
		}
		if err := q.SetStudentCohort(ctx, dbq.SetStudentCohortParams{ID: id, CohortID: &cohortID}); err != nil {
			return err
		}
		var from any
		if st.CohortID != nil {
			from = st.CohortID.String()
		}
		return audit.Write(ctx, q, req, entry("student.cohort_changed", "student", id, map[string]any{"from": from, "to": cohortID.String()}))
	})
}

type GuardianInput struct {
	FullName     string `json:"full_name"`
	Relationship string `json:"relationship"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	IsPrimary    bool   `json:"is_primary"`
}

func (in *GuardianInput) check() error {
	in.FullName, in.Phone, in.Email = strings.TrimSpace(in.FullName), strings.TrimSpace(in.Phone), strings.TrimSpace(in.Email)
	var pr content.Problems
	pr.Required("full_name", in.FullName, 120)
	pr.OneOf("relationship", in.Relationship, Relationships...)
	if in.Phone == "" && in.Email == "" {
		pr.Add("phone", "Give a phone number or an email address.")
	}
	pr.Phone("phone", in.Phone)
	pr.Email("email", in.Email)
	return pr.Err()
}

func optionalEmail(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// AddGuardian adds a parent, guardian or next of kin.
func (s *Service) AddGuardian(ctx context.Context, sc Scope, id uuid.UUID, in GuardianInput, req audit.Request) error {
	if err := in.check(); err != nil {
		return err
	}
	return s.changeStudent(ctx, sc, id, func(q *dbq.Queries, _ dbq.LockStudentRow) error {
		gid, err := q.InsertStudentGuardian(ctx, dbq.InsertStudentGuardianParams{StudentID: id, FullName: in.FullName,
			Relationship: in.Relationship, Phone: in.Phone, Email: optionalEmail(in.Email), IsPrimary: in.IsPrimary})
		if err != nil {
			return err
		}
		if in.IsPrimary {
			if err := q.ClearPrimaryGuardian(ctx, dbq.ClearPrimaryGuardianParams{StudentID: id, Keep: gid}); err != nil {
				return err
			}
		}
		return audit.Write(ctx, q, req, entry("student.guardian_added", "student", id, map[string]any{"guardian_id": gid.String()}))
	})
}

// UpdateGuardian corrects a guardian's details.
func (s *Service) UpdateGuardian(ctx context.Context, sc Scope, id, gid uuid.UUID, in GuardianInput, req audit.Request) error {
	if err := in.check(); err != nil {
		return err
	}
	return s.changeStudent(ctx, sc, id, func(q *dbq.Queries, _ dbq.LockStudentRow) error {
		n, err := q.UpdateStudentGuardian(ctx, dbq.UpdateStudentGuardianParams{ID: gid, StudentID: id, FullName: in.FullName,
			Relationship: in.Relationship, Phone: in.Phone, Email: optionalEmail(in.Email), IsPrimary: in.IsPrimary})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		if in.IsPrimary {
			if err := q.ClearPrimaryGuardian(ctx, dbq.ClearPrimaryGuardianParams{StudentID: id, Keep: gid}); err != nil {
				return err
			}
		}
		return audit.Write(ctx, q, req, entry("student.guardian_updated", "student", id, map[string]any{"guardian_id": gid.String()}))
	})
}

// RemoveGuardian takes a guardian off the record (contact details only; the
// audit log keeps that it happened).
func (s *Service) RemoveGuardian(ctx context.Context, sc Scope, id, gid uuid.UUID, req audit.Request) error {
	return s.changeStudent(ctx, sc, id, func(q *dbq.Queries, _ dbq.LockStudentRow) error {
		n, err := q.DeleteStudentGuardian(ctx, dbq.DeleteStudentGuardianParams{ID: gid, StudentID: id})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return audit.Write(ctx, q, req, entry("student.guardian_removed", "student", id, map[string]any{"guardian_id": gid.String()}))
	})
}

// IsStudentStatus reports whether v is a student status.
func IsStudentStatus(v string) bool { return slices.Contains(StudentStatuses, v) }
