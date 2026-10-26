package portal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/academics"
	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/notify"
)

// The staff side of the portal: notices (academics.manage within scope;
// trainers for the classes they teach, academics.teach), service requests
// and profile change requests (students.view to read, students.update to
// act, within scope).

// ---------- notices ----------

type StaffNotice struct {
	Notice
	Withdrawn bool      `json:"withdrawn"`
	CreatedBy uuid.UUID `json:"created_by"`
}

type NoticeInput struct {
	Audience   string     `json:"audience"`
	SchoolID   *uuid.UUID `json:"school_id"`
	CohortID   *uuid.UUID `json:"cohort_id"`
	OfferingID *uuid.UUID `json:"offering_id"`
	Title      string     `json:"title"`
	Body       string     `json:"body"`
	EndsOn     string     `json:"ends_on"`
}

func (s *Service) ListNotices(ctx context.Context, sc academics.Scope, offering *uuid.UUID) ([]StaffNotice, error) {
	rows, err := dbq.New(s.pool).StaffListNotices(ctx, dbq.StaffListNoticesParams{OfferingID: offering, AllSchools: sc.All, Schools: sc.Schools})
	if err != nil {
		return nil, err
	}
	out := make([]StaffNotice, 0, len(rows))
	for _, n := range rows {
		out = append(out, StaffNotice{Notice: Notice{ID: n.ID, Audience: n.Audience, AudienceName: n.AudienceName, Title: n.Title, Body: n.Body,
			EndsOn: dateString(n.EndsOn), AuthorName: n.AuthorName, CreatedAt: n.CreatedAt}, Withdrawn: n.WithdrawnAt != nil, CreatedBy: n.CreatedBy})
	}
	return out, nil
}

func (in *NoticeInput) check(today time.Time) (*time.Time, error) {
	in.Title, in.Body = strings.TrimSpace(in.Title), strings.TrimSpace(in.Body)
	var pr content.Problems
	pr.Required("title", in.Title, 160)
	pr.Required("body", in.Body, 5000)
	var ends *time.Time
	if v := strings.TrimSpace(in.EndsOn); v != "" {
		d, err := time.Parse(time.DateOnly, v)
		switch {
		case err != nil:
			pr.Add("ends_on", "Enter the date as YYYY-MM-DD.")
		case d.Before(today):
			pr.Add("ends_on", "Choose today or a later date.")
		default:
			ends = &d
		}
	}
	return ends, pr.Err()
}

// PostNotice posts to everyone (needs schools.all), a school or a cohort in
// scope, or a class whose unit is in scope.
func (s *Service) PostNotice(ctx context.Context, p *auth.Principal, sc academics.Scope, in NoticeInput, req audit.Request) (uuid.UUID, error) {
	ends, err := in.check(s.today())
	if err != nil {
		return uuid.Nil, err
	}
	params := dbq.InsertNoticeParams{Audience: in.Audience, Title: in.Title, Body: in.Body, EndsOn: ends, CreatedBy: p.UserID}
	var id uuid.UUID
	err = s.tx(ctx, func(q *dbq.Queries) error {
		switch in.Audience {
		case "all":
			if !sc.All {
				return ErrCollegeAll
			}
		case "school":
			if in.SchoolID == nil {
				return content.Problems{{Field: "school_id", Message: "Choose a school."}}.Err()
			}
			if !sc.Covers(*in.SchoolID) {
				return academics.ErrOutsideScope
			}
			params.SchoolID = in.SchoolID
		case "cohort":
			if in.CohortID == nil {
				return content.Problems{{Field: "cohort_id", Message: "Choose a cohort."}}.Err()
			}
			if _, err := q.StaffGetCohort(ctx, dbq.StaffGetCohortParams{ID: *in.CohortID, AllSchools: sc.All, Schools: sc.Schools}); errors.Is(err, pgx.ErrNoRows) {
				return academics.ErrOutsideScope
			} else if err != nil {
				return err
			}
			params.CohortID = in.CohortID
		case "offering":
			if in.OfferingID == nil {
				return content.Problems{{Field: "offering_id", Message: "Choose a class."}}.Err()
			}
			o, err := q.GetOffering(ctx, *in.OfferingID)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && !sc.Covers(o.SchoolID)) {
				return academics.ErrOutsideScope
			}
			if err != nil {
				return err
			}
			params.OfferingID = in.OfferingID
		default:
			return content.Problems{{Field: "audience", Message: "Choose who the notice is for."}}.Err()
		}
		if id, err = q.InsertNotice(ctx, params); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "notice.posted", EntityType: "notice", EntityID: id.String(),
			Details: map[string]any{"audience": in.Audience}})
	})
	return id, err
}

