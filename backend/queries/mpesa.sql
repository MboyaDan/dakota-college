-- M-Pesa C2B (Stage 15a).

-- name: InsertMpesaCallback :one
INSERT INTO mpesa_callbacks (kind, remote_ip, body) VALUES (@kind, @remote_ip, @body) RETURNING id;

-- name: InsertMpesaTransaction :one
-- Nothing is returned when the code was recorded before (a repeated callback).
INSERT INTO mpesa_transactions (trans_id, trans_type, trans_time, amount_cents, short_code, bill_ref, msisdn, payer_name, callback_id, status, reason)
VALUES (@trans_id, @trans_type, @trans_time, @amount_cents, @short_code, @bill_ref, @msisdn, @payer_name, @callback_id, 'unmatched', '')
ON CONFLICT (trans_id) DO NOTHING
RETURNING id;

-- name: SettleMpesaTransaction :exec
-- Sets the outcome of a transaction that is still unmatched.
UPDATE mpesa_transactions
SET status = @status, reason = @reason, student_id = sqlc.narg(student_id), payment_id = sqlc.narg(payment_id),
    handled_by = sqlc.narg(handled_by), handled_at = CASE WHEN sqlc.narg(handled_by)::uuid IS NULL THEN NULL ELSE now() END,
    note = @note
WHERE id = @id AND status = 'unmatched';

-- name: LockMpesaTransaction :one
SELECT id, trans_id, trans_time, amount_cents, bill_ref, payer_name, status FROM mpesa_transactions WHERE id = @id FOR UPDATE;

-- name: StudentsByAccountKey :many
-- Students whose number, without punctuation and in capitals, is key.
SELECT id FROM students WHERE upper(regexp_replace(student_number, '[^A-Za-z0-9]', '', 'g')) = @key::text;

-- name: MpesaPaymentByReference :one
SELECT id, student_id, amount_cents, status FROM payments
WHERE method = 'mpesa' AND reference = @reference AND status <> 'rejected'
FOR UPDATE;

-- name: InsertProviderPayment :one
INSERT INTO payments (student_id, method, amount_cents, paid_on, reference, payer_name, note, recorded_by)
VALUES (@student_id, 'mpesa', @amount_cents, @paid_on, @reference, @payer_name, @note, sqlc.narg(recorded_by))
RETURNING id;

-- name: ListMpesaTransactions :many
SELECT t.id, t.trans_id, t.trans_type, t.trans_time, t.amount_cents, t.bill_ref, t.msisdn, t.payer_name, t.status, t.reason, t.note,
       t.student_id, t.payment_id, t.handled_at, s.student_number, s.first_name, s.middle_name, s.last_name, u.full_name AS handled_by_name,
       r.number AS receipt_number,
       count(*) OVER ()::bigint AS total_count
FROM mpesa_transactions t
LEFT JOIN students s ON s.id = t.student_id
LEFT JOIN users u ON u.id = t.handled_by
LEFT JOIN receipts r ON r.payment_id = t.payment_id
WHERE (sqlc.narg(status)::text IS NULL OR t.status = sqlc.narg(status)::text)
  AND (sqlc.narg(day)::date IS NULL OR (t.trans_time AT TIME ZONE 'Africa/Nairobi')::date = sqlc.narg(day)::date)
  AND (sqlc.narg(q)::text IS NULL
       OR t.trans_id ILIKE '%' || sqlc.narg(q)::text || '%'
       OR t.bill_ref ILIKE '%' || sqlc.narg(q)::text || '%'
       OR t.payer_name ILIKE '%' || sqlc.narg(q)::text || '%')
ORDER BY t.trans_time DESC, t.id
LIMIT @lim OFFSET @off;

-- name: MpesaDaySummary :many
-- What arrived on one day (Nairobi), by outcome.
SELECT status, count(*)::bigint AS n, coalesce(sum(amount_cents), 0)::bigint AS total_cents
FROM mpesa_transactions
WHERE (trans_time AT TIME ZONE 'Africa/Nairobi')::date = @day::date
GROUP BY status
ORDER BY status;

-- name: MpesaManualPaymentsOn :one
-- M-Pesa payments recorded by hand for one day, which the statement should also show.
SELECT count(*)::bigint AS n, coalesce(sum(amount_cents), 0)::bigint AS total_cents,
       count(*) FILTER (WHERE status = 'pending')::bigint AS pending
FROM payments
WHERE method = 'mpesa' AND paid_on = @day::date AND status <> 'rejected'
  AND NOT EXISTS (SELECT 1 FROM mpesa_transactions t WHERE t.payment_id = payments.id);

-- name: MpesaActivity :one
SELECT coalesce((SELECT extract(epoch FROM max(received_at)) FROM mpesa_callbacks), 0)::bigint AS last_callback_epoch,
       (SELECT count(*) FROM mpesa_transactions WHERE status = 'unmatched')::bigint AS unmatched;

-- name: UnprocessedMpesaCallbacks :many
-- Confirmations stored but never recorded as a transaction (processing failed).
SELECT c.id, c.body FROM mpesa_callbacks c
WHERE c.kind = 'confirmation'
  AND NOT EXISTS (SELECT 1 FROM mpesa_transactions t WHERE t.callback_id = c.id)
  AND NOT EXISTS (SELECT 1 FROM mpesa_transactions t WHERE t.trans_id = c.body->>'TransID')
ORDER BY c.id
LIMIT 500;
