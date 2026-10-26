// Package admissions is the staff side of the application workflow (brief 8,
// Stage 10a): the inbox, review, requests for more information, rejection,
// offers with a PDF letter, enrolment and the admissions report.
//
// Permissions are checked by the router on every route: admissions.view to
// read, admissions.review to assign, add notes and ask for information,
// admissions.offer to offer or reject, admissions.enrol to enrol. Every
// status change goes through application.Move under the row lock, so it is
// recorded with who, when and why, and a change that is no longer allowed
// (someone else acted first) is refused.
package admissions

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/application"
	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/media"
	"github.com/MboyaDan/dakota-college/backend/internal/notify"
)

const (
	PermView   = "admissions.view"
	PermReview = "admissions.review"
	PermOffer  = "admissions.offer"
	PermEnrol  = "admissions.enrol"
)

const (
	inboxPageSize = 25
	maxNoteLength = 5000
	maxTextLength = 2000
	// DefaultOfferResponseDays applies while the setting is empty.
	DefaultOfferResponseDays = 14
	maxOfferResponseDays     = 365
)

var (
	ErrNotFound      = errors.New("application not found")
	ErrNotAssignable = errors.New("that person cannot review applications")
	ErrUnchanged     = errors.New("nothing to change")
	ErrStorageOff    = errors.New("document storage is not configured")
	// ErrNotApplicantAccount: enrolment needs the applicant's own account,
	// and it is not one (already a student, or changed by hand).
	ErrNotApplicantAccount = errors.New("the applicant's account cannot become a student account")
)

// reviewing are the statuses in which an application is with the office.
var reviewing = []string{"submitted", "under_review", "info_requested"}

var nairobi = time.FixedZone("EAT", 3*60*60)

type Service struct {
	pool       *pgxpool.Pool
	notify     *notify.Service
	logger     *slog.Logger
	appBaseURL string
	now        func() time.Time
	storage    media.Storage
	bucket     string
}

type Options struct {
	Pool       *pgxpool.Pool
	Notify     *notify.Service
	Logger     *slog.Logger
	AppBaseURL string
	// Storage and PrivateBucket keep offer letters; nil Storage refuses offers.
	Storage       media.Storage
	PrivateBucket string
	// Now is for tests; defaults to time.Now.
	Now func() time.Time
}

func NewService(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Service{pool: o.Pool, notify: o.Notify, logger: o.Logger, appBaseURL: strings.TrimRight(o.AppBaseURL, "/"),
		now: now, storage: o.Storage, bucket: o.PrivateBucket}
}

// today is the Nairobi date, as midnight UTC (how pgx reads a date).
func (s *Service) today() time.Time {
	y, m, d := s.now().In(nairobi).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (s *Service) tx(ctx context.Context, fn func(q *dbq.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(dbq.New(tx)) })
}

// expireOffers lapses offers past their respond-by date, so lists and
// screens never show a stale "offered".
func (s *Service) expireOffers(ctx context.Context) error {
	_, err := dbq.New(s.pool).ExpireOverdueOffers(ctx, s.today())
	return err
}

func optionalText(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if len([]rune(v)) > 100 {
		v = string([]rune(v)[:100])
	}
	return &v
}

func fullName(first, middle, last string) string {
	return strings.Join(strings.Fields(first+" "+middle+" "+last), " ")
}

func dateString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := t.Format(time.DateOnly)
	return &v
}

// ---------- inbox ----------

type Filter struct {
	Status      string
	ProgrammeID *uuid.UUID
	IntakeID    *uuid.UUID
	AssignedTo  *uuid.UUID
	Unassigned  bool
	Search      string
	Page        int
}

