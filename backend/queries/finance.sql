-- Finance (Stage 14a). Balances are always computed here from records,
-- never stored (NN-03, AC-10).

-- ---------- students ----------

-- name: FinanceStudent :one
SELECT s.id, s.student_number, s.first_name, s.middle_name, s.last_name, s.email, s.status,
       s.programme_id, s.intake_id, s.cohort_id, p.title AS programme_title, pi.name AS intake_name
FROM students s
JOIN programmes p ON p.id = s.programme_id
LEFT JOIN programme_intakes pi ON pi.id = s.intake_id
WHERE s.id = @id;

-- name: FinanceStudentByUser :one
SELECT s.id, s.student_number, s.first_name, s.middle_name, s.last_name, s.email, s.status,
       s.programme_id, s.intake_id, s.cohort_id, p.title AS programme_title, pi.name AS intake_name
FROM students s
JOIN programmes p ON p.id = s.programme_id
LEFT JOIN programme_intakes pi ON pi.id = s.intake_id
WHERE s.user_id = @user_id;

-- name: FinanceCohort :one
SELECT id, programme_id, intake_id, name FROM cohorts WHERE id = @id;

-- name: FinanceTerm :one
SELECT id, name, starts_on, ends_on FROM academic_terms WHERE id = @id;

-- name: CohortStudentsForBilling :many
-- Active students of a cohort, and whether they already have a live
-- invoice for the term.
SELECT s.id, s.programme_id, s.intake_id,
       EXISTS (SELECT 1 FROM invoices i WHERE i.student_id = s.id AND i.kind = 'term' AND i.term_id = @term_id AND i.status <> 'void')::bool AS billed
FROM students s
WHERE s.cohort_id = @cohort_id AND s.status = 'active'
ORDER BY s.student_number;

-- ---------- fee structures ----------

-- name: ListFeeStructures :many
SELECT fs.id, fs.programme_id, p.title AS programme_title, fs.intake_id, pi.name AS intake_name, pi.starts_on AS intake_starts_on,
       fs.is_sample, fs.updated_at,
       (SELECT coalesce(sum(l.amount_cents), 0) FROM fee_structure_lines l WHERE l.structure_id = fs.id AND l.charge = 'once')::bigint AS once_cents,
       (SELECT coalesce(sum(l.amount_cents), 0) FROM fee_structure_lines l WHERE l.structure_id = fs.id AND l.charge = 'per_term')::bigint AS per_term_cents
FROM fee_structures fs
JOIN programmes p ON p.id = fs.programme_id
LEFT JOIN programme_intakes pi ON pi.id = fs.intake_id
ORDER BY p.title, pi.starts_on NULLS FIRST;

-- name: GetFeeStructure :one
SELECT fs.id, fs.programme_id, p.title AS programme_title, fs.intake_id, pi.name AS intake_name,
       fs.note, fs.is_sample, fs.updated_at, u.full_name AS updated_by_name
FROM fee_structures fs
JOIN programmes p ON p.id = fs.programme_id
LEFT JOIN programme_intakes pi ON pi.id = fs.intake_id
LEFT JOIN users u ON u.id = fs.updated_by
WHERE fs.id = @id;

-- name: LockFeeStructure :one
SELECT id FROM fee_structures WHERE id = @id FOR UPDATE;

-- name: FeeStructureFor :one
-- The structure for a programme and intake: the intake's own, else the
-- programme's structure for every intake.
SELECT id FROM fee_structures
WHERE programme_id = @programme_id AND (intake_id = sqlc.narg(intake_id)::uuid OR intake_id IS NULL)
ORDER BY intake_id IS NULL
LIMIT 1;

-- name: FeeStructureLines :many
SELECT id, label, category, charge, amount_cents, sort_order
FROM fee_structure_lines WHERE structure_id = @structure_id ORDER BY sort_order, id;

-- name: FeeStructureInstalments :many
SELECT seq, percent, due_days FROM fee_structure_instalments WHERE structure_id = @structure_id ORDER BY seq;

