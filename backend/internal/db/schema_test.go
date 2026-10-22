package db_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/testdb"
)

// Database-level guarantees that hold whatever the application code does.

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// expectCode runs sql and requires it to fail with the given SQLSTATE.
func expectCode(t *testing.T, pool *pgxpool.Pool, code, sql string, args ...any) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql, args...)
	if got := pgCode(err); got != code {
		t.Fatalf("%s\nerror %v (SQLSTATE %q), want SQLSTATE %s", sql, err, got, code)
	}
}

const (
	checkViolation    = "23514"
	uniqueViolation   = "23505"
	fkViolation       = "23503"
	restrictViolation = "23001"
)

func TestMoneyIsIntegerCents(t *testing.T) {
	pool, _ := testdb.Migrated(t)
	ctx := context.Background()

	rows, err := pool.Query(ctx, `
		SELECT table_name || '.' || column_name, data_type FROM information_schema.columns
		WHERE table_schema = 'public' AND (column_name LIKE '%cents' OR data_type IN ('real', 'double precision', 'numeric', 'money'))`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var found int
	for rows.Next() {
		var col, typ string
		if err := rows.Scan(&col, &typ); err != nil {
			t.Fatal(err)
		}
		found++
		if typ != "bigint" {
			t.Errorf("%s is %s; money must be bigint cents and nothing may be floating point (NN-03)", col, typ)
		}
	}
	if found == 0 {
		t.Fatal("no *_cents columns found; the check is not looking at anything")
	}

	school := insertSchool(t, pool)
	prog := insertProgramme(t, pool, school)
	expectCode(t, pool, checkViolation, `INSERT INTO fee_items (programme_id, label, category, amount_cents, frequency) VALUES ($1, 'x', 'tuition', -1, 'once')`, prog)
	expectCode(t, pool, checkViolation, `INSERT INTO fee_items (programme_id, label, category, amount_cents, currency, frequency) VALUES ($1, 'x', 'tuition', 1, 'USD', 'once')`, prog)
}

func TestUnverifiedClaimsCannotBePublished(t *testing.T) {
	pool, _ := testdb.Migrated(t)

	expectCode(t, pool, checkViolation, `INSERT INTO statistics (label, value, status) VALUES ('Graduates', '1000', 'published')`)
	expectCode(t, pool, checkViolation, `INSERT INTO statistics (label, value, verified_on, status) VALUES ('Graduates', '1000', '2026-10-01', 'published')`)
	mustExec(t, pool, `INSERT INTO statistics (label, value, source_note, verified_on, status) VALUES ('Graduates', '1000', 'Registry report', '2026-10-01', 'published')`)

	expectCode(t, pool, checkViolation, `INSERT INTO accreditations (body, reference, status) VALUES ('Body', 'REF', 'published')`)
	mustExec(t, pool, `INSERT INTO accreditations (body, reference, status) VALUES ('Body', 'REF', 'draft')`)
}

func TestGalleryNeedsConsentToPublish(t *testing.T) {
	pool, _ := testdb.Migrated(t)
	var fileID uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO files (bucket, object_key, original_name, content_type, size_bytes, sha256)
		VALUES ('public', 'g/1.jpg', '1.jpg', 'image/jpeg', 10, decode(repeat('ab', 32), 'hex')) RETURNING id`).Scan(&fileID); err != nil {
		t.Fatal(err)
	}
	expectCode(t, pool, checkViolation, `INSERT INTO gallery_items (file_id, alt_text, status) VALUES ($1, 'Students in a lab', 'published')`, fileID)
	expectCode(t, pool, checkViolation, `INSERT INTO gallery_items (file_id, alt_text) VALUES ($1, '  ')`, fileID)
	mustExec(t, pool, `INSERT INTO gallery_items (file_id, alt_text, consent_confirmed, status) VALUES ($1, 'Students in a lab', true, 'published')`, fileID)
}

func TestAppendOnlyTables(t *testing.T) {
	pool, _ := testdb.Migrated(t)
	school := insertSchool(t, pool)
	app := insertApplication(t, pool, insertApplicant(t, pool), insertProgramme(t, pool, school))

	mustExec(t, pool, `INSERT INTO audit_logs (action, entity_type) VALUES ('test.action', 'test')`)
	expectCode(t, pool, restrictViolation, `UPDATE audit_logs SET action = 'changed'`)
	expectCode(t, pool, restrictViolation, `DELETE FROM audit_logs`)

	mustExec(t, pool, `INSERT INTO application_status_history (application_id, from_status, to_status) VALUES ($1, 'draft', 'submitted')`, app)
	expectCode(t, pool, restrictViolation, `UPDATE application_status_history SET reason = 'edited'`)
	expectCode(t, pool, restrictViolation, `DELETE FROM application_status_history`)
	expectCode(t, pool, checkViolation, `INSERT INTO application_status_history (application_id, from_status, to_status) VALUES ($1, 'under_review', 'rejected')`, app)

	mustExec(t, pool, `INSERT INTO consents (subject_type, subject_id, purpose, given, notice_version, given_by_name) VALUES ('applicant', gen_random_uuid(), 'data_processing', true, 'v1', 'Test Applicant')`)
	expectCode(t, pool, restrictViolation, `UPDATE consents SET given = false`)
	expectCode(t, pool, restrictViolation, `DELETE FROM consents`)
}

func TestOneActiveApplicationPerProgrammeAndIntake(t *testing.T) {
	pool, _ := testdb.Migrated(t)
	school := insertSchool(t, pool)
	applicant := insertApplicant(t, pool)
	prog := insertProgramme(t, pool, school)

	first := insertApplication(t, pool, applicant, prog)
	expectCode(t, pool, uniqueViolation, `INSERT INTO applications (reference, applicant_id, programme_id) VALUES ('DTC-APP-T-2', $1, $2)`, applicant, prog)

	mustExec(t, pool, `UPDATE applications SET status = 'withdrawn', declaration_accepted_at = now(), submitted_at = now() WHERE id = $1`, first)
	mustExec(t, pool, `INSERT INTO applications (reference, applicant_id, programme_id) VALUES ('DTC-APP-T-3', $1, $2)`, applicant, prog)

	expectCode(t, pool, checkViolation, `INSERT INTO applications (reference, applicant_id, programme_id, status) VALUES ('DTC-APP-T-4', $1, $2, 'submitted')`, insertApplicant(t, pool), prog)
}

func TestRolesAndPermissions(t *testing.T) {
	pool, _ := testdb.Migrated(t)
	ctx := context.Background()
	q := dbq.New(pool)

	holders, err := q.ListRolesWithPermission(ctx, "system.admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(holders) != 1 || holders[0] != "super_admin" {
		t.Fatalf("system.admin granted to %v, want only super_admin (AC-12)", holders)
	}

	var roles, perms int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM roles), (SELECT count(*) FROM permissions)`).Scan(&roles, &perms); err != nil {
		t.Fatal(err)
	}
	if roles != 10 || perms != 23 {
		t.Fatalf("got %d roles and %d permissions, want 10 and 23", roles, perms)
	}

	// A role in use cannot be deleted (brief 11).
	var userID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, full_name, kind) VALUES ('t@example.invalid', 'Test User 001', 'staff') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, pool, `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE key = 'trainer'`, userID)
	expectCode(t, pool, fkViolation, `DELETE FROM roles WHERE key = 'trainer'`)

	expectCode(t, pool, uniqueViolation, `INSERT INTO users (email, full_name, kind) VALUES ('T@Example.Invalid', 'Dup', 'staff')`)
}