type InboxItem struct {
	ID             uuid.UUID  `json:"id"`
	Reference      string     `json:"reference"`
	Status         string     `json:"status"`
	ApplicantName  string     `json:"applicant_name"`
	Email          string     `json:"email"`
	ProgrammeTitle string     `json:"programme_title"`
	IntakeName     *string    `json:"intake_name"`
	IntakeStartsOn *string    `json:"intake_starts_on"`
	AssignedTo     *uuid.UUID `json:"assigned_to"`
	AssigneeName   *string    `json:"assignee_name"`
	RespondBy      *string    `json:"respond_by"`
	SubmittedAt    time.Time  `json:"submitted_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type InboxPage struct {
	Items      []InboxItem `json:"items"`
	Page       int         `json:"page"`
	TotalPages int         `json:"total_pages"`
	Total      int64       `json:"total"`
}

// IsStatus reports whether v is an application status staff can filter by.
func IsStatus(v string) bool { return v != "draft" && slices.Contains(application.Statuses, v) }

// List returns one page of submitted applications, newest first.
func (s *Service) List(ctx context.Context, f Filter) (*InboxPage, error) {
	if err := s.expireOffers(ctx); err != nil {
		return nil, err
	}
	if f.Page < 1 {
		f.Page = 1
	}
	status := optionalText(f.Status)
	search := optionalText(f.Search)
	q := dbq.New(s.pool)
	total, err := q.StaffCountApplications(ctx, dbq.StaffCountApplicationsParams{
		Status: status, ProgrammeID: f.ProgrammeID, IntakeID: f.IntakeID, AssignedTo: f.AssignedTo, Unassigned: f.Unassigned, Search: search,
	})
	if err != nil {
		return nil, err
	}
	rows, err := q.StaffListApplications(ctx, dbq.StaffListApplicationsParams{
		Status: status, ProgrammeID: f.ProgrammeID, IntakeID: f.IntakeID, AssignedTo: f.AssignedTo, Unassigned: f.Unassigned, Search: search,
		MaxItems: inboxPageSize, Skip: int32((f.Page - 1) * inboxPageSize),
	})
	if err != nil {
		return nil, err
	}
	out := &InboxPage{Items: make([]InboxItem, 0, len(rows)), Page: f.Page, Total: total,
		TotalPages: int((total + inboxPageSize - 1) / inboxPageSize)}
	for _, r := range rows {
		item := InboxItem{ID: r.ID, Reference: r.Reference, Status: r.Status, ApplicantName: fullName(r.FirstName, r.MiddleName, r.LastName),
			Email: r.Email, ProgrammeTitle: r.ProgrammeTitle, IntakeName: r.IntakeName, IntakeStartsOn: dateString(r.IntakeStartsOn),
			AssignedTo: r.AssignedTo, AssigneeName: r.AssigneeName, UpdatedAt: r.UpdatedAt}
		if r.SubmittedAt != nil {
			item.SubmittedAt = *r.SubmittedAt
		}
		if r.Status == "offered" {
			item.RespondBy = dateString(r.RespondBy)
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

type Counts struct {
	New           int64 `json:"new"`
	Mine          int64 `json:"mine"`
	InfoRequested int64 `json:"info_requested"`
	ToEnrol       int64 `json:"to_enrol"`
}

// Counts are the numbers on the staff overview.
func (s *Service) Counts(ctx context.Context, user uuid.UUID) (Counts, error) {
	if err := s.expireOffers(ctx); err != nil {
		return Counts{}, err
	}
	r, err := dbq.New(s.pool).CountApplicationsForInbox(ctx, &user)
	if err != nil {
		return Counts{}, err
	}
	return Counts{New: r.NewCount, Mine: r.MineCount, InfoRequested: r.InfoRequestedCount, ToEnrol: r.ToEnrolCount}, nil
}

type Assignee struct {
	ID       uuid.UUID `json:"id"`
	FullName string    `json:"full_name"`
}

// Assignees lists active staff who can review applications.
func (s *Service) Assignees(ctx context.Context) ([]Assignee, error) {
	rows, err := dbq.New(s.pool).ListAdmissionsAssignees(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Assignee, 0, len(rows))
	for _, r := range rows {
		out = append(out, Assignee{ID: r.ID, FullName: r.FullName})
	}
	return out, nil
}

// ---------- one application ----------

type Applicant struct {
	Name  string  `json:"name"`
	Email string  `json:"email"`
	Phone *string `json:"phone"`
}

type Document struct {
	ID          uuid.UUID `json:"id"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	ScanStatus  string    `json:"scan_status"`
	CreatedAt   time.Time `json:"created_at"`
	// AfterSubmission: sent later, in reply to a request for information.
	AfterSubmission bool `json:"after_submission"`
}