-- name: ProgrammeIntakeValid :one
-- Whether the programme exists and the intake (if any) belongs to it.
SELECT (sqlc.narg(intake_id)::uuid IS NULL
        OR EXISTS (SELECT 1 FROM programme_intakes pi WHERE pi.id = sqlc.narg(intake_id)::uuid AND pi.programme_id = p.id))::bool AS intake_ok,
       p.is_sample
FROM programmes p WHERE p.id = @programme_id;

-- name: InsertFeeStructure :one
INSERT INTO fee_structures (programme_id, intake_id, note, is_sample, updated_by)
VALUES (@programme_id, sqlc.narg(intake_id), @note, @is_sample, @updated_by)
RETURNING id;

-- name: UpdateFeeStructure :exec
UPDATE fee_structures SET note = @note, updated_by = @updated_by WHERE id = @id;

-- name: DeleteFeeStructure :execrows
DELETE FROM fee_structures WHERE id = @id;

-- name: DeleteFeeStructureLines :exec
DELETE FROM fee_structure_lines WHERE structure_id = @structure_id;

-- name: DeleteFeeStructureInstalments :exec
DELETE FROM fee_structure_instalments WHERE structure_id = @structure_id;

-- name: InsertFeeStructureLine :exec
INSERT INTO fee_structure_lines (structure_id, label, category, charge, amount_cents, sort_order)
VALUES (@structure_id, @label, @category, @charge, @amount_cents, @sort_order);

-- name: InsertFeeStructureInstalment :exec
INSERT INTO fee_structure_instalments (structure_id, seq, percent, due_days)
VALUES (@structure_id, @seq, @percent, @due_days);

-- name: PublishedFeeLines :many
-- The programme's fee lines on the website, to start a structure from.
SELECT label, category, amount_cents, frequency
FROM fee_items
WHERE programme_id = @programme_id AND (intake_id IS NULL OR intake_id = sqlc.narg(intake_id)::uuid)
ORDER BY sort_order, label;

-- ---------- invoices ----------

-- name: LockInvoice :one
SELECT id, student_id, intake_id, kind, term_id, status, number FROM invoices WHERE id = @id FOR UPDATE;

-- name: GetInvoice :one
SELECT i.id, i.student_id, i.kind, i.term_id, i.status, i.number, i.note, i.void_reason, i.created_at, i.issued_at,
       t.name AS term_name, s.student_number, s.first_name, s.middle_name, s.last_name, p.title AS programme_title,
       cu.full_name AS created_by_name, iu.full_name AS issued_by_name,
       (SELECT coalesce(sum(l.amount_cents), 0) FROM invoice_lines l WHERE l.invoice_id = i.id)::bigint AS total_cents
FROM invoices i
JOIN students s ON s.id = i.student_id
JOIN programmes p ON p.id = s.programme_id
LEFT JOIN academic_terms t ON t.id = i.term_id
JOIN users cu ON cu.id = i.created_by
LEFT JOIN users iu ON iu.id = i.issued_by
WHERE i.id = @id;

-- name: ListInvoices :many
SELECT i.id, i.student_id, i.kind, i.status, i.number, i.created_at, i.issued_at, t.name AS term_name,
       s.student_number, s.first_name, s.middle_name, s.last_name,
       (SELECT coalesce(sum(l.amount_cents), 0) FROM invoice_lines l WHERE l.invoice_id = i.id)::bigint AS total_cents,
       count(*) OVER ()::bigint AS total_count
FROM invoices i
JOIN students s ON s.id = i.student_id
LEFT JOIN academic_terms t ON t.id = i.term_id
WHERE (sqlc.narg(status)::text IS NULL OR i.status = sqlc.narg(status)::text)
  AND (sqlc.narg(term_id)::uuid IS NULL OR i.term_id = sqlc.narg(term_id)::uuid)
  AND (sqlc.narg(cohort_id)::uuid IS NULL OR s.cohort_id = sqlc.narg(cohort_id)::uuid)
  AND (sqlc.narg(q)::text IS NULL
       OR i.number ILIKE '%' || sqlc.narg(q)::text || '%'
       OR s.student_number ILIKE '%' || sqlc.narg(q)::text || '%'
       OR (s.first_name || ' ' || s.last_name) ILIKE '%' || sqlc.narg(q)::text || '%')
