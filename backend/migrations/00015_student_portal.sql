-- Student portal (Stage 12a): notices, service requests and profile change
-- requests, plus the messages that go with them. Students sign in with
-- their student number and the password they set from the link in their
-- enrolment email (D-141).

-- +goose Up
-- Announcements to everyone, a school, a cohort or one class (offering).
CREATE TABLE notices (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  audience      text        NOT NULL CHECK (audience IN ('all', 'school', 'cohort', 'offering')),
  school_id     uuid        REFERENCES schools (id) ON DELETE CASCADE,
  cohort_id     uuid        REFERENCES cohorts (id) ON DELETE CASCADE,
  offering_id   uuid        REFERENCES unit_offerings (id) ON DELETE CASCADE,
  title         text        NOT NULL CHECK (length(trim(title)) BETWEEN 1 AND 160),
  body          text        NOT NULL CHECK (length(trim(body)) BETWEEN 1 AND 5000),
  ends_on       date,
  created_by    uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  created_at    timestamptz NOT NULL DEFAULT now(),
  withdrawn_at  timestamptz,
  withdrawn_by  uuid        REFERENCES users (id) ON DELETE RESTRICT,
  CHECK ((audience = 'school') = (school_id IS NOT NULL)),
  CHECK ((audience = 'cohort') = (cohort_id IS NOT NULL)),
  CHECK ((audience = 'offering') = (offering_id IS NOT NULL)),
  CHECK ((withdrawn_at IS NULL) = (withdrawn_by IS NULL))
);
CREATE INDEX notices_recent_idx ON notices (created_at DESC) WHERE withdrawn_at IS NULL;

-- Letters, transcripts, deferment, change of course (brief 7.1).
CREATE TABLE service_requests (
  id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  student_id   uuid        NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
  kind         text        NOT NULL CHECK (kind IN ('letter', 'transcript', 'deferment', 'change_of_course', 'other')),
  details      text        NOT NULL CHECK (length(trim(details)) BETWEEN 1 AND 2000),
  status       text        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'in_progress', 'done', 'declined')),
  response     text        NOT NULL DEFAULT '' CHECK (length(response) <= 2000),
  handled_by   uuid        REFERENCES users (id) ON DELETE RESTRICT,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  CHECK (status <> 'declined' OR length(trim(response)) > 0)
);
CREATE INDEX service_requests_student_idx ON service_requests (student_id, created_at DESC);
CREATE INDEX service_requests_open_idx ON service_requests (status, created_at) WHERE status IN ('open', 'in_progress');
CREATE TRIGGER service_requests_updated_at BEFORE UPDATE ON service_requests FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Changes to identity details need the registrar (D-142). The old and new
-- values are kept, so an approved change can always be traced.
CREATE TABLE profile_change_requests (
  id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  student_id   uuid        NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
  old_values   jsonb       NOT NULL CHECK (jsonb_typeof(old_values) = 'object'),
  new_values   jsonb       NOT NULL CHECK (jsonb_typeof(new_values) = 'object' AND new_values <> '{}'::jsonb),
  reason       text        NOT NULL CHECK (length(trim(reason)) BETWEEN 1 AND 2000),
  status       text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'declined')),
  decision     text        NOT NULL DEFAULT '' CHECK (length(decision) <= 2000),
  decided_by   uuid        REFERENCES users (id) ON DELETE RESTRICT,
  decided_at   timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  CHECK ((status = 'pending') = (decided_at IS NULL)),
  CHECK (status <> 'declined' OR length(trim(decision)) > 0)
);
-- One pending request at a time per student.
CREATE UNIQUE INDEX profile_change_requests_pending_idx ON profile_change_requests (student_id) WHERE status = 'pending';

-- The enrolment email now carries the link to choose a password. Changed
-- only if staff have not reworded it.
UPDATE notification_templates
SET body = E'Hello {name},\n\nYou are now enrolled on {programme}. Your student number is {student_number}. Keep it: you will use it to sign in to the student portal.\n\nChoose your password here (the link works once, for 7 days): {link}\n\nThe college will contact you about fees and your first day.\n\nDakota Training College'
WHERE key = 'application.enrolled' AND channel = 'email' AND updated_by IS NULL;

INSERT INTO notification_templates (key, channel, name, description, subject, body) VALUES
  ('student.request_updated', 'email', 'Student request updated',
   'Sent when staff change the status of a student''s service request (letter, transcript and so on).',
   'Your request: {kind}',
   E'Hello {name},\n\nYour request for {kind} is now: {status}.\n\n{response}\n\nSee it in the student portal: {link}\n\nDakota Training College'),
  ('student.profile_change_decided', 'email', 'Profile change decided',
   'Sent when the registrar approves or declines a change to a student''s personal details.',
   'Your change of details was {decision}',
   E'Hello {name},\n\nYour request to change your personal details was {decision}.\n\n{reason}\n\nSee your profile in the student portal: {link}\n\nDakota Training College');

-- +goose Down
DELETE FROM notifications WHERE template_key IN ('student.request_updated', 'student.profile_change_decided');
DELETE FROM notification_templates WHERE key IN ('student.request_updated', 'student.profile_change_decided');
UPDATE notification_templates
SET body = E'Hello {name},\n\nYou are now enrolled on {programme}. Your student number is {student_number}. Keep it: you will use it to sign in to the student portal.\n\nThe college will contact you about fees and your first day.\n\nDakota Training College'
WHERE key = 'application.enrolled' AND channel = 'email' AND updated_by IS NULL;
DROP TABLE profile_change_requests;
DROP TABLE service_requests;
DROP TABLE notices;