// PostClassNotice is a trainer's notice to a class they teach.
func (s *Service) PostClassNotice(ctx context.Context, p *auth.Principal, offering uuid.UUID, in NoticeInput, req audit.Request) (uuid.UUID, error) {
	ok, err := dbq.New(s.pool).IsAssignedTrainer(ctx, dbq.IsAssignedTrainerParams{OfferingID: offering, UserID: p.UserID})
	if err != nil {
		return uuid.Nil, err
	}
	if !ok {
		return uuid.Nil, ErrNotFound
	}
	in.Audience, in.OfferingID = "offering", &offering
	// The trainer's scope for this one notice is the class itself.
	o, err := dbq.New(s.pool).GetOffering(ctx, offering)
	if err != nil {
		return uuid.Nil, err
	}
	return s.PostNotice(ctx, p, academics.Scope{Schools: []uuid.UUID{o.SchoolID}}, in, req)
}

// ClassNotices are the notices of a class the trainer teaches.
func (s *Service) ClassNotices(ctx context.Context, p *auth.Principal, offering uuid.UUID) ([]StaffNotice, error) {
	ok, err := dbq.New(s.pool).IsAssignedTrainer(ctx, dbq.IsAssignedTrainerParams{OfferingID: offering, UserID: p.UserID})
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return s.ListNotices(ctx, academics.Scope{All: true}, &offering)
}

// WithdrawNotice takes a notice down. Managers may withdraw any notice in
// scope; anyone may withdraw their own (trainers use this).
func (s *Service) WithdrawNotice(ctx context.Context, p *auth.Principal, sc academics.Scope, manager bool, id uuid.UUID, req audit.Request) error {
	return s.tx(ctx, func(q *dbq.Queries) error {
		n, err := q.LockNotice(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		inScope := sc.All
		for _, school := range []*uuid.UUID{n.SchoolID, n.CohortSchoolID, n.UnitSchoolID} {
			if school != nil && sc.Covers(*school) {
				inScope = true
			}
		}
		if !(n.CreatedBy == p.UserID || (manager && inScope)) {
			return ErrNotFound
		}
		if n.WithdrawnAt != nil {
			return ErrUnchanged
		}
		if err := q.WithdrawNotice(ctx, dbq.WithdrawNoticeParams{ID: id, WithdrawnBy: &p.UserID}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "notice.withdrawn", EntityType: "notice", EntityID: id.String()})
	})
}

// ---------- service requests ----------

type StaffServiceRequest struct {
	ServiceRequest
	StudentID     uuid.UUID `json:"student_id"`
	StudentNumber string    `json:"student_number"`
	StudentName   string    `json:"student_name"`
	HandledByName *string   `json:"handled_by_name"`
}

func (s *Service) ListServiceRequests(ctx context.Context, sc academics.Scope, status string) ([]StaffServiceRequest, error) {
	var st *string
	if status != "" {
		st = &status
	}
	rows, err := dbq.New(s.pool).StaffListServiceRequests(ctx, dbq.StaffListServiceRequestsParams{AllSchools: sc.All, Schools: sc.Schools, Status: st})
	if err != nil {
		return nil, err
	}
	out := make([]StaffServiceRequest, 0, len(rows))
	for _, r := range rows {
		out = append(out, StaffServiceRequest{
			ServiceRequest: ServiceRequest{ID: r.ID, Kind: r.Kind, Details: r.Details, Status: r.Status, Response: r.Response, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt},
			StudentID:      r.StudentID, StudentNumber: r.StudentNumber, StudentName: fullName(r.FirstName, r.MiddleName, r.LastName), HandledByName: r.HandledByName})
	}
	return out, nil
}

var requestKindLabels = map[string]string{
	"letter": "a letter", "transcript": "a transcript", "deferment": "deferment", "change_of_course": "a change of course", "other": "your request",
}
var requestStatusLabels = map[string]string{"open": "received", "in_progress": "in progress", "done": "done", "declined": "declined"}