ORDER BY i.created_at DESC, i.id
LIMIT @lim OFFSET @off;

-- name: StudentInvoices :many
SELECT i.id, i.kind, i.status, i.number, i.created_at, i.issued_at, t.name AS term_name,
       (SELECT coalesce(sum(l.amount_cents), 0) FROM invoice_lines l WHERE l.invoice_id = i.id)::bigint AS total_cents
FROM invoices i
LEFT JOIN academic_terms t ON t.id = i.term_id
WHERE i.student_id = @student_id AND (NOT @issued_only::bool OR i.status = 'issued')
ORDER BY i.created_at, i.id;

-- name: InvoiceLines :many
SELECT id, label, category, amount_cents FROM invoice_lines WHERE invoice_id = @invoice_id ORDER BY sort_order, id;

-- name: InvoiceInstalments :many
SELECT seq, percent, due_days, amount_cents, due_on FROM invoice_instalments WHERE invoice_id = @invoice_id ORDER BY seq;

-- name: InsertInvoice :one
INSERT INTO invoices (student_id, intake_id, kind, term_id, note, created_by)
VALUES (@student_id, sqlc.narg(intake_id), @kind, sqlc.narg(term_id), @note, @created_by)
RETURNING id;

-- name: UpdateInvoiceNote :exec
UPDATE invoices SET note = @note WHERE id = @id;

-- name: DeleteInvoiceLines :exec
DELETE FROM invoice_lines WHERE invoice_id = @invoice_id;

-- name: DeleteInvoiceInstalments :exec
DELETE FROM invoice_instalments WHERE invoice_id = @invoice_id;

-- name: InsertInvoiceLine :exec
INSERT INTO invoice_lines (invoice_id, label, category, amount_cents, sort_order)
VALUES (@invoice_id, @label, @category, @amount_cents, @sort_order);

-- name: InsertInvoiceInstalment :exec
INSERT INTO invoice_instalments (invoice_id, seq, percent, due_days)
VALUES (@invoice_id, @seq, @percent, @due_days);

-- name: FixInvoiceInstalment :exec
UPDATE invoice_instalments SET amount_cents = @amount_cents, due_on = @due_on
WHERE invoice_id = @invoice_id AND seq = @seq;

-- name: IssueInvoice :execrows
UPDATE invoices SET status = 'issued', number = @number, issued_at = now(), issued_by = @issued_by
WHERE id = @id AND status = 'draft';

-- name: VoidInvoice :execrows
UPDATE invoices SET status = 'void', void_reason = @void_reason WHERE id = @id AND status = 'draft';

-- ---------- payments ----------

-- name: MpesaReferenceUsed :one
SELECT EXISTS (SELECT 1 FROM payments WHERE method = 'mpesa' AND reference = @reference AND status <> 'rejected')::bool;

-- name: InsertPayment :one
INSERT INTO payments (student_id, method, amount_cents, paid_on, reference, payer_name, note, recorded_by)
VALUES (@student_id, @method, @amount_cents, @paid_on, @reference, @payer_name, @note, @recorded_by)
RETURNING id;

-- name: LockPayment :one
SELECT id, student_id, method, amount_cents, status, recorded_by FROM payments WHERE id = @id FOR UPDATE;

-- name: DecidePayment :execrows
UPDATE payments
SET status = @status, confirmed_via = sqlc.narg(confirmed_via), decided_by = @decided_by, decided_at = now(), decision_note = @decision_note
WHERE id = @id AND status = 'pending';

-- name: InsertReceipt :one
INSERT INTO receipts (number, payment_id, issued_by) VALUES (@number, @payment_id, sqlc.narg(issued_by))
RETURNING id;

-- name: GetPayment :one
SELECT pm.id, pm.student_id, pm.method, pm.amount_cents, pm.paid_on, pm.reference, pm.payer_name, pm.note, pm.status,
       pm.confirmed_via, pm.recorded_at, pm.decided_at, pm.decision_note,
       ru.full_name AS recorded_by_name, du.full_name AS decided_by_name,
       r.id AS receipt_id, r.number AS receipt_number, r.issued_at AS receipt_issued_at,
       s.student_number, s.first_name, s.middle_name, s.last_name, p.title AS programme_title
