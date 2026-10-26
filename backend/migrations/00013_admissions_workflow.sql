-- Staff admissions workflow (Stage 10a): review, information requests,
-- offers, acceptance and enrolment (brief 8). The allowed status changes are
-- a table, checked by a trigger, so no code path can skip a step; the Go
-- state machine is tested against the same table.

-- +goose Up
CREATE TABLE application_transitions (
  from_status  text NOT NULL,
  to_status    text NOT NULL,
  PRIMARY KEY (from_status, to_status)
);
INSERT INTO application_transitions (from_status, to_status) VALUES
  ('draft', 'submitted'), ('draft', 'expired'), ('draft', 'withdrawn'),
  ('submitted', 'under_review'), ('submitted', 'withdrawn'),
  ('under_review', 'info_requested'), ('under_review', 'offered'), ('under_review', 'rejected'), ('under_review', 'withdrawn'),
  ('info_requested', 'under_review'), ('info_requested', 'rejected'), ('info_requested', 'withdrawn'),
  ('offered', 'offer_accepted'), ('offered', 'withdrawn'), ('offered', 'expired'),
  ('offer_accepted', 'enrolled'), ('offer_accepted', 'withdrawn');

-- +goose StatementBegin
CREATE FUNCTION check_application_transition() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    IF NEW.status <> 'draft' THEN
      RAISE EXCEPTION 'a new application must start as draft, not %', NEW.status
        USING ERRCODE = 'check_violation';
    END IF;
  ELSIF NEW.status IS DISTINCT FROM OLD.status AND NOT EXISTS (
    SELECT 1 FROM application_transitions WHERE from_status = OLD.status AND to_status = NEW.status
  ) THEN
    RAISE EXCEPTION 'application status cannot change from % to %', OLD.status, NEW.status
      USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER applications_transition BEFORE INSERT OR UPDATE OF status ON applications
  FOR EACH ROW EXECUTE FUNCTION check_application_transition();

CREATE INDEX applications_assigned_idx ON applications (assigned_to) WHERE status IN ('submitted', 'under_review', 'info_requested');

-- What staff asked the applicant for, and the answer. One open request at a
-- time; it closes when the applicant answers or staff resume the review.
CREATE TABLE application_info_requests (
  id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id  uuid        NOT NULL REFERENCES applications (id) ON DELETE RESTRICT,
  message         text        NOT NULL CHECK (length(trim(message)) > 0 AND length(message) <= 2000),
  requested_by    uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  requested_at    timestamptz NOT NULL DEFAULT now(),
  response        text        NOT NULL DEFAULT '' CHECK (length(response) <= 2000),
  closed_at       timestamptz,
  -- NULL when the applicant answered; the member of staff otherwise.
  closed_by       uuid        REFERENCES users (id) ON DELETE RESTRICT,
  CHECK (closed_at IS NOT NULL OR (response = '' AND closed_by IS NULL))
);
CREATE UNIQUE INDEX application_info_requests_open_idx ON application_info_requests (application_id) WHERE closed_at IS NULL;
CREATE INDEX application_info_requests_application_idx ON application_info_requests (application_id, requested_at);

-- Internal notes, for staff only. Append-only: a correction is a new note.
CREATE TABLE application_notes (
  id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id  uuid        NOT NULL REFERENCES applications (id) ON DELETE RESTRICT,
  author_id       uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  body            text        NOT NULL CHECK (length(trim(body)) > 0 AND length(body) <= 5000),
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX application_notes_application_idx ON application_notes (application_id, created_at);
CREATE TRIGGER application_notes_append_only BEFORE UPDATE OR DELETE ON application_notes
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();

ALTER TABLE offers ALTER COLUMN respond_by SET NOT NULL;

-- Students (brief 10). Stage 10 creates the record at enrolment; Stage 11
-- adds guardians, cohorts and units. Soft status, never deleted.
CREATE TABLE students (
  id                      uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  student_number          text        NOT NULL UNIQUE,
  user_id                 uuid        NOT NULL UNIQUE REFERENCES users (id) ON DELETE RESTRICT,
  applicant_id            uuid        NOT NULL REFERENCES applicants (id) ON DELETE RESTRICT,
  application_id          uuid        NOT NULL UNIQUE REFERENCES applications (id) ON DELETE RESTRICT,
  programme_id            uuid        NOT NULL REFERENCES programmes (id) ON DELETE RESTRICT,
  intake_id               uuid        REFERENCES programme_intakes (id) ON DELETE RESTRICT,
  first_name              text        NOT NULL,
  middle_name             text        NOT NULL DEFAULT '',
  last_name               text        NOT NULL,
  date_of_birth           date,
  email                   citext      NOT NULL,
  phone                   text,
  identity_document_type  text        CHECK (identity_document_type IN ('national_id', 'passport', 'birth_certificate')),
  identity_document       text        NOT NULL DEFAULT '',
  status                  text        NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active', 'deferred', 'suspended', 'withdrawn', 'graduated')),
  enrolled_at             timestamptz NOT NULL DEFAULT now(),
  enrolled_by             uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX students_programme_idx ON students (programme_id, intake_id);
CREATE TRIGGER students_updated_at BEFORE UPDATE ON students FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Invoices (brief 9). Enrolment opens the first one as an empty draft; Stage
-- 14 adds lines from the fee structure, numbers and issues it. Amounts are
-- never stored here: balances derive from lines and payments.
CREATE TABLE invoices (
  id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  student_id  uuid        NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
  intake_id   uuid        REFERENCES programme_intakes (id) ON DELETE RESTRICT,
  -- Set when issued (Stage 14), from a gapless counter.
  number      text        UNIQUE,
  status      text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'issued', 'void')),
  note        text        NOT NULL DEFAULT '',
  created_by  uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  CHECK (status = 'draft' OR number IS NOT NULL)
);
CREATE INDEX invoices_student_idx ON invoices (student_id, created_at);
CREATE TRIGGER invoices_updated_at BEFORE UPDATE ON invoices FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO site_settings (key, value, description) VALUES
  ('admissions.offer_response_days', '14', 'Days an applicant has to accept an offer before it expires');

