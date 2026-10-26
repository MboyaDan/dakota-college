package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// Sample academics (Stage 11b, 12b): a current term with registration open
// around today, a notice to everyone, three ICT units on the ICT
// diploma, two offerings (one taught by the test trainer), four enrolled
// test students in the diploma's cohort, and the medical Head of School
// assigned to their school. Idempotent like the rest of the seed.

const (
	sampleTermName   = "Current term (Sample)"
	sampleProgramme  = "example-information-technology-diploma"
	sampleStudents   = 4
	sampleHeadEmail  = "head.medical@example.invalid"
	sampleHeadSchool = "medical-and-health-sciences"
	sampleTrainer    = "trainer@example.invalid"
)

var sampleUnits = []struct{ code, title string }{
	{"ICT 101", "(Sample) Computer basics"},
	{"ICT 102", "(Sample) Office applications"},
	{"ICT 103", "(Sample) Networking basics"},
}

func seedAcademics(ctx context.Context, tx pgx.Tx, q *dbq.Queries, hash string, sum *Summary) error {
	one := func(sql string, dst any, args ...any) error {
		if err := tx.QueryRow(ctx, sql, args...).Scan(dst); err != nil {
			return fmt.Errorf("%s: %w", sql, err)
		}
		return nil
	}
	exec := func(sql string, args ...any) error {
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return fmt.Errorf("%s: %w", sql, err)
		}
		return nil
	}

	if err := exec(`INSERT INTO school_assignments (user_id, school_id)
		SELECT u.id, s.id FROM users u, schools s WHERE u.email = $1 AND s.slug = $2
		ON CONFLICT DO NOTHING`, sampleHeadEmail, sampleHeadSchool); err != nil {
		return err
	}

	// A term running now: from a month ago to two months ahead.
	now := time.Now()
	starts := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	ends := starts.AddDate(0, 3, 0).AddDate(0, 0, -1)
	var term uuid.UUID
	if err := one(`INSERT INTO academic_terms (name, starts_on, ends_on, registration_opens_on, registration_closes_on, is_sample)
		VALUES ($1, $2, $3, current_date - 7, current_date + 14, true)
		ON CONFLICT (name) DO UPDATE SET starts_on = EXCLUDED.starts_on, ends_on = EXCLUDED.ends_on,
		  registration_opens_on = EXCLUDED.registration_opens_on, registration_closes_on = EXCLUDED.registration_closes_on
		RETURNING id`, &term, sampleTermName, starts, ends); err != nil {
		return err
	}

	var programme, school, intake uuid.UUID
	var programmeTitle, intakeName string
	if err := tx.QueryRow(ctx, `SELECT p.id, p.school_id, p.title, i.id, i.name FROM programmes p
		JOIN programme_intakes i ON i.programme_id = p.id AND i.status = 'open'
		WHERE p.slug = $1 ORDER BY i.starts_on LIMIT 1`, sampleProgramme).Scan(&programme, &school, &programmeTitle, &intake, &intakeName); err != nil {
		return fmt.Errorf("sample academics programme: %w", err)
	}

	var units []uuid.UUID
	for i, u := range sampleUnits {
		var id uuid.UUID
		if err := one(`INSERT INTO units (school_id, code, title, description, hours, is_sample)
			VALUES ($1, $2, $3, '(Sample) Placeholder unit for development.', 40, true)
			ON CONFLICT (code) DO UPDATE SET title = EXCLUDED.title RETURNING id`, &id, school, u.code, u.title); err != nil {
			return err
		}
		if err := exec(`INSERT INTO programme_units (programme_id, unit_id, stage, is_core, sort_order) VALUES ($1, $2, 1, $3, $4)
			ON CONFLICT (programme_id, unit_id) DO NOTHING`, programme, id, i < 2, i); err != nil {
			return err
		}
		units = append(units, id)
	}

	// Offerings of the first two units, with a weekly class each.
	var trainer uuid.UUID
	if err := one(`SELECT id FROM users WHERE email = $1`, &trainer, sampleTrainer); err != nil {
		return err
	}
	var offerings []uuid.UUID
	for i, unit := range units[:2] {
		var id uuid.UUID
		if err := one(`INSERT INTO unit_offerings (term_id, unit_id) VALUES ($1, $2)
			ON CONFLICT (term_id, unit_id) DO UPDATE SET unit_id = EXCLUDED.unit_id RETURNING id`, &id, term, unit); err != nil {
			return err
		}
		if err := exec(`DELETE FROM timetable_entries WHERE offering_id = $1`, id); err != nil {
			return err
		}
		if err := exec(`INSERT INTO timetable_entries (offering_id, kind, weekday, starts_at, ends_at, room)
			VALUES ($1, 'class', $2, '08:30', '10:30', '(Sample) Room 1')`, id, i+1); err != nil {
			return err
		}
		offerings = append(offerings, id)
	}
	if err := exec(`INSERT INTO trainer_assignments (offering_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, offerings[0], trainer); err != nil {
		return err
	}

	// A notice to everyone, so the portal has one to show.
	if err := exec(`DELETE FROM notices WHERE title = '(Sample) Welcome to the new term'`); err != nil {
		return err
	}
	if err := exec(`INSERT INTO notices (audience, title, body, created_by)
		SELECT 'all', '(Sample) Welcome to the new term', '(Sample) A placeholder notice for development.', id
		FROM users WHERE email = 'registrar@example.invalid'`); err != nil {
		return err
	}

	cohort, err := q.CohortForIntake(ctx, dbq.CohortForIntakeParams{ProgrammeID: programme, IntakeID: &intake,
		Name: programmeTitle + " · " + intakeName})
	if err != nil {
		return err
	}

	// Test students, enrolled the way the workflow would have: the
	// application walks through each status (the trigger allows no shortcut).
	for n := 1; n <= sampleStudents; n++ {
		email := fmt.Sprintf("student%03d@example.invalid", n)
		number := fmt.Sprintf("DTC/SAMPLE/%04d", n)
		var exists bool
		if err := one(`SELECT EXISTS (SELECT 1 FROM students WHERE student_number = $1)`, &exists, number); err != nil {
			return err
		}
		if !exists {
			var user, applicant, application, student uuid.UUID
			if err := one(`INSERT INTO users (email, full_name, kind) VALUES ($1, $2, 'student')
				ON CONFLICT (email) DO UPDATE SET full_name = EXCLUDED.full_name RETURNING id`, &user, email, fmt.Sprintf("Test Student %03d", n)); err != nil {
				return err
			}
			if err := exec(`INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE key = 'student' ON CONFLICT DO NOTHING`, user); err != nil {
				return err
			}
			if err := one(`INSERT INTO applicants (user_id, email, first_name, last_name, phone) VALUES ($1, $2, 'Test', $3, $4) RETURNING id`,
				&applicant, user, email, fmt.Sprintf("Student %03d", n), fmt.Sprintf("+254 700 000 %03d", 200+n)); err != nil {
				return err
			}
			if err := one(`INSERT INTO applications (reference, applicant_id, programme_id, intake_id) VALUES ($1, $2, $3, $4) RETURNING id`,
				&application, fmt.Sprintf("DTC-APP-SAMPLE-%06d", 100+n), applicant, programme, intake); err != nil {
				return err
			}
			if err := exec(`UPDATE applications SET status = 'submitted', submitted_at = now(), declaration_accepted_at = now(),
				submitted_snapshot = '{"sample": true}' WHERE id = $1`, application); err != nil {
				return err
			}
			for _, st := range []string{"under_review", "offered", "offer_accepted", "enrolled"} {
				if err := exec(`UPDATE applications SET status = $2 WHERE id = $1`, application, st); err != nil {
					return err
				}
			}
			if err := exec(`INSERT INTO application_status_history (application_id, from_status, to_status, reason)
				VALUES ($1, NULL, 'enrolled', '(Sample) Enrolled by the dev seed')`, application); err != nil {
				return err
			}
			if err := one(`INSERT INTO students (student_number, user_id, applicant_id, application_id, programme_id, intake_id, cohort_id,
				first_name, last_name, email, phone, enrolled_by)
				VALUES ($1, $2, $3, $4, $5, $6, $7, 'Test', $8, $9, $10, $2) RETURNING id`,
				&student, number, user, applicant, application, programme, intake, cohort, fmt.Sprintf("Student %03d", n), email,
				fmt.Sprintf("+254 700 000 %03d", 200+n)); err != nil {
				return err
			}
			if err := exec(`INSERT INTO student_status_history (student_id, to_status, reason) VALUES ($1, 'active', '(Sample) Enrolled by the dev seed')`, student); err != nil {
				return err
			}
			if err := exec(`INSERT INTO invoices (student_id, intake_id, note, created_by) VALUES ($1, $2, '(Sample) Draft first invoice', $3)`,
				student, intake, user); err != nil {
				return err
			}
		}
		// Every test student signs in to the portal with SEED_PASSWORD.
		if err := exec(`UPDATE users SET password_hash = $2, password_changed_at = now() WHERE email = $1`, email, hash); err != nil {
			return err
		}
		for _, o := range offerings {
			if err := exec(`INSERT INTO unit_registrations (offering_id, student_id)
				SELECT $1, id FROM students WHERE student_number = $2 ON CONFLICT DO NOTHING`, o, number); err != nil {
				return err
			}
		}
		sum.Students++
	}
	return nil
}