FROM payments pm
JOIN students s ON s.id = pm.student_id
JOIN programmes p ON p.id = s.programme_id
JOIN users ru ON ru.id = pm.recorded_by
LEFT JOIN users du ON du.id = pm.decided_by
LEFT JOIN receipts r ON r.payment_id = pm.id
WHERE pm.id = @id;

-- name: ListPayments :many
SELECT pm.id, pm.student_id, pm.method, pm.amount_cents, pm.paid_on, pm.reference, pm.payer_name, pm.status, pm.recorded_at,
       ru.full_name AS recorded_by_name, r.number AS receipt_number,
       s.student_number, s.first_name, s.middle_name, s.last_name,
       count(*) OVER ()::bigint AS total_count
FROM payments pm
JOIN students s ON s.id = pm.student_id
JOIN users ru ON ru.id = pm.recorded_by
LEFT JOIN receipts r ON r.payment_id = pm.id
WHERE (sqlc.narg(status)::text IS NULL OR pm.status = sqlc.narg(status)::text)
  AND (sqlc.narg(method)::text IS NULL OR pm.method = sqlc.narg(method)::text)
  AND (sqlc.narg(q)::text IS NULL
       OR pm.reference ILIKE '%' || sqlc.narg(q)::text || '%'
       OR r.number ILIKE '%' || sqlc.narg(q)::text || '%'
       OR s.student_number ILIKE '%' || sqlc.narg(q)::text || '%'
       OR (s.first_name || ' ' || s.last_name) ILIKE '%' || sqlc.narg(q)::text || '%')
ORDER BY pm.recorded_at DESC, pm.id
LIMIT @lim OFFSET @off;

-- name: StudentPayments :many
SELECT pm.id, pm.method, pm.amount_cents, pm.paid_on, pm.reference, pm.payer_name, pm.status, pm.recorded_at, pm.decided_at,
       pm.decision_note, r.id AS receipt_id, r.number AS receipt_number
FROM payments pm
LEFT JOIN receipts r ON r.payment_id = pm.id
WHERE pm.student_id = @student_id
ORDER BY pm.paid_on, pm.recorded_at, pm.id;

-- ---------- adjustments ----------

-- name: InsertAdjustment :one
INSERT INTO adjustments (student_id, kind, amount_cents, reason, invoice_id, payment_id, created_by)
VALUES (@student_id, @kind, @amount_cents, @reason, sqlc.narg(invoice_id), sqlc.narg(payment_id), @created_by)
RETURNING id;

-- name: StudentAdjustments :many
SELECT a.id, a.kind, a.amount_cents, a.reason, a.invoice_id, a.payment_id, a.created_at, u.full_name AS created_by_name,
       i.number AS invoice_number, r.number AS receipt_number
FROM adjustments a
JOIN users u ON u.id = a.created_by
LEFT JOIN invoices i ON i.id = a.invoice_id
LEFT JOIN receipts r ON r.payment_id = a.payment_id
WHERE a.student_id = @student_id
ORDER BY a.created_at, a.id;

-- ---------- balances and reports ----------

-- name: StudentBalance :one
-- The four sums a balance is made of, and what waits for confirmation.
SELECT
  (SELECT coalesce(sum(l.amount_cents), 0) FROM invoice_lines l JOIN invoices i ON i.id = l.invoice_id
     WHERE i.student_id = @student_id AND i.status = 'issued')::bigint AS invoiced_cents,
  (SELECT coalesce(sum(a.amount_cents), 0) FROM adjustments a WHERE a.student_id = @student_id AND a.kind = 'charge')::bigint AS charges_cents,
  (SELECT coalesce(sum(pm.amount_cents), 0) FROM payments pm WHERE pm.student_id = @student_id AND pm.status = 'confirmed')::bigint AS paid_cents,
  (SELECT coalesce(sum(a.amount_cents), 0) FROM adjustments a WHERE a.student_id = @student_id AND a.kind = 'credit')::bigint AS credits_cents,
  (SELECT coalesce(sum(pm.amount_cents), 0) FROM payments pm WHERE pm.student_id = @student_id AND pm.status = 'pending')::bigint AS pending_cents;

