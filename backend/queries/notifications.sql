-- ---------- notification templates and log (Stage 8b) ----------

-- name: GetNotificationTemplate :one
SELECT key, channel, subject, body FROM notification_templates WHERE key = @key AND channel = @channel;

-- name: ListNotificationTemplates :many
SELECT t.key, t.channel, t.name, t.description, t.subject, t.body, t.updated_at, u.full_name AS updated_by_name
FROM notification_templates t
LEFT JOIN users u ON u.id = t.updated_by
ORDER BY t.key, t.channel;

-- name: UpdateNotificationTemplate :execrows
UPDATE notification_templates SET subject = @subject, body = @body, updated_by = @updated_by
WHERE key = @key AND channel = @channel;

-- ---------- message queue (Stage 16a) ----------

-- name: InsertNotification :one
INSERT INTO notifications (template_key, system_key, channel, recipient, status, error, related_type, related_id, payload, next_attempt_at, expires_at)
VALUES (@template_key, @system_key, @channel, @recipient, @status, @error, @related_type, @related_id, @payload, @next_attempt_at, @expires_at)
RETURNING id;

-- name: ExpireNotifications :execrows
-- Messages not sent in time are given up; a claim still being worked on is
-- left to finish.
UPDATE notifications SET status = 'failed', error = 'Not sent before it expired', payload = NULL, locked_until = NULL
WHERE expires_at <= @now
  AND (status = 'queued' OR (status = 'sending' AND locked_until < @now));

-- name: EraseExpiredPayloads :execrows
UPDATE notifications SET payload = NULL
WHERE status = 'failed' AND payload IS NOT NULL AND expires_at <= @now;

-- name: ClaimNotifications :many
-- Takes due messages for one attempt. A claim whose worker stopped before
-- recording the result is taken again once its lock runs out.
UPDATE notifications SET status = 'sending', attempts = attempts + 1, locked_until = @locked_until::timestamptz, last_attempt_at = @now::timestamptz
WHERE id IN (
  SELECT n.id FROM notifications n
  WHERE n.payload IS NOT NULL AND n.expires_at > @now::timestamptz
    AND ((n.status = 'queued' AND n.next_attempt_at <= @now) OR (n.status = 'sending' AND n.locked_until < @now))
  ORDER BY n.next_attempt_at
  LIMIT @max_rows
  FOR UPDATE SKIP LOCKED
)
RETURNING id, channel, recipient, payload, attempts;

-- name: MarkNotificationSent :exec
UPDATE notifications SET status = 'sent', sent_at = @now::timestamptz, error = '', payload = NULL, locked_until = NULL
WHERE id = @id;

-- name: RetryNotificationLater :exec
UPDATE notifications SET status = 'queued', error = @error, next_attempt_at = @next_attempt_at, locked_until = NULL
WHERE id = @id;

-- name: MarkNotificationFailed :exec
UPDATE notifications SET status = 'failed', error = @error, locked_until = NULL
WHERE id = @id;

-- name: ResendNotification :one
-- Only a failed message whose text is still stored can go again.
UPDATE notifications SET status = 'queued', attempts = 0, error = '', next_attempt_at = @now, locked_until = NULL
WHERE id = @id AND status = 'failed' AND payload IS NOT NULL AND expires_at > @now::timestamptz
RETURNING id, channel, template_key, system_key;

-- name: ListNotifications :many
SELECT n.id, n.template_key, n.system_key, n.channel, n.recipient, n.status, n.error, n.attempts,
       n.related_type, n.related_id, n.created_at, n.sent_at, n.next_attempt_at, n.expires_at,
       (n.payload IS NOT NULL)::boolean AS has_payload, t.name AS template_name
FROM notifications n
LEFT JOIN notification_templates t ON t.key = n.template_key AND t.channel = n.channel
WHERE (sqlc.narg(status)::text IS NULL OR n.status = sqlc.narg(status)::text)
  AND (sqlc.narg(channel)::text IS NULL OR n.channel = sqlc.narg(channel)::text)
  AND (sqlc.narg(search)::text IS NULL OR n.recipient ILIKE '%' || sqlc.narg(search)::text || '%')
ORDER BY n.created_at DESC, n.id
LIMIT @max_rows OFFSET @skip_rows;

-- name: CountNotifications :one
SELECT count(*) FROM notifications n
WHERE (sqlc.narg(status)::text IS NULL OR n.status = sqlc.narg(status)::text)
  AND (sqlc.narg(channel)::text IS NULL OR n.channel = sqlc.narg(channel)::text)
  AND (sqlc.narg(search)::text IS NULL OR n.recipient ILIKE '%' || sqlc.narg(search)::text || '%');

-- name: CountNotificationsByStatus :one
SELECT count(*) FILTER (WHERE status IN ('queued', 'sending')) AS waiting,
       count(*) FILTER (WHERE status = 'failed' AND created_at >= @since) AS failed_recent
FROM notifications;