func TestDocumentCountersAreGapless(t *testing.T) {
	pool, _ := testdb.Migrated(t)
	ctx := context.Background()

	const workers = 20
	var (
		mu   sync.Mutex
		got  []int64
		wg   sync.WaitGroup
		errs = make(chan error, workers)
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
				v, err := dbq.New(tx).NextCounterValue(ctx, dbq.NextCounterValueParams{Name: "application", Period: 2026})
				if err != nil {
					return err
				}
				mu.Lock()
				got = append(got, v)
				mu.Unlock()
				return nil
			})
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	for i, v := range got {
		if v != int64(i+1) {
			t.Fatalf("counter values %v are not 1..%d without gaps", got, workers)
		}
	}

	// A rolled-back transaction gives its number back.
	_ = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		_, _ = dbq.New(tx).NextCounterValue(ctx, dbq.NextCounterValueParams{Name: "application", Period: 2026})
		return errors.New("roll back")
	})
	next, err := dbq.New(pool).NextCounterValue(ctx, dbq.NextCounterValueParams{Name: "application", Period: 2026})
	if err != nil || next != workers+1 {
		t.Fatalf("after rollback next = %d (%v), want %d", next, err, workers+1)
	}
}

func insertSchool(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO schools (slug, name, short_name, accent_key) VALUES ('s-'||substr(md5(random()::text),1,8), 'School', 'S', 'ict') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertProgramme(t *testing.T, pool *pgxpool.Pool, school uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO programmes (school_id, slug, title, level) VALUES ($1, 'p-'||substr(md5(random()::text),1,8), 'Programme', 'diploma') RETURNING id`, school).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertApplicant(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO applicants (email, first_name, last_name) VALUES ('applicant@example.invalid', 'Test', 'Applicant 001') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertApplication(t *testing.T, pool *pgxpool.Pool, applicant, programme uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO applications (reference, applicant_id, programme_id) VALUES ('DTC-APP-T-'||substr(md5(random()::text),1,8), $1, $2) RETURNING id`, applicant, programme).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