-- name: StudentDueInstalments :many
-- Instalments of issued invoices, earliest first, for what is due and when.
SELECT n.due_on, n.amount_cents, i.number
FROM invoice_instalments n
JOIN invoices i ON i.id = n.invoice_id
WHERE i.student_id = @student_id AND i.status = 'issued'
ORDER BY n.due_on, i.issued_at, n.seq;

-- name: ArrearsReport :many
-- Students who owe money, with what was due by today and is unpaid.
WITH sums AS (
  SELECT s.id,
    (SELECT coalesce(sum(l.amount_cents), 0) FROM invoice_lines l JOIN invoices i ON i.id = l.invoice_id
       WHERE i.student_id = s.id AND i.status = 'issued')::bigint AS invoiced,
    (SELECT coalesce(sum(n.amount_cents), 0) FROM invoice_instalments n JOIN invoices i ON i.id = n.invoice_id
       WHERE i.student_id = s.id AND i.status = 'issued' AND n.due_on <= @today::date)::bigint AS due,
    (SELECT coalesce(sum(a.amount_cents), 0) FROM adjustments a WHERE a.student_id = s.id AND a.kind = 'charge')::bigint AS charges,
    (SELECT coalesce(sum(pm.amount_cents), 0) FROM payments pm WHERE pm.student_id = s.id AND pm.status = 'confirmed')::bigint AS paid,
    (SELECT coalesce(sum(a.amount_cents), 0) FROM adjustments a WHERE a.student_id = s.id AND a.kind = 'credit')::bigint AS credits
  FROM students s
)
SELECT s.id, s.student_number, s.first_name, s.middle_name, s.last_name, s.status, p.title AS programme_title, c.name AS cohort_name,
       (x.invoiced + x.charges - x.paid - x.credits)::bigint AS balance_cents,
       greatest(x.due + x.charges - x.paid - x.credits, 0)::bigint AS overdue_cents
FROM sums x
JOIN students s ON s.id = x.id
JOIN programmes p ON p.id = s.programme_id
LEFT JOIN cohorts c ON c.id = s.cohort_id
WHERE x.invoiced + x.charges - x.paid - x.credits > 0
  AND (sqlc.narg(cohort_id)::uuid IS NULL OR s.cohort_id = sqlc.narg(cohort_id)::uuid)
  AND (NOT @overdue_only::bool OR x.due + x.charges - x.paid - x.credits > 0)
ORDER BY overdue_cents DESC, balance_cents DESC, s.student_number;

-- name: FinanceSummary :one
SELECT
  (SELECT count(*) FROM invoices WHERE status = 'draft')::bigint AS draft_invoices,
  (SELECT count(*) FROM payments WHERE status = 'pending')::bigint AS pending_payments,
  (SELECT coalesce(sum(amount_cents), 0) FROM payments WHERE status = 'pending')::bigint AS pending_cents,
  ((SELECT coalesce(sum(l.amount_cents), 0) FROM invoice_lines l JOIN invoices i ON i.id = l.invoice_id WHERE i.status = 'issued')
   + (SELECT coalesce(sum(amount_cents), 0) FROM adjustments WHERE kind = 'charge')
   - (SELECT coalesce(sum(amount_cents), 0) FROM payments WHERE status = 'confirmed')
   - (SELECT coalesce(sum(amount_cents), 0) FROM adjustments WHERE kind = 'credit'))::bigint AS net_outstanding_cents;

-- name: FinanceProgrammes :many
-- Programmes and their intakes, for choosing what a fee structure is for.
SELECT p.id, p.title, p.is_sample, pi.id AS intake_id, pi.name AS intake_name, pi.starts_on AS intake_starts_on
FROM programmes p
LEFT JOIN programme_intakes pi ON pi.programme_id = p.id
ORDER BY p.title, pi.starts_on;
