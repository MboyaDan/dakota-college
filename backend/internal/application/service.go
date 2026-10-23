// Package application is the online application form for applicants
// (brief 8, Stage 9a). The applicant starts an application for a programme
// and intake, then fills it in step by step; every step saves on its own, so
// nothing is lost on a poor connection, and the draft can be resumed later.
//
// Saving checks format only (a date must be a date); what is still missing
// is reported separately, so a half-filled step can always be saved. The
// declaration, documents and submission are Stage 9b.
//
// A draft expires when it has not been saved for applications.draft_expiry_days
// (default 60). Expiry is applied when the draft is next opened, and recorded
// in the status history like any other change.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/notify"
)

const (
	// DefaultDraftExpiryDays applies while the setting is empty.
	DefaultDraftExpiryDays = 60
	// AdultAge: younger applicants need a parent or guardian and their consent.
	AdultAge = 18
	// PrivacyNoticeVersion is recorded with every consent. Change it when the
	// approved privacy notice (DG-06) replaces the draft.
	PrivacyNoticeVersion = "draft-2026-10"
	MaxEducationEntries  = 6
	MaxSubjectGrades     = 20
)

var (
	ErrNotFound    = errors.New("application not found")
	ErrNotEditable = errors.New("application can no longer be changed")
	// ErrClosed means the programme takes no applications now.
	ErrClosed = errors.New("programme is not taking applications")
	// ErrDuplicate means another live application has the same programme and intake.
	ErrDuplicate = errors.New("an application for this programme and intake already exists")
)

// nairobi is Kenya time (UTC+3, no daylight saving): ages, deadlines and
// reference years follow the college's calendar.
var nairobi = time.FixedZone("EAT", 3*60*60)

type Service struct {
	pool       *pgxpool.Pool
	notify     *notify.Service
	logger     *slog.Logger
	appBaseURL string
	now        func() time.Time
}

type Options struct {
	Pool       *pgxpool.Pool
	Notify     *notify.Service
	Logger     *slog.Logger
	AppBaseURL string
	// Now is for tests; defaults to time.Now.
	Now func() time.Time
}

func NewService(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Service{pool: o.Pool, notify: o.Notify, logger: o.Logger, appBaseURL: strings.TrimRight(o.AppBaseURL, "/"), now: now}
}

func (s *Service) today() time.Time {
	y, m, d := s.now().In(nairobi).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (s *Service) tx(ctx context.Context, fn func(q *dbq.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(dbq.New(tx)) })
}

func (s *Service) draftExpiry(ctx context.Context, q *dbq.Queries) (time.Time, error) {
	rows, err := q.GetSiteSettings(ctx, []string{"applications.draft_expiry_days"})
	if err != nil {
		return time.Time{}, err
	}
	days := DefaultDraftExpiryDays
	if len(rows) == 1 {
		if n, err := strconv.Atoi(rows[0].Value); err == nil && n > 0 {
			days = n
		}
	}
	return s.now().Add(time.Duration(days) * 24 * time.Hour), nil
}

// Started is the result of Start.
type Started struct {
	ID uuid.UUID
	// Existing is true when the applicant already had a live application for
	// this programme and intake; that one is returned instead of a new one.
	Existing bool
}

// Start creates a draft for a published programme and one of its open
// intakes, gives it a reference number and emails the reference. Starting
// the same programme and intake again returns the existing application.
func (s *Service) Start(ctx context.Context, p *auth.Principal, programmeSlug string, intakeID *uuid.UUID, req audit.Request) (*Started, error) {
	q := dbq.New(s.pool)
	prog, err := q.GetPublishedProgrammeBySlug(ctx, strings.TrimSpace(programmeSlug))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, content.Problems{{Field: "programme", Message: "Choose a programme from the list."}}.Err()
	}
	if err != nil {
		return nil, err
	}
	open, err := q.ListOpenIntakes(ctx, dbq.ListOpenIntakesParams{ProgrammeID: prog.ID, Today: s.today()})
	if err != nil {
		return nil, err
	}
	if len(open) == 0 {
		return nil, ErrClosed
	}
	switch {
	case intakeID == nil && len(open) == 1:
		intakeID = &open[0].ID
	case intakeID == nil:
		return nil, content.Problems{{Field: "intake_id", Message: "Choose when you want to start."}}.Err()
	case !containsIntake(open, *intakeID):
		return nil, content.Problems{{Field: "intake_id", Message: "That intake is not taking applications. Choose another."}}.Err()
	}

	var out Started
	var reference string
	err = s.tx(ctx, func(q *dbq.Queries) error {
		// One start at a time per applicant, so a double tap makes one draft.
		if err := q.LockApplicantEmail(ctx, p.Email); err != nil {
			return err
		}
		first, last := splitName(p.FullName)
		applicantID, err := q.CreateApplicantProfile(ctx, dbq.CreateApplicantProfileParams{
			UserID: &p.UserID, Email: p.Email, FirstName: first, LastName: last,
		})
		if err != nil {
			return err
		}
		existing, err := q.FindLiveApplication(ctx, dbq.FindLiveApplicationParams{ApplicantID: applicantID, ProgrammeID: prog.ID, IntakeID: intakeID})
		if err == nil {
			out = Started{ID: existing, Existing: true}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		year := s.now().In(nairobi).Year()
		n, err := q.NextCounterValue(ctx, dbq.NextCounterValueParams{Name: "application", Period: int32(year)})
		if err != nil {
			return err
		}
		reference = fmt.Sprintf("DTC-APP-%d-%06d", year, n)
		expires, err := s.draftExpiry(ctx, q)
		if err != nil {
			return err
		}
		id, err := q.InsertApplication(ctx, dbq.InsertApplicationParams{
			Reference: reference, ApplicantID: applicantID, ProgrammeID: prog.ID, IntakeID: intakeID, DraftExpiresAt: &expires,
		})
		if err != nil {
			return err
		}
		if err := q.InsertStatusHistory(ctx, dbq.InsertStatusHistoryParams{ApplicationID: id, ToStatus: "draft", Reason: "Started online"}); err != nil {
			return err
		}
		out.ID = id
		return audit.Write(ctx, q, req, audit.Entry{Action: "application.started", EntityType: "application", EntityID: id.String(),
			Details: map[string]any{"reference": reference}})
	})
	if err != nil {
		return nil, err
	}
	if !out.Existing {
		s.sendStarted(ctx, p, out.ID, reference, prog.Title)
	}
	return &out, nil
}

