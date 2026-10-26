// Package portal is the student portal (brief 7.1, Stage 12a) and the staff
// side of what students ask for: notices, service requests and changes to
// personal details.
//
// A student reaches only their own record (AC-08): every student method
// starts from the signed-in user and finds their student row; no student
// route takes a student id. Staff methods use the academics school scope.
package portal

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/academics"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/notify"
)

var (
	// ErrNoStudent: the account has no student record (should not happen
	// for student accounts; refused rather than guessed).
	ErrNoStudent = errors.New("no student record for this account")
	ErrNotFound  = errors.New("not found")
	// ErrNotActive: only active students register for units.
	ErrNotActive = errors.New("student is not active")
	// ErrClosed: registration for that class is not open to this student today.
	ErrClosed     = errors.New("registration closed")
	ErrPending    = errors.New("a change request is already waiting")
	ErrUnchanged  = errors.New("nothing to change")
	ErrTooMany    = errors.New("too many open requests")
	ErrNotYours   = errors.New("not your notice or class")
	ErrCollegeAll = errors.New("needs schools.all")
)

var nairobi = time.FixedZone("EAT", 3*60*60)

type Service struct {
	pool       *pgxpool.Pool
	academics  *academics.Service
	notify     *notify.Service
	logger     *slog.Logger
	appBaseURL string
	now        func() time.Time
}

type Options struct {
	Pool       *pgxpool.Pool
	Academics  *academics.Service
	Notify     *notify.Service
	Logger     *slog.Logger
	AppBaseURL string
	Now        func() time.Time
}

func NewService(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Service{pool: o.Pool, academics: o.Academics, notify: o.Notify, logger: o.Logger,
		appBaseURL: strings.TrimRight(o.AppBaseURL, "/"), now: now}
}

func (s *Service) today() time.Time {
	y, m, d := s.now().In(nairobi).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (s *Service) tx(ctx context.Context, fn func(q *dbq.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(dbq.New(tx)) })
}

// own finds the signed-in student's record.
func (s *Service) own(ctx context.Context, q *dbq.Queries, user uuid.UUID) (dbq.GetStudentByUserRow, error) {
	st, err := q.GetStudentByUser(ctx, user)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, ErrNoStudent
	}
	return st, err
}

// ownScope is a scope that covers only the student's own school, so the
// academics service can be reused for the student's own record.
func ownScope(st dbq.GetStudentByUserRow) academics.Scope {
	return academics.Scope{Schools: []uuid.UUID{st.SchoolID}}
}

func dateString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := t.Format(time.DateOnly)
	return &v
}

func fullName(first, middle, last string) string {
	return strings.Join(strings.Fields(first+" "+middle+" "+last), " ")
}

// tell emails a student; a failure is logged and never undoes the change.
func (s *Service) tell(ctx context.Context, template, to, related string, vars map[string]string) {
	m := notify.Message{Template: template, To: to, Vars: vars, RelatedType: "student", RelatedID: related}
	if err := s.notify.Email(ctx, m); err != nil {
		s.logger.Error("student email not queued", slog.String("template", template), slog.String("error", err.Error()))
	}
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

// LoginEmail is the sign-in address of a student number. An unknown number
// maps to an address no account has, so sign-in fails the ordinary way.
func (s *Service) LoginEmail(ctx context.Context, number string) (string, error) {
	email, err := dbq.New(s.pool).GetStudentLoginEmail(ctx, number)
	if errors.Is(err, pgx.ErrNoRows) {
		return "unknown-student-" + strings.ToLower(strings.NewReplacer("/", "-", " ", "-").Replace(number)) + "@student.invalid", nil
	}
	return email, err
}
