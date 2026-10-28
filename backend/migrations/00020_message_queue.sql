-- Message queue with retries (Stage 16a, brief 10). Every email (and, after
-- DG-03, every text message) is a row in notifications that a worker sends,
-- retrying failures with growing waits, so a mail server outage no longer
-- loses messages.
--
-- The message itself is stored encrypted (AES-GCM, MESSAGE_KEY) only while
-- it may still be sent: it is erased once sent, and a failed message's copy
-- is erased when it expires. Some messages carry sign-in or password reset
-- links, so they expire with the link.
--
-- Staff-facing emails (password reset, enquiry routing and assignment) have
-- fixed text in code: they carry a system_key instead of a template_key.

-- +goose Up
ALTER TABLE notifications ALTER COLUMN template_key DROP NOT NULL;
ALTER TABLE notifications
  ADD COLUMN system_key       text CHECK (system_key ~ '^[a-z]+(\.[a-z_]+)+$'),
  ADD COLUMN payload          bytea,
  ADD COLUMN attempts         integer     NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  ADD COLUMN next_attempt_at  timestamptz NOT NULL DEFAULT now(),
  ADD COLUMN locked_until     timestamptz,
  ADD COLUMN last_attempt_at  timestamptz,
  ADD COLUMN expires_at       timestamptz NOT NULL DEFAULT now() + interval '7 days',
  ADD CONSTRAINT notifications_one_key CHECK ((template_key IS NULL) <> (system_key IS NULL)),
  -- A sent or skipped message keeps no copy of its text.
  ADD CONSTRAINT notifications_no_payload_when_done CHECK (status NOT IN ('sent', 'skipped') OR payload IS NULL);

ALTER TABLE notifications DROP CONSTRAINT notifications_status_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_status_check
  CHECK (status IN ('queued', 'sending', 'sent', 'failed', 'skipped'));

-- Rows queued before the queue existed have no stored text to send.
UPDATE notifications SET status = 'failed', error = 'Queued before Stage 16; not sent'
WHERE status = 'queued';

CREATE INDEX notifications_due_idx ON notifications (next_attempt_at) WHERE status IN ('queued', 'sending');
CREATE INDEX notifications_status_idx ON notifications (status, created_at DESC);

UPDATE permissions SET description = 'Edit the text of messages and see sent messages'
WHERE key = 'notifications.manage';

-- +goose Down
UPDATE permissions SET description = 'Edit the text of emails and text messages sent to applicants'
WHERE key = 'notifications.manage';
DROP INDEX notifications_status_idx;
DROP INDEX notifications_due_idx;
-- Removes the log of staff-facing messages (system_key rows); development only.
DELETE FROM notifications WHERE system_key IS NOT NULL;
UPDATE notifications SET status = 'failed' WHERE status = 'sending';
UPDATE notifications SET status = 'failed', error = 'Queued when the queue was removed' WHERE status = 'queued';
ALTER TABLE notifications DROP CONSTRAINT notifications_status_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_status_check
  CHECK (status IN ('queued', 'sent', 'failed', 'skipped'));
ALTER TABLE notifications
  DROP CONSTRAINT notifications_no_payload_when_done,
  DROP CONSTRAINT notifications_one_key,
  DROP COLUMN expires_at,
  DROP COLUMN last_attempt_at,
  DROP COLUMN locked_until,
  DROP COLUMN next_attempt_at,
  DROP COLUMN attempts,
  DROP COLUMN payload,
  DROP COLUMN system_key;
ALTER TABLE notifications ALTER COLUMN template_key SET NOT NULL;
