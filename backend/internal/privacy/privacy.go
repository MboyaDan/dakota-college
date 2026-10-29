// Package privacy runs the data retention schedule, keeps the register of
// data subject requests, exports what is held about a person and erases
// applicants who did not enrol (Stage 17c, brief 13, Data Protection Act,
// 2019). Student, academic and financial records are never erased here:
// the college keeps them (docs/retention.md).
package privacy

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/media"
)

// PermManage covers the request register, exports and erasure.
const PermManage = "privacy.manage"

var nairobi = time.FixedZone("EAT", 3*60*60)

var (
	ErrNotFound = errors.New("not found")
	// ErrClosed means the request was already completed or refused.
	ErrClosed = errors.New("request is closed")
	// ErrIdentityNotChecked means an export or erasure was tried before
	// staff recorded how they checked who is asking.
	ErrIdentityNotChecked = errors.New("identity not checked")
	// ErrIsStudent means the applicant became a student; student records
	// are kept (docs/retention.md).
	ErrIsStudent = errors.New("applicant is a student")
	// ErrInProgress means an application is still being decided.
	ErrInProgress = errors.New("application in progress")
	// ErrAlreadyErased means the applicant was erased before.
	ErrAlreadyErased = errors.New("already erased")
	// ErrNotThisPerson means the record does not match the request's email
	// or phone, so it may belong to someone else.
	ErrNotThisPerson = errors.New("record does not match the request")
	// ErrWrongKind means the action does not fit the request: exports are
	// for access and portability requests, erasure for erasure requests.
	ErrWrongKind = errors.New("action does not fit this kind of request")
	// ErrBusy means another retention run holds the lock.
	ErrBusy = errors.New("another retention run is in progress")
)

type Options struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	// Storage removes the files of erased applicants. May be nil when
	// object storage is not configured; removal then waits.
	Storage media.Storage
	// Mode is RETENTION_MODE: off, report or apply.
	Mode string
	// Now is for tests; defaults to time.Now.
	Now func() time.Time
}

type Service struct {
	pool    *pgxpool.Pool
	logger  *slog.Logger
	storage media.Storage
	mode    string
	now     func() time.Time
}

func NewService(o Options) *Service {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Mode == "" {
		o.Mode = ModeReport
	}
	return &Service{pool: o.Pool, logger: o.Logger, storage: o.Storage, mode: o.Mode, now: o.Now}
}

// Mode is the configured retention mode.
func (s *Service) Mode() string { return s.mode }

func (s *Service) tx(ctx context.Context, fn func(q *dbq.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(dbq.New(tx)) })
}

func (s *Service) today() time.Time {
	y, m, d := s.now().In(nairobi).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