// sendStarted tells the applicant their reference number (brief 8). A
// failure here is logged, never undoes the application.
func (s *Service) sendStarted(ctx context.Context, p *auth.Principal, id uuid.UUID, reference, programme string) {
	name := p.FullName
	if first, _ := splitName(name); first == "" {
		name = "there"
	}
	m := notify.Message{
		Template: notify.ApplicationStarted, To: p.Email,
		Vars:        map[string]string{"name": name, "reference": reference, "programme": programme, "link": s.appBaseURL + "/apply/resume"},
		RelatedType: "application", RelatedID: id.String(),
	}
	if err := s.notify.Email(ctx, m); err != nil {
		s.logger.Error("application started email not queued", slog.String("application_id", id.String()), slog.String("error", err.Error()))
	}
}

func containsIntake(open []dbq.ListOpenIntakesRow, id uuid.UUID) bool {
	for _, i := range open {
		if i.ID == id {
			return true
		}
	}
	return false
}

// splitName gives a starting first and last name from the name typed on the
// sign-in form; the applicant corrects them in the form.
func splitName(full string) (first, last string) {
	f := strings.Fields(full)
	if len(f) == 0 || (len(f) == 1 && f[0] == "Applicant") {
		return "", ""
	}
	return f[0], strings.Join(f[1:], " ")
}

// load reads the application for its owner, expiring it first if it is a
// draft past its expiry date.
func (s *Service) load(ctx context.Context, userID, id uuid.UUID) (dbq.GetOwnApplicationRow, error) {
	q := dbq.New(s.pool)
	a, err := q.GetOwnApplication(ctx, dbq.GetOwnApplicationParams{ID: id, UserID: &userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, err
	}
	if s.due(a.Status, a.DraftExpiresAt) {
		if err := s.tx(ctx, func(q *dbq.Queries) error {
			lk, err := q.LockOwnApplication(ctx, dbq.LockOwnApplicationParams{ID: id, UserID: &userID})
			if err != nil {
				return err
			}
			return s.expire(ctx, q, lk.ID, lk.Status, lk.DraftExpiresAt)
		}); err != nil {
			return a, err
		}
		return q.GetOwnApplication(ctx, dbq.GetOwnApplicationParams{ID: id, UserID: &userID})
	}
	return a, nil
}

func (s *Service) due(status string, expires *time.Time) bool {
	return status == "draft" && expires != nil && !s.now().Before(*expires)
}

func (s *Service) expire(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, expires *time.Time) error {
	if !s.due(status, expires) {
		return nil
	}
	n, err := q.ExpireDraft(ctx, id)
	if err != nil || n == 0 {
		return err
	}
	return q.InsertStatusHistory(ctx, dbq.InsertStatusHistoryParams{
		ApplicationID: id, FromStatus: &status, ToStatus: "expired", Reason: "Draft not saved before its expiry date",
	})
}

// edit runs fn on a draft the user owns, inside one transaction, and
// extends the draft's expiry. Expired and submitted applications refuse.
func (s *Service) edit(ctx context.Context, userID, id uuid.UUID, fn func(q *dbq.Queries, a dbq.LockOwnApplicationRow) error) error {
	expired := false
	err := s.tx(ctx, func(q *dbq.Queries) error {
		a, err := q.LockOwnApplication(ctx, dbq.LockOwnApplicationParams{ID: id, UserID: &userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if s.due(a.Status, a.DraftExpiresAt) {
			expired = true
			return s.expire(ctx, q, a.ID, a.Status, a.DraftExpiresAt)
		}
		if a.Status != "draft" {
			return ErrNotEditable
		}
		if err := fn(q, a); err != nil {
			return err
		}
		expires, err := s.draftExpiry(ctx, q)
		if err != nil {
			return err
		}
		// Draft saves are the applicant's own work in progress and are not
		// audited one by one; submission and every status change are.
		return q.ExtendDraft(ctx, dbq.ExtendDraftParams{ID: id, DraftExpiresAt: &expires})
	})
	if err == nil && expired {
		return ErrNotEditable
	}
	return err
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}
