// Package academics holds student records and the core of academics (brief
// 7.2 and 10, Stage 11a): students, guardians, cohorts, terms, units,
// programme units, unit offerings with trainers and a timetable, and unit
// registration.
//
// Who sees what (AC-08, D-133):
//   - students.view reads student records; students.update changes them.
//   - academics.manage manages units, cohorts, offerings, timetables,
//     trainer assignments and registration.
//   - Both reach every school only with schools.all; otherwise only the
//     schools the person is assigned to (Heads of School). No assignment
//     means nothing: the rule fails closed.
//   - academics.teach shows a trainer only the offerings they are assigned
//     to, their timetable and class lists. Trainers have no students.view.
//
// Records outside someone's scope answer 404, as if they did not exist.
package academics

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

const (
	PermStudentsView   = "students.view"
	PermStudentsUpdate = "students.update"
	PermManage         = "academics.manage"
	PermTeach          = "academics.teach"
	PermAllSchools     = "schools.all"
)

const pageSize = 25

var (
	ErrNotFound = errors.New("not found")
	// ErrOutsideScope: the record is in a school the person does not work in.
	ErrOutsideScope = errors.New("outside your schools")
	// ErrCollegeWide: the change affects every school (terms), so it needs schools.all.
	ErrCollegeWide = errors.New("needs schools.all")
	ErrDuplicate   = errors.New("already exists")
	ErrUnchanged   = errors.New("nothing to change")
)

var nairobi = time.FixedZone("EAT", 3*60*60)

type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{pool: pool, now: now}
}

func (s *Service) today() time.Time {
	y, m, d := s.now().In(nairobi).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (s *Service) tx(ctx context.Context, fn func(q *dbq.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(dbq.New(tx)) })
}

// Scope is the set of schools someone may work in.
type Scope struct {
	All     bool
	Schools []uuid.UUID
}

// Covers reports whether a record in school is within the scope.
func (sc Scope) Covers(school uuid.UUID) bool { return sc.All || slices.Contains(sc.Schools, school) }

// ScopeFor reads the principal's scope: every school with schools.all,
// otherwise their school assignments.
func (s *Service) ScopeFor(ctx context.Context, p *auth.Principal) (Scope, error) {
	if p.Has(PermAllSchools) {
		return Scope{All: true}, nil
	}
	ids, err := dbq.New(s.pool).ListUserSchoolIDs(ctx, p.UserID)
	if err != nil {
		return Scope{}, err
	}
	return Scope{Schools: ids}, nil
}

func entry(action, entityType string, id uuid.UUID, details map[string]any) audit.Entry {
	return audit.Entry{Action: action, EntityType: entityType, EntityID: id.String(), Details: details}
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func optionalText(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if r := []rune(v); len(r) > 100 {
		v = string(r[:100])
	}
	return &v
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

func pages(total int64) int { return int((total + pageSize - 1) / pageSize) }

func isForeignKey(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23503"
}
