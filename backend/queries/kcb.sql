-- KCB Buni IPN (Stage 15b).

-- name: InsertKcbNotification :one
INSERT INTO kcb_notifications (remote_ip, signature, body) VALUES (@remote_ip, @signature, @body) RETURNING id;

-- name: InsertKcbTransaction :one
-- Nothing is returned when the reference was recorded before (a repeat).
INSERT INTO kcb_transactions (transaction_reference, request_id, channel_code, sent_at, sent_at_raw, amount_cents, currency,
  customer_reference, customer_name, customer_mobile, narration, credit_account, org_short_code, till_number, notification_id,
  verified, status)
VALUES (@transaction_reference, @request_id, @channel_code, @sent_at, @sent_at_raw, @amount_cents, @currency,
  @customer_reference, @customer_name, @customer_mobile, @narration, @credit_account, @org_short_code, @till_number, @notification_id,
  @verified, 'unmatched')
ON CONFLICT (transaction_reference) DO NOTHING
RETURNING id;

-- name: SettleKcbTransaction :exec
UPDATE kcb_transactions
SET status = @status, reason = @reason, student_id = sqlc.narg(student_id), payment_id = sqlc.narg(payment_id),
    handled_by = sqlc.narg(handled_by), handled_at = CASE WHEN sqlc.narg(handled_by)::uuid IS NULL THEN NULL ELSE now() END,
    note = @note
WHERE id = @id AND status = 'unmatched';

-- name: LockKcbTransaction :one
SELECT id, transaction_reference, sent_at, amount_cents, customer_name, status FROM kcb_transactions WHERE id = @id FOR UPDATE;

-- name: BankPaymentByReference :one
SELECT id FROM payments WHERE method = 'bank' AND reference = @reference AND status <> 'rejected' LIMIT 1;

-- name: InsertBankNotificationPayment :one
INSERT INTO payments (student_id, method, amount_cents, paid_on, reference, payer_name, note, recorded_by, source)
VALUES (@student_id, 'bank', @amount_cents, @paid_on, @reference, @payer_name, @note, sqlc.narg(recorded_by), 'kcb')
RETURNING id;

-- name: ListKcbTransactions :many
SELECT t.id, t.transaction_reference, t.channel_code, t.sent_at, t.amount_cents, t.customer_reference, t.customer_mobile, t.customer_name,
       t.narration, t.verified, t.status, t.reason, t.note, t.student_id, t.payment_id, t.handled_at,
       s.student_number, s.first_name, s.middle_name, s.last_name, u.full_name AS handled_by_name,
       p.status AS payment_status, r.number AS receipt_number,
       count(*) OVER ()::bigint AS total_count
FROM kcb_transactions t
LEFT JOIN students s ON s.id = t.student_id
LEFT JOIN users u ON u.id = t.handled_by
LEFT JOIN payments p ON p.id = t.payment_id
LEFT JOIN receipts r ON r.payment_id = t.payment_id
WHERE (sqlc.narg(status)::text IS NULL OR t.status = sqlc.narg(status)::text)
  AND (sqlc.narg(day)::date IS NULL OR (t.sent_at AT TIME ZONE 'Africa/Nairobi')::date = sqlc.narg(day)::date)
  AND (sqlc.narg(q)::text IS NULL
       OR t.transaction_reference ILIKE '%' || sqlc.narg(q)::text || '%'
       OR t.customer_reference ILIKE '%' || sqlc.narg(q)::text || '%'
       OR t.customer_name ILIKE '%' || sqlc.narg(q)::text || '%')
ORDER BY t.sent_at DESC, t.id
LIMIT @lim OFFSET @off;

-- name: KcbDaySummary :many
SELECT status, count(*)::bigint AS n, coalesce(sum(amount_cents), 0)::bigint AS total_cents
FROM kcb_transactions
WHERE (sent_at AT TIME ZONE 'Africa/Nairobi')::date = @day::date
GROUP BY status
ORDER BY status;

-- name: KcbActivity :one
SELECT coalesce((SELECT extract(epoch FROM max(received_at)) FROM kcb_notifications), 0)::bigint AS last_notification_epoch,
       (SELECT count(*) FROM kcb_transactions WHERE status = 'unmatched')::bigint AS unmatched;

-- name: UnprocessedKcbNotifications :many
SELECT n.id, n.body FROM kcb_notifications n
WHERE NOT EXISTS (SELECT 1 FROM kcb_transactions t WHERE t.notification_id = n.id)
  AND NOT EXISTS (SELECT 1 FROM kcb_transactions t WHERE t.transaction_reference = n.body->>'transactionReference')
ORDER BY n.id
LIMIT 500;
