package seed

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/finance"
)

// Sample finance (Stage 14a): a fee structure for the sample ICT diploma,
// copied from its sample published fees, half due on issue and half after
// 60 days; the test students' first invoices filled from it (still drafts,
// for Finance to issue); and one pending M-Pesa payment for Test Student
// 002 waiting for confirmation. Idempotent like the rest of the seed.

const sampleMpesaCode = "SAMPLE0001"

func seedFinance(ctx context.Context, tx pgx.Tx, q *dbq.Queries, programme uuid.UUID) error {
	exec := func(sql string, args ...any) error {
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return fmt.Errorf("%s: %w", sql, err)
		}
		return nil
	}
	var structure uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM fee_structures WHERE programme_id = $1 AND intake_id IS NULL`, programme).Scan(&structure)
	if err == pgx.ErrNoRows {
		if err := tx.QueryRow(ctx, `INSERT INTO fee_structures (programme_id, note, is_sample)
			VALUES ($1, '(Sample) Copied from the sample published fees for development.', true) RETURNING id`, programme).Scan(&structure); err != nil {
			return err
		}
		if err := exec(`INSERT INTO fee_structure_lines (structure_id, label, category, charge, amount_cents, sort_order)
			SELECT $1, label, category, CASE frequency WHEN 'per_term' THEN 'per_term' ELSE 'once' END, amount_cents, sort_order
			FROM fee_items WHERE programme_id = $2 AND intake_id IS NULL`, structure, programme); err != nil {
			return err
		}
		if err := exec(`INSERT INTO fee_structure_instalments (structure_id, seq, percent, due_days) VALUES ($1, 1, 50, 0), ($1, 2, 50, 60)`, structure); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// Fill the sample students' first invoices that are still empty drafts.
	rows, err := tx.Query(ctx, `SELECT i.id FROM invoices i JOIN students s ON s.id = i.student_id
		WHERE s.student_number LIKE 'DTC/SAMPLE/%' AND i.status = 'draft' AND i.kind = 'enrolment'
		  AND NOT EXISTS (SELECT 1 FROM invoice_lines l WHERE l.invoice_id = i.id)`)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := finance.FillFromStructure(ctx, q, id); err != nil {
			return err
		}
	}

	return exec(`INSERT INTO payments (student_id, method, amount_cents, paid_on, reference, note, recorded_by)
		SELECT s.id, 'mpesa', 1000000, current_date, $1, '(Sample) Waiting for confirmation against the M-Pesa statement.', u.id
		FROM students s, users u
		WHERE s.student_number = 'DTC/SAMPLE/0002' AND u.email = 'finance@example.invalid'
		  AND NOT EXISTS (SELECT 1 FROM payments WHERE reference = $1)`, sampleMpesaCode)
}
