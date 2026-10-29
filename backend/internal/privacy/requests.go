package privacy

import (
	"context"
	"errors"
	"fmt"
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

// Request kinds, matching the rights in the privacy notice.
var Kinds = []string{"access", "correction", "erasure", "objection", "portability", "withdraw_consent", "other"}

// DueDays is the default time to answer, counted from the day the request
// arrived. Staff can set another due date. A-49: to be confirmed with the
// college's legal adviser (DG-06).
const DueDays = 7

// RequestInput records a new request.
type RequestInput struct {
	Kind             string `json:"kind"`
	RequesterName    string `json:"requester_name"`
	RequesterContact string `json:"requester_contact"`
	ActingFor        string `json:"acting_for"`
	SubjectEmail     string `json:"subject_email"`
	SubjectPhone     string `json:"subject_phone"`
	Details          string `json:"details"`
	// ReceivedOn and DueOn are YYYY-MM-DD. Empty means today, and
	// DueDays after ReceivedOn.
	ReceivedOn string `json:"received_on"`
	DueOn      string `json:"due_on"`
}

type RequestSummary struct {
	ID            uuid.UUID  `json:"id"`
	Reference     string     `json:"reference"`
	Kind          string     `json:"kind"`
	RequesterName string     `json:"requester_name"`
	Status        string     `json:"status"`
	ReceivedOn    string     `json:"received_on"`
	DueOn         string     `json:"due_on"`
	Overdue       bool       `json:"overdue"`
	ClosedAt      *time.Time `json:"closed_at"`
}

type RequestList struct {
	Items []RequestSummary `json:"items"`
	Total int64            `json:"total"`
}

type Event struct {
	Action    string    `json:"action"`
	Note      string    `json:"note"`
	ActorName string    `json:"actor_name"`
	At        time.Time `json:"at"`
}

type Request struct {
	RequestSummary
	RequesterContact      string     `json:"requester_contact"`
	ActingFor             string     `json:"acting_for"`
	SubjectEmail          *string    `json:"subject_email"`
	SubjectPhone          *string    `json:"subject_phone"`
	Details               string     `json:"details"`
	IdentityCheckedAt     *time.Time `json:"identity_checked_at"`
	IdentityCheckedByName *string    `json:"identity_checked_by_name"`
	IdentityMethod        string     `json:"identity_method"`
	Outcome               string     `json:"outcome"`
	ClosedByName          *string    `json:"closed_by_name"`
	CreatedByName         string     `json:"created_by_name"`
	CreatedAt             time.Time  `json:"created_at"`
	Events                []Event    `json:"events"`
	// Matches are the records found by the request's email and phone.
	Matches *Matches `json:"matches"`
}

func (s *Service) parseDate(pr *content.Problems, field, v string, fallback time.Time) time.Time {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	t, err := time.Parse(time.DateOnly, strings.TrimSpace(v))
	if err != nil {
		pr.Add(field, "Enter the date as YYYY-MM-DD.")
		return fallback
	}
	return t
}

func optional(v string) *string {
	if v = strings.TrimSpace(v); v == "" {
		return nil
	}
	return &v
}

// Create records a request as it arrived.
func (s *Service) Create(ctx context.Context, actor *auth.Principal, req audit.Request, in RequestInput) (*Request, error) {
	var pr content.Problems
	pr.OneOf("kind", in.Kind, Kinds...)
	pr.Required("requester_name", in.RequesterName, 200)
	pr.Required("requester_contact", in.RequesterContact, 200)
	pr.Optional("acting_for", in.ActingFor, 200)
	pr.Optional("details", in.Details, 5000)
	in.SubjectEmail = auth.NormaliseEmail(in.SubjectEmail)
	pr.Email("subject_email", in.SubjectEmail)
	pr.Phone("subject_phone", strings.TrimSpace(in.SubjectPhone))
	if in.SubjectEmail == "" && strings.TrimSpace(in.SubjectPhone) == "" {
		pr.Add("subject_email", "Give the email address or phone number we hold for the person, so their records can be found.")
	}
	today := s.today()
	received := s.parseDate(&pr, "received_on", in.ReceivedOn, today)
	if received.After(today) {
		pr.Add("received_on", "The date received cannot be in the future.")
	}
	due := s.parseDate(&pr, "due_on", in.DueOn, received.AddDate(0, 0, DueDays))
	if due.Before(received) {
		pr.Add("due_on", "The due date cannot be before the date received.")
	}
	if err := pr.Err(); err != nil {
		return nil, err
	}
	var id uuid.UUID
	err := s.tx(ctx, func(q *dbq.Queries) error {
		year := int32(received.Year())
		seq, err := q.NextCounterValue(ctx, dbq.NextCounterValueParams{Name: "data_request", Period: year})
		if err != nil {
			return err
		}
		ref := fmt.Sprintf("DTC-DSR-%d-%04d", year, seq)
		id, err = q.InsertDataRequest(ctx, dbq.InsertDataRequestParams{
			Reference: ref, Kind: in.Kind, RequesterName: strings.TrimSpace(in.RequesterName),
			RequesterContact: strings.TrimSpace(in.RequesterContact), ActingFor: strings.TrimSpace(in.ActingFor),
			SubjectEmail: optional(in.SubjectEmail), SubjectPhone: optional(in.SubjectPhone),
			Details: strings.TrimSpace(in.Details), ReceivedOn: received, DueOn: due, CreatedBy: actor.UserID,
		})
		if err != nil {
			return err
		}
		if err := q.InsertDataRequestEvent(ctx, dbq.InsertDataRequestEventParams{RequestID: id, Action: "recorded", ActorID: actor.UserID}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "privacy.request_recorded", EntityType: "data_request", EntityID: id.String(),
			Details: map[string]any{"reference": ref, "kind": in.Kind}})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *Service) summary(id uuid.UUID, ref, kind, name, status string, received, due time.Time, closed *time.Time) RequestSummary {
	return RequestSummary{
		ID: id, Reference: ref, Kind: kind, RequesterName: name, Status: status,
		ReceivedOn: received.Format(time.DateOnly), DueOn: due.Format(time.DateOnly),
		Overdue: status == "open" && due.Before(s.today()), ClosedAt: closed,
	}
}

// List shows open requests first, soonest due first. status may be empty.
func (s *Service) List(ctx context.Context, status string, page int) (*RequestList, error) {
	if status != "" && !slices.Contains([]string{"open", "completed", "refused"}, status) {
		return nil, content.Problems{{Field: "status", Message: "Choose open, completed or refused."}}.Err()
	}
	const per = 50
	if page < 1 {
		page = 1
	}
	q := dbq.New(s.pool)
	st := optional(status)
	rows, err := q.ListDataRequests(ctx, dbq.ListDataRequestsParams{Status: st, MaxRows: per, Skip: int32((page - 1) * per)})
	if err != nil {
		return nil, err
	}
	total, err := q.CountDataRequests(ctx, st)
	if err != nil {
		return nil, err
	}
	l := &RequestList{Items: make([]RequestSummary, len(rows)), Total: total}
	for i, r := range rows {
		l.Items[i] = s.summary(r.ID, r.Reference, r.Kind, r.RequesterName, r.Status, r.ReceivedOn, r.DueOn, r.ClosedAt)
	}
	return l, nil
}

// Get returns one request with its history and the records it matches.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Request, error) {
	q := dbq.New(s.pool)
	d, err := q.GetDataRequest(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	events, err := q.ListDataRequestEvents(ctx, id)
	if err != nil {
		return nil, err
	}
	m, err := s.find(ctx, q, d.SubjectEmail, d.SubjectPhone)
	if err != nil {
		return nil, err
	}
	r := &Request{
		RequestSummary:   s.summary(d.ID, d.Reference, d.Kind, d.RequesterName, d.Status, d.ReceivedOn, d.DueOn, d.ClosedAt),
		RequesterContact: d.RequesterContact, ActingFor: d.ActingFor, SubjectEmail: d.SubjectEmail, SubjectPhone: d.SubjectPhone,
		Details: d.Details, IdentityCheckedAt: d.IdentityCheckedAt, IdentityCheckedByName: d.IdentityCheckedByName,
		IdentityMethod: d.IdentityMethod, Outcome: d.Outcome, ClosedByName: d.ClosedByName, CreatedByName: d.CreatedByName,
		CreatedAt: d.CreatedAt, Events: make([]Event, len(events)), Matches: m,
	}
	for i, e := range events {
		r.Events[i] = Event{Action: e.Action, Note: e.Note, ActorName: e.ActorName, At: e.CreatedAt}
	}
	return r, nil
}

// act runs fn on an open request, then records the event and an audit row.
func (s *Service) act(ctx context.Context, actor *auth.Principal, req audit.Request, id uuid.UUID, action, note string,
	fn func(q *dbq.Queries, d dbq.LockDataRequestRow) (map[string]any, error)) error {
	return s.tx(ctx, func(q *dbq.Queries) error {
		d, err := q.LockDataRequest(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if d.Status != "open" {
			return ErrClosed
		}
		details := map[string]any{"reference": d.Reference}
		if fn != nil {
			extra, err := fn(q, d)
			if err != nil {
				return err
			}
			for k, v := range extra {
				details[k] = v
			}
		}
		if err := q.InsertDataRequestEvent(ctx, dbq.InsertDataRequestEventParams{RequestID: id, Action: action, Note: note, ActorID: actor.UserID}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "privacy.request_" + action, EntityType: "data_request", EntityID: id.String(), Details: details})
	})
}

// CheckIdentity records how staff confirmed who is asking, for example
// "Saw national ID at the front office". Exports and erasure need it.
func (s *Service) CheckIdentity(ctx context.Context, actor *auth.Principal, req audit.Request, id uuid.UUID, method string) error {
	var pr content.Problems
	pr.Required("method", method, 500)
	if err := pr.Err(); err != nil {
		return err
	}
	method = strings.TrimSpace(method)
	return s.act(ctx, actor, req, id, "identity_checked", method, func(q *dbq.Queries, _ dbq.LockDataRequestRow) (map[string]any, error) {
		return nil, q.CheckDataRequestIdentity(ctx, dbq.CheckDataRequestIdentityParams{ID: id, ActorID: &actor.UserID, Method: method})
	})
}

// AddNote records what was done, for example a correction made on the
// student's record.
func (s *Service) AddNote(ctx context.Context, actor *auth.Principal, req audit.Request, id uuid.UUID, note string) error {
	var pr content.Problems
	pr.Required("note", note, 5000)
	if err := pr.Err(); err != nil {
		return err
	}
	return s.act(ctx, actor, req, id, "note", strings.TrimSpace(note), nil)
}

// Close completes or refuses a request. The outcome says what was done or
// why it was refused; it is what the person is told.
func (s *Service) Close(ctx context.Context, actor *auth.Principal, req audit.Request, id uuid.UUID, status, outcome string) error {
	var pr content.Problems
	pr.OneOf("status", status, "completed", "refused")
	pr.Required("outcome", outcome, 5000)
	if err := pr.Err(); err != nil {
		return err
	}
	outcome = strings.TrimSpace(outcome)
	return s.act(ctx, actor, req, id, status, outcome, func(q *dbq.Queries, _ dbq.LockDataRequestRow) (map[string]any, error) {
		return nil, q.CloseDataRequest(ctx, dbq.CloseDataRequestParams{ID: id, Status: status, Outcome: outcome, ActorID: &actor.UserID})
	})
}

// Erase erases one applicant or enquiry found by an erasure request. The
// record must be among the request's matches, and identity checked first.
func (s *Service) Erase(ctx context.Context, actor *auth.Principal, req audit.Request, id uuid.UUID, recordType string, recordID uuid.UUID) error {
	var pr content.Problems
	pr.OneOf("record_type", recordType, "applicant", "enquiry")
	if err := pr.Err(); err != nil {
		return err
	}
	var removed bool
	err := s.act(ctx, actor, req, id, "erased", recordType+" "+recordID.String(), func(q *dbq.Queries, d dbq.LockDataRequestRow) (map[string]any, error) {
		if d.Kind != "erasure" {
			return nil, ErrWrongKind
		}
		if d.IdentityCheckedAt == nil {
			return nil, ErrIdentityNotChecked
		}
		m, err := s.find(ctx, q, d.SubjectEmail, d.SubjectPhone)
		if err != nil {
			return nil, err
		}
		switch recordType {
		case "applicant":
			if !slices.ContainsFunc(m.Applicants, func(a ApplicantMatch) bool { return a.ID == recordID }) {
				return nil, ErrNotThisPerson
			}
			if err := eraseApplicant(ctx, q, recordID, req, "request "+d.Reference); err != nil {
				return nil, err
			}
		case "enquiry":
			if !slices.ContainsFunc(m.Enquiries, func(e EnquiryMatch) bool { return e.ID == recordID }) {
				return nil, ErrNotThisPerson
			}
			if _, err := q.DeleteEnquiryByID(ctx, recordID); err != nil {
				return nil, err
			}
		}
		removed = true
		return map[string]any{"record_type": recordType, "record_id": recordID.String()}, nil
	})
	if err == nil && removed && recordType == "applicant" {
		// Best effort: anything left is retried by the next retention run.
		_ = s.removeFiles(ctx, ModeApply, &RunResult{})
	}
	return err
}
