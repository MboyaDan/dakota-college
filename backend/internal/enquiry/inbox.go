package enquiry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/mail"
	"github.com/MboyaDan/dakota-college/backend/internal/outbox"
)

// The staff inbox (Stage 8a): enquiries.view to read, enquiries.manage to
// assign, change status and add notes (checked on each route). Every change
// is audited in the same transaction, and the audit log is the history.

const PermView, PermManage = "enquiries.view", "enquiries.manage"

var (
	ErrNotFound      = errors.New("enquiry not found")
	ErrNotAssignable = errors.New("that person cannot see the enquiry inbox")
	ErrUnchanged     = errors.New("nothing to change")
	ErrInvalidStatus = errors.New("unknown status")
	ErrNoteEmpty     = errors.New("note is empty")
	ErrNoteTooLong   = errors.New("note is too long")
)

const (
	inboxPageSize = 25
	maxNoteLength = 5000
)

var settableStatuses = []string{"open", "waiting", "closed"}

// officeQueryTypes groups query types by the office that handles them; it
// matches the email routing in Submit (D-71).
var officeQueryTypes = map[string][]string{
	"admissions": {"admissions", "programmes"},
	"finance":    {"fees"},
	"general":    {"student_life", "alumni", "other"},
}

// Office is the office that handles a query type; it matches the email
// routing in Submit.
func Office(queryType string) string {
	for office, types := range officeQueryTypes {
		for _, t := range types {
			if t == queryType {
				return office
			}
		}
	}
	return "general"
}

type InboxFilter struct {
	Office     string // admissions, finance or general; empty for all
	Status     string // new, open, waiting or closed; empty for all
	AssignedTo *uuid.UUID
	Unassigned bool
	Search     string
	Page       int
}