// UpdateServiceRequest changes a request's status, with a response the
// student reads (required to decline), and emails the student.
func (s *Service) UpdateServiceRequest(ctx context.Context, p *auth.Principal, sc academics.Scope, id uuid.UUID, status, response string, req audit.Request) error {
	response = strings.TrimSpace(response)
	var pr content.Problems
	pr.OneOf("status", status, "in_progress", "done", "declined")
	pr.Optional("response", response, 2000)
	if status == "declined" && response == "" {
		pr.Add("response", "Say why the request is declined. The student will read it.")
	}
	if err := pr.Err(); err != nil {
		return err
	}
	var to, name, kind string
	err := s.tx(ctx, func(q *dbq.Queries) error {
		r, err := q.LockServiceRequest(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !sc.Covers(r.SchoolID)) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if r.Status == "done" || r.Status == "declined" {
			return content.Problems{{Field: "status", Message: "This request is closed."}}.Err()
		}
		if err := q.UpdateServiceRequest(ctx, dbq.UpdateServiceRequestParams{ID: id, Status: status, Response: response, HandledBy: &p.UserID}); err != nil {
			return err
		}
		to, name, kind = r.Email, r.FirstName, r.Kind
		return audit.Write(ctx, q, req, audit.Entry{Action: "student.request_updated", EntityType: "service_request", EntityID: id.String(),
			Details: map[string]any{"from": r.Status, "to": status}})
	})
	if err != nil {
		return err
	}
	s.tell(ctx, notify.StudentRequestUpdated, to, id.String(), map[string]string{
		"name": name, "kind": requestKindLabels[kind], "status": requestStatusLabels[status], "response": response,
		"link": s.appBaseURL + "/portal/requests"})
	return nil
}

// ---------- profile change requests ----------

type StaffChangeRequest struct {
	ChangeRequest
	StudentID     uuid.UUID `json:"student_id"`
	StudentNumber string    `json:"student_number"`
	StudentName   string    `json:"student_name"`
	DecidedByName *string   `json:"decided_by_name"`
}

func (s *Service) ListChangeRequests(ctx context.Context, sc academics.Scope, status string) ([]StaffChangeRequest, error) {
	var st *string
	if status != "" {
		st = &status
	}
	rows, err := dbq.New(s.pool).StaffListProfileChangeRequests(ctx, dbq.StaffListProfileChangeRequestsParams{AllSchools: sc.All, Schools: sc.Schools, Status: st})
	if err != nil {
		return nil, err
	}
	out := make([]StaffChangeRequest, 0, len(rows))
	for _, r := range rows {
		c, err := changeRequest(r.ID, r.OldValues, r.NewValues, r.Reason, r.Status, r.Decision, r.DecidedAt, r.CreatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, StaffChangeRequest{ChangeRequest: c, StudentID: r.StudentID, StudentNumber: r.StudentNumber,
			StudentName: fullName(r.FirstName, r.MiddleName, r.LastName), DecidedByName: r.DecidedByName})
	}
	return out, nil
}

// DecideChange approves (applying the new values to the student record and
// account name) or declines (with a reason) a pending change request.
func (s *Service) DecideChange(ctx context.Context, p *auth.Principal, sc academics.Scope, id uuid.UUID, approve bool, decision string, req audit.Request) error {
	decision = strings.TrimSpace(decision)
	var pr content.Problems
	pr.Optional("decision", decision, 2000)
	if !approve && decision == "" {
		pr.Add("decision", "Say why the change is declined. The student will read it.")
	}
	if err := pr.Err(); err != nil {
		return err
	}
	var to, name string
	status := "declined"
	if approve {
		status = "approved"
	}
	err := s.tx(ctx, func(q *dbq.Queries) error {
		r, err := q.LockProfileChangeRequest(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !sc.Covers(r.SchoolID)) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if r.Status != "pending" {
			return ErrUnchanged
		}
		if approve {
			var nv map[string]string
			if err := json.Unmarshal(r.NewValues, &nv); err != nil {
				return err
			}
			opt := func(k string) *string {
				if v, ok := nv[k]; ok {
					return &v
				}
				return nil
			}
			var dob *time.Time
			if v, ok := nv["date_of_birth"]; ok {
				d, err := time.Parse(time.DateOnly, v)
				if err != nil {
					return err
				}
				dob = &d
			}
			if err := q.ApplyProfileChange(ctx, dbq.ApplyProfileChangeParams{ID: r.StudentID, FirstName: opt("first_name"), MiddleName: opt("middle_name"),
				LastName: opt("last_name"), DateOfBirth: dob, IdentityDocument: opt("identity_document")}); err != nil {
				return err
			}
			if err := q.SyncStudentAccountName(ctx, r.StudentID); err != nil {
				return err
			}
		}
		if err := q.DecideProfileChangeRequest(ctx, dbq.DecideProfileChangeRequestParams{ID: id, Status: status, Decision: decision, DecidedBy: &p.UserID}); err != nil {
			return err
		}
		to, name = r.Email, r.FirstName
		return audit.Write(ctx, q, req, audit.Entry{Action: "student.change_" + status, EntityType: "student", EntityID: r.StudentID.String(),
			Details: map[string]any{"request_id": id.String()}})
	})
	if err != nil {
		return err
	}
	s.tell(ctx, notify.StudentProfileChangeDecided, to, id.String(), map[string]string{
		"name": name, "decision": status, "reason": decision, "link": s.appBaseURL + "/portal/profile"})
	return nil
}