type HistoryEntry struct {
	From          *string   `json:"from"`
	To            string    `json:"to"`
	Reason        string    `json:"reason"`
	At            time.Time `json:"at"`
	ChangedByName *string   `json:"changed_by_name"`
}

type Note struct {
	ID         uuid.UUID `json:"id"`
	Body       string    `json:"body"`
	AuthorName string    `json:"author_name"`
	CreatedAt  time.Time `json:"created_at"`
}

type InfoRequest struct {
	ID              uuid.UUID  `json:"id"`
	Message         string     `json:"message"`
	RequestedAt     time.Time  `json:"requested_at"`
	RequestedByName string     `json:"requested_by_name"`
	Response        string     `json:"response"`
	ClosedAt        *time.Time `json:"closed_at"`
	// ClosedByName is set when staff closed it; empty when the applicant replied.
	ClosedByName *string `json:"closed_by_name"`
}

type Offer struct {
	IssuedAt     time.Time  `json:"issued_at"`
	IssuedByName string     `json:"issued_by_name"`
	RespondBy    string     `json:"respond_by"`
	Conditions   string     `json:"conditions"`
	AcceptedAt   *time.Time `json:"accepted_at"`
	DeclinedAt   *time.Time `json:"declined_at"`
	HasLetter    bool       `json:"has_letter"`
}

type Student struct {
	ID            uuid.UUID `json:"id"`
	StudentNumber string    `json:"student_number"`
	Status        string    `json:"status"`
	EnrolledAt    time.Time `json:"enrolled_at"`
	InvoiceID     uuid.UUID `json:"invoice_id"`
	InvoiceStatus string    `json:"invoice_status"`
}

// Actions the viewer can take now, given the status and their permissions.
const (
	ActAssign       = "assign"
	ActNote         = "note"
	ActRequestInfo  = "request_info"
	ActResumeReview = "resume_review"
	ActOffer        = "offer"
	ActReject       = "reject"
	ActEnrol        = "enrol"
)

// Actions lists what someone with these permissions may do to an
// application in this status. The routes check the same permissions; the
// status is checked again under the row lock.
func Actions(status string, has func(string) bool) []string {
	out := []string{}
	add := func(perm, action string, ok bool) {
		if ok && has(perm) {
			out = append(out, action)
		}
	}
	add(PermReview, ActAssign, slices.Contains(reviewing, status))
	add(PermReview, ActNote, true)
	add(PermReview, ActRequestInfo, status == "under_review")
	add(PermReview, ActResumeReview, status == "info_requested")
	add(PermOffer, ActOffer, status == "under_review")
	add(PermOffer, ActReject, application.CanMove(status, "rejected"))
	add(PermEnrol, ActEnrol, status == "offer_accepted")
	return out
}

