package db_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/db"
	"github.com/MboyaDan/dakota-college/backend/internal/testdb"
)

// Migration 00014 files students enrolled before it (Stage 10) into their
// intake's cohort, copies the application's guardian and starts their
// status history.
func TestAcademicsMigrationBackfillsStudents(t *testing.T) {
	url := testdb.Fresh(t)
	ctx := context.Background()
	provider, sqlDB, err := db.NewMigrator(url)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := provider.UpTo(ctx, 13); err != nil {
		t.Fatal(err)
	}

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	for _, sql := range []string{
		`INSERT INTO schools (id, slug, name, short_name, accent_key) VALUES ('00000000-0000-0000-0000-00000000000a', 's-backfill', 'School', 'S', 'ict')`,
		`INSERT INTO programmes (id, school_id, slug, title, level) VALUES ('00000000-0000-0000-0000-00000000000b', '00000000-0000-0000-0000-00000000000a', 'p-backfill', 'Diploma (Sample)', 'diploma')`,
		`INSERT INTO programme_intakes (id, programme_id, name, starts_on) VALUES ('00000000-0000-0000-0000-00000000000c', '00000000-0000-0000-0000-00000000000b', 'January (Sample)', '2027-01-11')`,
		`INSERT INTO users (id, email, full_name, kind) VALUES ('00000000-0000-0000-0000-00000000000d', 'student@example.invalid', 'Test Student', 'student')`,
		`INSERT INTO applicants (id, user_id, email, first_name, last_name) VALUES ('00000000-0000-0000-0000-00000000000e', '00000000-0000-0000-0000-00000000000d', 'student@example.invalid', 'Test', 'Student')`,
		`INSERT INTO applications (id, reference, applicant_id, programme_id, intake_id) VALUES ('00000000-0000-0000-0000-00000000000f', 'DTC-APP-TEST-1', '00000000-0000-0000-0000-00000000000e', '00000000-0000-0000-0000-00000000000b', '00000000-0000-0000-0000-00000000000c')`,
		`INSERT INTO application_guardians (application_id, full_name, relationship, phone) VALUES ('00000000-0000-0000-0000-00000000000f', 'Test Parent', 'mother', '+254 700 000 001')`,
		`INSERT INTO students (id, student_number, user_id, applicant_id, application_id, programme_id, intake_id, first_name, last_name, email, enrolled_by)
		 VALUES ('00000000-0000-0000-0000-000000000010', 'DTC/2027/0001', '00000000-0000-0000-0000-00000000000d', '00000000-0000-0000-0000-00000000000e',
		 '00000000-0000-0000-0000-00000000000f', '00000000-0000-0000-0000-00000000000b', '00000000-0000-0000-0000-00000000000c', 'Test', 'Student', 'student@example.invalid', '00000000-0000-0000-0000-00000000000d')`,
	} {
		if _, err := conn.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	if _, err := provider.UpTo(ctx, 14); err != nil {
		t.Fatal(err)
	}
	var cohort, guardian, history string
	if err := conn.QueryRow(ctx, `SELECT c.name,
		(SELECT full_name FROM student_guardians WHERE student_id = s.id AND is_primary),
		(SELECT to_status || ':' || reason FROM student_status_history WHERE student_id = s.id)
		FROM students s JOIN cohorts c ON c.id = s.cohort_id`).Scan(&cohort, &guardian, &history); err != nil {
		t.Fatal(err)
	}
	if cohort != "Diploma (Sample) · January (Sample)" || guardian != "Test Parent" || history != "active:Enrolled" {
		t.Fatalf("cohort %q, guardian %q, history %q", cohort, guardian, history)
	}
}
