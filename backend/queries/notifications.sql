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

-- name: InsertNotification :one
INSERT INTO notifications (template_key, channel, recipient, status, error, related_type, related_id)
VALUES (@template_key, @channel, @recipient, @status, @error, @related_type, @related_id)
RETURNING id;

-- name: MarkNotificationSent :exec
UPDATE notifications SET status = 'sent', sent_at = now(), error = '' WHERE id = @id;

-- name: MarkNotificationFailed :exec
UPDATE notifications SET status = 'failed', error = @error WHERE id = @id;
