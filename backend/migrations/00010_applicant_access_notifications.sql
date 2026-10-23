-- Applicant sign-in and notification templates (Stage 8b).
-- Applicants sign in with a one-time link sent to their email, never a
-- password (D-96). Templates are editable text; the notification log records
-- what was sent to whom, but never the body (sign-in links are secrets).

-- +goose Up
CREATE TABLE applicant_sign_in_links (
  id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  email           citext      NOT NULL,
  -- Name typed on the start form; used only if this link creates the account.
  full_name       text        NOT NULL DEFAULT '',
  -- Set when the link came from "resume by reference number".
  application_id  uuid        REFERENCES applications (id) ON DELETE CASCADE,
  token_hash      bytea       NOT NULL UNIQUE,
  expires_at      timestamptz NOT NULL,
  used_at         timestamptz,
  requested_ip    inet,
  created_at      timestamptz NOT NULL DEFAULT now(),
  CHECK (expires_at > created_at)
);
CREATE INDEX applicant_sign_in_links_email_idx ON applicant_sign_in_links (email) WHERE used_at IS NULL;

CREATE INDEX applications_applicant_idx ON applications (applicant_id, created_at DESC);

-- One template per message and channel. SMS text is kept ready but is not
-- sent until an SMS provider is chosen (DG-03).
CREATE TABLE notification_templates (
  key          text        NOT NULL CHECK (key ~ '^[a-z]+(\.[a-z_]+)+$'),
  channel      text        NOT NULL CHECK (channel IN ('email', 'sms')),
  name         text        NOT NULL,
  description  text        NOT NULL DEFAULT '',
  subject      text        NOT NULL DEFAULT '',
  body         text        NOT NULL CHECK (length(trim(body)) > 0),
  updated_by   uuid        REFERENCES users (id) ON DELETE RESTRICT,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (key, channel),
  CHECK (channel <> 'email' OR length(trim(subject)) > 0),
  CHECK (channel <> 'sms' OR (subject = '' AND length(body) <= 320))
);
CREATE TRIGGER notification_templates_updated_at BEFORE UPDATE ON notification_templates
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE notifications (
  id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  template_key   text        NOT NULL,
  channel        text        NOT NULL CHECK (channel IN ('email', 'sms')),
  recipient      text        NOT NULL,
  status         text        NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'sent', 'failed', 'skipped')),
  error          text        NOT NULL DEFAULT '',
  related_type   text        NOT NULL DEFAULT '',
  related_id     text        NOT NULL DEFAULT '',
  created_at     timestamptz NOT NULL DEFAULT now(),
  sent_at        timestamptz,
  FOREIGN KEY (template_key, channel) REFERENCES notification_templates (key, channel) ON DELETE RESTRICT,
  CHECK ((status = 'sent') = (sent_at IS NOT NULL))
);
CREATE INDEX notifications_created_idx ON notifications (created_at DESC);
CREATE INDEX notifications_related_idx ON notifications (related_type, related_id);

INSERT INTO permissions (key, module, description) VALUES
  ('notifications.manage', 'admin', 'Edit the text of emails and text messages sent to applicants');

INSERT INTO role_permissions (role_id, permission_key)
SELECT r.id, 'notifications.manage' FROM roles r WHERE r.key IN ('super_admin', 'ict_admin', 'registrar')
ON CONFLICT DO NOTHING;

-- Starter text. Placeholders in braces are filled in when a message is sent;
-- staff can reword everything else in the portal.
INSERT INTO notification_templates (key, channel, name, description, subject, body) VALUES
  ('applicant.sign_in_link', 'email', 'Applicant sign-in link',
   'Sent when someone starts or continues an application on the website.',
   'Your link to continue your application',
   E'Hello {name},\n\nUse this link to continue your application to Dakota Training College:\n\n{link}\n\nThe link works once, for {minutes} minutes. If it has expired, ask for a new one on the website.\n\nIf you did not ask for this, you can ignore this email.\n\nDakota Training College admissions'),
  ('applicant.resume_link', 'email', 'Resume by reference number',
   'Sent when someone enters a reference number and the email address on that application.',
   'Continue application {reference}',
   E'Hello {name},\n\nUse this link to continue application {reference}:\n\n{link}\n\nThe link works once, for {minutes} minutes.\n\nIf you did not ask for this, you can ignore this email.\n\nDakota Training College admissions'),
  ('application.started', 'email', 'Reference number for a new application',
   'Sent when an applicant starts an application (online form, Stage 9).',
   'Your application reference: {reference}',
   E'Hello {name},\n\nThank you for starting an application for {programme}.\n\nYour reference number is {reference}. Keep it: you can use it with your email address to continue your application at {link}\n\nDakota Training College admissions'),
  ('application.started', 'sms', 'Reference number for a new application',
   'Text message version. Not sent until an SMS provider is set up (DG-03).',
   '',
   'Dakota Training College: your application reference is {reference}. Use it with your email to continue at {link}'),
  ('application.submitted', 'email', 'Application received',
   'Sent when an applicant submits an application (online form, Stage 9).',
   'We received your application {reference}',
   E'Hello {name},\n\nWe received your application for {programme}. Your reference number is {reference}.\n\nThe admissions office will review it and contact you. You can check its status at any time at {link}\n\nDakota Training College admissions'),
  ('application.submitted', 'sms', 'Application received',
   'Text message version. Not sent until an SMS provider is set up (DG-03).',
   '',
   'Dakota Training College: we received application {reference}. Check its status at {link}');

-- +goose Down
DELETE FROM role_permissions WHERE permission_key = 'notifications.manage';
DELETE FROM permissions WHERE key = 'notifications.manage';
DROP TABLE notifications;
DROP TABLE notification_templates;
DROP INDEX applications_applicant_idx;
DROP TABLE applicant_sign_in_links;