-- The offer letter is a template too ("letter" channel: printed, not sent).
ALTER TABLE notification_templates DROP CONSTRAINT notification_templates_channel_check;
ALTER TABLE notification_templates ADD CONSTRAINT notification_templates_channel_check
  CHECK (channel IN ('email', 'sms', 'letter'));

INSERT INTO notification_templates (key, channel, name, description, subject, body) VALUES
  ('application.info_requested', 'email', 'More information needed',
   'Sent when the admissions office asks the applicant for more information.',
   'Application {reference}: we need more information',
   E'Hello {name},\n\nThe admissions office is reviewing your application {reference} and needs more from you:\n\n{message}\n\nOpen your application to reply or upload documents: {link}\n\nDakota Training College admissions'),
  ('application.info_requested', 'sms', 'More information needed',
   'Text message version. Not sent until an SMS provider is set up (DG-03).', '',
   'Dakota Training College: we need more information for application {reference}. Check your email or open {link}'),
  ('application.offered', 'email', 'Offer of a place',
   'Sent when the admissions office offers the applicant a place. The offer letter is in the application.',
   'Offer of a place: {programme}',
   E'Hello {name},\n\nWe are pleased to offer you a place on {programme} ({intake}).\n\nYour offer letter is ready in your application. Please accept or decline by {respond_by}: {link}\n\nDakota Training College admissions'),
  ('application.offered', 'sms', 'Offer of a place',
   'Text message version. Not sent until an SMS provider is set up (DG-03).', '',
   'Dakota Training College: you have been offered a place on {programme}. Please reply by {respond_by} at {link}'),
  ('application.rejected', 'email', 'Application not successful',
   'Sent when the admissions office decides not to offer a place. The reason is included.',
   'Your application {reference}',
   E'Hello {name},\n\nThank you for applying for {programme}. We are sorry that we cannot offer you a place this time.\n\nReason: {reason}\n\nYou can see your application at {link}. If you have questions, reply to this email or contact the admissions office.\n\nDakota Training College admissions'),
  ('application.rejected', 'sms', 'Application not successful',
   'Text message version. Not sent until an SMS provider is set up (DG-03).', '',
   'Dakota Training College: a decision was made on application {reference}. Please check your email or {link}'),
  ('application.enrolled', 'email', 'Enrolled as a student',
   'Sent when the registrar enrols an applicant who accepted an offer.',
   'Welcome to Dakota Training College',
   E'Hello {name},\n\nYou are now enrolled on {programme}. Your student number is {student_number}. Keep it: you will use it to sign in to the student portal.\n\nThe college will contact you about fees and your first day.\n\nDakota Training College'),
  ('application.enrolled', 'sms', 'Enrolled as a student',
   'Text message version. Not sent until an SMS provider is set up (DG-03).', '',
   'Dakota Training College: you are enrolled on {programme}. Your student number is {student_number}.'),
  ('offer.letter', 'letter', 'Offer letter',
   'The letter attached to an offer, made as a PDF. The college logo, date, fees and any conditions are added around this text.',
   'Offer of admission',
   E'Dear {name},\n\nWe are pleased to offer you a place on {programme}, starting with the {intake} intake on {starts_on}.\n\nTo accept this offer, sign in to your application on our website and choose "Accept offer" by {respond_by}. If we do not hear from you by then, the offer will lapse.\n\nOnce you accept, the registrar will enrol you and send you your student number.\n\nWe look forward to welcoming you.');

-- +goose Down
DELETE FROM notifications WHERE template_key IN
  ('application.info_requested', 'application.offered', 'application.rejected', 'application.enrolled');
DELETE FROM notification_templates WHERE key IN
  ('application.info_requested', 'application.offered', 'application.rejected', 'application.enrolled', 'offer.letter');
ALTER TABLE notification_templates DROP CONSTRAINT notification_templates_channel_check;
ALTER TABLE notification_templates ADD CONSTRAINT notification_templates_channel_check
  CHECK (channel IN ('email', 'sms'));
DELETE FROM site_settings WHERE key = 'admissions.offer_response_days';
DROP TABLE invoices;
DROP TABLE students;
ALTER TABLE offers ALTER COLUMN respond_by DROP NOT NULL;
DROP TABLE application_notes;
DROP TABLE application_info_requests;
DROP INDEX applications_assigned_idx;
DROP TRIGGER applications_transition ON applications;
DROP FUNCTION check_application_transition();
DROP TABLE application_transitions;