type Detail struct {
	ID             uuid.UUID             `json:"id"`
	Reference      string                `json:"reference"`
	Status         string                `json:"status"`
	ProgrammeID    uuid.UUID             `json:"programme_id"`
	ProgrammeTitle string                `json:"programme_title"`
	IntakeID       *uuid.UUID            `json:"intake_id"`
	IntakeName     *string               `json:"intake_name"`
	IntakeStartsOn *string               `json:"intake_starts_on"`
	Applicant      Applicant             `json:"applicant"`
	AssignedTo     *uuid.UUID            `json:"assigned_to"`
	AssigneeName   *string               `json:"assignee_name"`
	SubmittedAt    time.Time             `json:"submitted_at"`
	DecidedAt      *time.Time            `json:"decided_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
	Submitted      *application.Snapshot `json:"submitted"`
	Documents      []Document            `json:"documents"`
	History        []HistoryEntry        `json:"history"`
	Notes          []Note                `json:"notes"`
	InfoRequests   []InfoRequest         `json:"info_requests"`
	Offer          *Offer                `json:"offer"`
	Student        *Student              `json:"student"`
	Actions        []string              `json:"actions"`
}

// Get returns a submitted application with everything staff need to decide.
func (s *Service) Get(ctx context.Context, p *auth.Principal, id uuid.UUID) (*Detail, error) {
	if err := s.expireOffers(ctx); err != nil {
		return nil, err
	}
	q := dbq.New(s.pool)
	r, err := q.StaffGetApplication(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d := &Detail{
		ID: r.ID, Reference: r.Reference, Status: r.Status, ProgrammeID: r.ProgrammeID, ProgrammeTitle: r.ProgrammeTitle,
		IntakeID: r.IntakeID, IntakeName: r.IntakeName, IntakeStartsOn: dateString(r.IntakeStartsOn),
		Applicant:  Applicant{Name: fullName(r.FirstName, "", r.LastName), Email: r.Email, Phone: r.Phone},
		AssignedTo: r.AssignedTo, AssigneeName: r.AssigneeName, DecidedAt: r.DecidedAt, UpdatedAt: r.UpdatedAt,
		Documents: []Document{}, History: []HistoryEntry{}, Notes: []Note{}, InfoRequests: []InfoRequest{},
		Actions: Actions(r.Status, p.Has),
	}
	if r.SubmittedAt != nil {
		d.SubmittedAt = *r.SubmittedAt
	}
	if len(r.SubmittedSnapshot) > 0 {
		var snap application.Snapshot
		if err := json.Unmarshal(r.SubmittedSnapshot, &snap); err != nil {
			return nil, err
		}
		d.Submitted = &snap
		d.Applicant.Name = fullName(snap.Personal.FirstName, snap.Personal.MiddleName, snap.Personal.LastName)
	}

	docs, err := q.StaffListApplicationDocuments(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, x := range docs {
		d.Documents = append(d.Documents, Document{ID: x.ID, Kind: x.Kind, Name: x.OriginalName, ContentType: x.ContentType,
			SizeBytes: x.SizeBytes, ScanStatus: x.ScanStatus, CreatedAt: x.CreatedAt, AfterSubmission: x.CreatedAt.After(d.SubmittedAt)})
	}
	hist, err := q.StaffStatusHistory(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, h := range hist {
		d.History = append(d.History, HistoryEntry{From: h.FromStatus, To: h.ToStatus, Reason: h.Reason, At: h.CreatedAt, ChangedByName: h.ChangedByName})
	}
	notes, err := q.ListApplicationNotes(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, n := range notes {
		d.Notes = append(d.Notes, Note{ID: n.ID, Body: n.Body, AuthorName: n.AuthorName, CreatedAt: n.CreatedAt})
	}
	reqs, err := q.ListInfoRequests(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, x := range reqs {
		d.InfoRequests = append(d.InfoRequests, InfoRequest{ID: x.ID, Message: x.Message, RequestedAt: x.RequestedAt, RequestedByName: x.RequestedByName,
			Response: x.Response, ClosedAt: x.ClosedAt, ClosedByName: x.ClosedByName})
	}
	o, err := q.GetOffer(ctx, id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return nil, err
	default:
		d.Offer = &Offer{IssuedAt: o.IssuedAt, IssuedByName: o.IssuedByName, RespondBy: o.RespondBy.Format(time.DateOnly),
			Conditions: o.Conditions, AcceptedAt: o.AcceptedAt, DeclinedAt: o.DeclinedAt, HasLetter: o.LetterFileID != nil}
	}
	st, err := q.GetStudentByApplication(ctx, id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return nil, err
	default:
		d.Student = &Student{ID: st.ID, StudentNumber: st.StudentNumber, Status: st.Status, EnrolledAt: st.EnrolledAt,
			InvoiceID: st.InvoiceID, InvoiceStatus: st.InvoiceStatus}
	}
	return d, nil
}

// change locks a submitted application and runs fn in one transaction.
func (s *Service) change(ctx context.Context, id uuid.UUID, fn func(q *dbq.Queries, a dbq.LockApplicationRow) error) error {
	return s.tx(ctx, func(q *dbq.Queries) error {
		a, err := q.LockApplication(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && a.SubmittedAt == nil) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		return fn(q, a)
	})
}

func entry(action string, id uuid.UUID, details map[string]any) audit.Entry {
	return audit.Entry{Action: action, EntityType: "application", EntityID: id.String(), Details: details}
}

// Assign gives the application to someone who can review applications, or
// clears the owner (nil). Assigning a submitted application starts its
// review (brief 8: under_review means assigned to an officer).
func (s *Service) Assign(ctx context.Context, p *auth.Principal, id uuid.UUID, assignee *uuid.UUID, req audit.Request) error {
	return s.change(ctx, id, func(q *dbq.Queries, a dbq.LockApplicationRow) error {
		if !slices.Contains(reviewing, a.Status) {
			return application.ErrIllegalTransition
		}
		if sameUser(a.AssignedTo, assignee) {
			return ErrUnchanged
		}
		details := map[string]any{"from": idOrNil(a.AssignedTo), "to": idOrNil(assignee)}
		name := ""
		if assignee != nil {
			people, err := q.ListAdmissionsAssignees(ctx)
			if err != nil {
				return err
			}
			i := slices.IndexFunc(people, func(r dbq.ListAdmissionsAssigneesRow) bool { return r.ID == *assignee })
			if i < 0 {
				return ErrNotAssignable
			}
			name = people[i].FullName
			details["to_name"] = name
		}
		if err := q.SetApplicationAssignee(ctx, dbq.SetApplicationAssigneeParams{ID: id, AssignedTo: assignee}); err != nil {
			return err
		}
		if a.Status == "submitted" && assignee != nil {
			if err := application.Move(ctx, q, id, a.Status, "under_review", &p.UserID, "Review started by "+name); err != nil {
				return err
			}
		}
		return audit.Write(ctx, q, req, entry("application.assigned", id, details))
	})
}

// AddNote records an internal note, never shown to the applicant.
func (s *Service) AddNote(ctx context.Context, p *auth.Principal, id uuid.UUID, body string, req audit.Request) error {
	body = strings.TrimSpace(body)
	var pr content.Problems
	pr.Required("body", body, maxNoteLength)
	if err := pr.Err(); err != nil {
		return err
	}
	return s.change(ctx, id, func(q *dbq.Queries, a dbq.LockApplicationRow) error {
		n, err := q.InsertApplicationNote(ctx, dbq.InsertApplicationNoteParams{ApplicationID: id, AuthorID: p.UserID, Body: body})
		if err != nil {
			return err
		}
		// The note text stays out of the audit log; it lives in application_notes.
		return audit.Write(ctx, q, req, entry("application.note_added", id, map[string]any{"note_id": n.ID.String()}))
	})
}

func sameUser(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func idOrNil(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

// offerResponseDays reads admissions.offer_response_days (default 14).
func offerResponseDays(ctx context.Context, q *dbq.Queries) (int, error) {
	rows, err := q.GetSiteSettings(ctx, []string{"admissions.offer_response_days"})
	if err != nil {
		return 0, err
	}
	if len(rows) == 1 {
		if n, err := strconv.Atoi(strings.TrimSpace(rows[0].Value)); err == nil && n > 0 && n <= maxOfferResponseDays {
			return n, nil
		}
	}
	return DefaultOfferResponseDays, nil
}