type InboxItem struct {
	ID             uuid.UUID  `json:"id"`
	FullName       string     `json:"full_name"`
	Email          *string    `json:"email"`
	Phone          *string    `json:"phone"`
	QueryType      string     `json:"query_type"`
	Office         string     `json:"office"`
	ProgrammeTitle *string    `json:"programme_title"`
	Status         string     `json:"status"`
	AssignedTo     *uuid.UUID `json:"assigned_to"`
	AssigneeName   *string    `json:"assignee_name"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type InboxPage struct {
	Items      []InboxItem `json:"items"`
	Page       int         `json:"page"`
	TotalPages int         `json:"total_pages"`
	Total      int64       `json:"total"`
}

func optionalText(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// List returns one page of the inbox, newest first.
func (s *Service) List(ctx context.Context, f InboxFilter) (*InboxPage, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	var types []string
	if f.Office != "" {
		types = officeQueryTypes[f.Office]
		if types == nil {
			return nil, fmt.Errorf("unknown office %q", f.Office)
		}
	}
	search := optionalText(f.Search)
	if search != nil && len(*search) > 100 {
		trimmed := (*search)[:100]
		search = &trimmed
	}
	q := dbq.New(s.pool)
	total, err := q.StaffCountEnquiries(ctx, dbq.StaffCountEnquiriesParams{
		Status: optionalText(f.Status), Types: types, AssignedTo: f.AssignedTo, Unassigned: f.Unassigned, Search: search,
	})
	if err != nil {
		return nil, err
	}
	rows, err := q.StaffListEnquiries(ctx, dbq.StaffListEnquiriesParams{
		Status: optionalText(f.Status), Types: types, AssignedTo: f.AssignedTo, Unassigned: f.Unassigned, Search: search,
		MaxItems: int32(inboxPageSize), Skip: int32((f.Page - 1) * inboxPageSize),
	})
	if err != nil {
		return nil, err
	}
	out := &InboxPage{Items: make([]InboxItem, 0, len(rows)), Page: f.Page, Total: total,
		TotalPages: int((total + int64(inboxPageSize) - 1) / int64(inboxPageSize))}
	for _, r := range rows {
		out.Items = append(out.Items, InboxItem{ID: r.ID, FullName: r.FullName, Email: r.Email, Phone: r.Phone, QueryType: r.QueryType,
			Office: Office(r.QueryType), ProgrammeTitle: r.ProgrammeTitle, Status: r.Status, AssignedTo: r.AssignedTo,
			AssigneeName: r.AssigneeName, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt})
	}
	return out, nil
}

type Note struct {
	ID         uuid.UUID `json:"id"`
	Body       string    `json:"body"`
	AuthorName string    `json:"author_name"`
	CreatedAt  time.Time `json:"created_at"`
}

type HistoryEntry struct {
	At        time.Time      `json:"at"`
	Action    string         `json:"action"`
	ActorName *string        `json:"actor_name"`
	Details   map[string]any `json:"details"`
}

type Detail struct {
	InboxItem
	Message       string         `json:"message"`
	Source        string         `json:"source"`
	ProgrammeSlug *string        `json:"programme_slug"`
	ClosedAt      *time.Time     `json:"closed_at"`
	Notes         []Note         `json:"notes"`
	History       []HistoryEntry `json:"history"`
}

// Get returns one enquiry with its notes and history.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Detail, error) {
	q := dbq.New(s.pool)
	r, err := q.StaffGetEnquiry(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d := &Detail{
		InboxItem: InboxItem{ID: r.ID, FullName: r.FullName, Email: r.Email, Phone: r.Phone, QueryType: r.QueryType,
			Office: Office(r.QueryType), ProgrammeTitle: r.ProgrammeTitle, Status: r.Status, AssignedTo: r.AssignedTo,
			AssigneeName: r.AssigneeName, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt},
		Message: r.Message, Source: r.Source, ProgrammeSlug: r.ProgrammeSlug, ClosedAt: r.ClosedAt,
		Notes: []Note{}, History: []HistoryEntry{},
	}
	notes, err := q.StaffListEnquiryNotes(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, n := range notes {
		d.Notes = append(d.Notes, Note{ID: n.ID, Body: n.Body, AuthorName: n.AuthorName, CreatedAt: n.CreatedAt})
	}
	history, err := q.StaffEnquiryHistory(ctx, id.String())
	if err != nil {
		return nil, err
	}
	for _, h := range history {
		e := HistoryEntry{At: h.OccurredAt, Action: h.Action, ActorName: h.ActorName, Details: map[string]any{}}
		if err := json.Unmarshal(h.Details, &e.Details); err != nil {
			return nil, err
		}
		d.History = append(d.History, e)
	}
	return d, nil
}

type Assignee struct {
	ID       uuid.UUID `json:"id"`
	FullName string    `json:"full_name"`
}

// Assignees lists the active staff who can be given an enquiry.
func (s *Service) Assignees(ctx context.Context) ([]Assignee, error) {
	rows, err := dbq.New(s.pool).ListEnquiryAssignees(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Assignee, 0, len(rows))
	for _, r := range rows {
		out = append(out, Assignee{ID: r.ID, FullName: r.FullName})
	}
	return out, nil
}

func entry(action string, id uuid.UUID, details map[string]any) audit.Entry {
	return audit.Entry{Action: action, EntityType: "enquiry", EntityID: id.String(), Details: details}
}

// change locks the enquiry and runs fn in one transaction.
func (s *Service) change(ctx context.Context, id uuid.UUID, fn func(q *dbq.Queries, cur dbq.LockEnquiryRow) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		cur, err := q.LockEnquiry(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		return fn(q, cur)
	})
}

// Assign gives the enquiry to someone who can see the inbox, or clears the
// owner when assignee is nil. The new owner is told by email, without the
// visitor's details.
func (s *Service) Assign(ctx context.Context, id uuid.UUID, assignee *uuid.UUID, req audit.Request) error {
	var notify *dbq.ListEnquiryAssigneesRow
	err := s.change(ctx, id, func(q *dbq.Queries, cur dbq.LockEnquiryRow) error {
		if sameUser(cur.AssignedTo, assignee) {
			return ErrUnchanged
		}
		details := map[string]any{"from": idOrNil(cur.AssignedTo), "to": idOrNil(assignee)}
		if assignee != nil {
			people, err := q.ListEnquiryAssignees(ctx)
			if err != nil {
				return err
			}
			for i := range people {
				if people[i].ID == *assignee {
					notify = &people[i]
					details["to_name"] = people[i].FullName
				}
			}
			if notify == nil {
				return ErrNotAssignable
			}
		}
		if err := q.SetEnquiryAssignee(ctx, dbq.SetEnquiryAssigneeParams{ID: id, AssignedTo: assignee}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, entry("enquiry.assigned", id, details))
	})
	if err != nil {
		return err
	}
	// Assigning to yourself needs no email.
	if notify != nil && (req.ActorID == nil || *req.ActorID != notify.ID) {
		s.send(ctx, outbox.SystemEnquiryAssigned, mail.Message{
			To:      notify.Email,
			Subject: "A website enquiry was assigned to you",
			Text: fmt.Sprintf("Hello %s,\n\nA website enquiry was assigned to you in the staff portal. Open it here:\n\n%s/staff/enquiries/%s\n\nDakota Training College\n",
				notify.FullName, s.appBaseURL, id),
		}, id)
	}
	return nil
}

// SetStatus moves the enquiry to open, waiting or closed. Closing records
// the time; reopening clears it. "new" is only for enquiries nobody has
// handled yet.
func (s *Service) SetStatus(ctx context.Context, id uuid.UUID, status string, req audit.Request) error {
	valid := false
	for _, v := range settableStatuses {
		valid = valid || v == status
	}
	if !valid {
		return ErrInvalidStatus
	}
	return s.change(ctx, id, func(q *dbq.Queries, cur dbq.LockEnquiryRow) error {
		if cur.Status == status {
			return ErrUnchanged
		}
		if err := q.SetEnquiryStatus(ctx, dbq.SetEnquiryStatusParams{ID: id, Status: status}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, entry("enquiry.status_changed", id, map[string]any{"from": cur.Status, "to": status}))
	})
}

// AddNote records an internal note, visible only to staff. A note on a new
// enquiry also marks it open: someone is handling it.
func (s *Service) AddNote(ctx context.Context, id uuid.UUID, author uuid.UUID, body string, req audit.Request) (*Note, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, ErrNoteEmpty
	}
	if len([]rune(body)) > maxNoteLength {
		return nil, ErrNoteTooLong
	}
	var note *Note
	err := s.change(ctx, id, func(q *dbq.Queries, cur dbq.LockEnquiryRow) error {
		row, err := q.InsertEnquiryNote(ctx, dbq.InsertEnquiryNoteParams{EnquiryID: id, AuthorID: author, Body: body})
		if err != nil {
			return err
		}
		note = &Note{ID: row.ID, Body: body, CreatedAt: row.CreatedAt}
		// The note text stays out of the audit log; it lives in enquiry_notes.
		if err := audit.Write(ctx, q, req, entry("enquiry.note_added", id, map[string]any{"note_id": row.ID.String()})); err != nil {
			return err
		}
		if cur.Status == "new" {
			if err := q.SetEnquiryStatus(ctx, dbq.SetEnquiryStatusParams{ID: id, Status: "open"}); err != nil {
				return err
			}
			return audit.Write(ctx, q, req, entry("enquiry.status_changed", id, map[string]any{"from": "new", "to": "open"}))
		}
		return nil
	})
	return note, err
}

type Counts struct {
	New  int64 `json:"new"`
	Mine int64 `json:"mine"`
}

// Counts are the numbers on the staff overview: unhandled enquiries, and
// open ones assigned to the user.
func (s *Service) Counts(ctx context.Context, user uuid.UUID) (Counts, error) {
	r, err := dbq.New(s.pool).CountOpenEnquiries(ctx, &user)
	if err != nil {
		return Counts{}, err
	}
	return Counts{New: r.NewCount, Mine: r.MineCount}, nil
}

func (s *Service) send(ctx context.Context, key string, m mail.Message, id uuid.UUID) {
	if _, err := s.outbox.Email(ctx, outbox.Item{SystemKey: key, RelatedType: "enquiry", RelatedID: id.String()}, m); err != nil {
		s.logger.Error("enquiry email not queued", slog.String("enquiry_id", id.String()), slog.String("error", err.Error()))
	}
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
