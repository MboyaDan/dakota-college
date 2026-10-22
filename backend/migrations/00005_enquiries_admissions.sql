-- Enquiries and admissions (brief sections 8 and 10). The application state
-- machine is enforced in the backend (Stage 10); the database guarantees the
-- invariants that must never break whatever the code does.

-- +goose Up
CREATE TABLE enquiries (
  id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  full_name       text        NOT NULL CHECK (length(trim(full_name)) > 0),
  email           citext,
  phone           text,
  query_type      text        NOT NULL CHECK (query_type IN ('admissions', 'programmes', 'fees', 'student_life', 'other')),
  programme_id    uuid        REFERENCES programmes (id) ON DELETE SET NULL,
  message         text        NOT NULL CHECK (length(message) BETWEEN 1 AND 5000),
  source          text        NOT NULL DEFAULT 'website' CHECK (source IN ('website', 'whatsapp', 'phone', 'walk_in', 'email')),
  status          text        NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'open', 'waiting', 'closed')),
  assigned_to     uuid        REFERENCES users (id) ON DELETE RESTRICT,
  ip              inet,
  user_agent      text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  closed_at       timestamptz,
  CHECK (email IS NOT NULL OR phone IS NOT NULL),
  CHECK ((status = 'closed') = (closed_at IS NOT NULL))
);
CREATE INDEX enquiries_inbox_idx ON enquiries (status, created_at DESC);
CREATE INDEX enquiries_assigned_idx ON enquiries (assigned_to) WHERE status <> 'closed';
CREATE TRIGGER enquiries_updated_at BEFORE UPDATE ON enquiries FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE enquiry_notes (
  id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  enquiry_id  uuid        NOT NULL REFERENCES enquiries (id) ON DELETE CASCADE,
  author_id   uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  body        text        NOT NULL CHECK (length(trim(body)) > 0),
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX enquiry_notes_enquiry_idx ON enquiry_notes (enquiry_id, created_at);

CREATE TABLE applicants (
  id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id            uuid        UNIQUE REFERENCES users (id) ON DELETE RESTRICT,
  email              citext      NOT NULL,
  phone              text,
  first_name         text        NOT NULL,
  middle_name        text        NOT NULL DEFAULT '',
  last_name          text        NOT NULL,
  date_of_birth      date,
  identity_document  text        NOT NULL DEFAULT '',
  nationality        text        NOT NULL DEFAULT '',
  county             text        NOT NULL DEFAULT '',
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  CHECK (date_of_birth IS NULL OR date_of_birth > DATE '1900-01-01')
);
CREATE INDEX applicants_email_idx ON applicants (email);
CREATE TRIGGER applicants_updated_at BEFORE UPDATE ON applicants FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE applications (
  id                       uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  -- Human reference, e.g. DTC-APP-2026-000123, from document_counters.
  reference                text        NOT NULL UNIQUE,
  applicant_id             uuid        NOT NULL REFERENCES applicants (id) ON DELETE RESTRICT,
  programme_id             uuid        NOT NULL REFERENCES programmes (id) ON DELETE RESTRICT,
  intake_id                uuid        REFERENCES programme_intakes (id) ON DELETE RESTRICT,
  status                   text        NOT NULL DEFAULT 'draft' CHECK (status IN (
                             'draft', 'submitted', 'under_review', 'info_requested', 'offered',
                             'offer_accepted', 'enrolled', 'rejected', 'withdrawn', 'expired')),
  sponsor_type             text        CHECK (sponsor_type IN ('self', 'parent_guardian', 'sponsor', 'other')),
  sponsor_name             text        NOT NULL DEFAULT '',
  sponsor_phone            text        NOT NULL DEFAULT '',
  assigned_to              uuid        REFERENCES users (id) ON DELETE RESTRICT,
  declaration_accepted_at  timestamptz,
  submitted_at             timestamptz,
  decided_at               timestamptz,
  draft_expires_at         timestamptz,
  created_at               timestamptz NOT NULL DEFAULT now(),
  updated_at               timestamptz NOT NULL DEFAULT now(),
  CHECK (status = 'draft' OR status = 'expired' OR submitted_at IS NOT NULL),
  CHECK (submitted_at IS NULL OR declaration_accepted_at IS NOT NULL)
);
-- One live application per applicant, programme and intake (AC-04 retries).
CREATE UNIQUE INDEX applications_one_active_idx
  ON applications (applicant_id, programme_id, COALESCE(intake_id, '00000000-0000-0000-0000-000000000000'::uuid))
  WHERE status NOT IN ('rejected', 'withdrawn', 'expired');
CREATE INDEX applications_inbox_idx ON applications (status, submitted_at DESC);
CREATE INDEX applications_programme_idx ON applications (programme_id, intake_id);
CREATE TRIGGER applications_updated_at BEFORE UPDATE ON applications FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE application_guardians (
  id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id  uuid        NOT NULL REFERENCES applications (id) ON DELETE CASCADE,
  full_name       text        NOT NULL,
  relationship    text        NOT NULL,
  phone           text        NOT NULL,
  email           citext,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX application_guardians_application_idx ON application_guardians (application_id);

CREATE TABLE application_education (
  id              uuid     PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id  uuid     NOT NULL REFERENCES applications (id) ON DELETE CASCADE,
  level           text     NOT NULL CHECK (level IN ('kcse', 'kcpe', 'certificate', 'diploma', 'degree', 'other')),
  institution     text     NOT NULL,
  year_completed  smallint CHECK (year_completed BETWEEN 1950 AND 2100),
  qualification   text     NOT NULL DEFAULT '',
  mean_grade      text     NOT NULL DEFAULT '',
  sort_order      integer  NOT NULL DEFAULT 0
);
CREATE INDEX application_education_application_idx ON application_education (application_id, sort_order);

CREATE TABLE application_documents (
  id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id  uuid        NOT NULL REFERENCES applications (id) ON DELETE CASCADE,
  kind            text        NOT NULL CHECK (kind IN (
                    'national_id', 'birth_certificate', 'kcse_certificate', 'other_certificate', 'passport_photo', 'other')),
  file_id         uuid        NOT NULL UNIQUE REFERENCES files (id) ON DELETE RESTRICT,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX application_documents_application_idx ON application_documents (application_id);

-- Every status change: who, when, why (brief 8). Append-only.
CREATE TABLE application_status_history (
  id              bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  application_id  uuid        NOT NULL REFERENCES applications (id) ON DELETE RESTRICT,
  from_status     text,
  to_status       text        NOT NULL,
  -- NULL when the applicant made the change themselves.
  changed_by      uuid        REFERENCES users (id) ON DELETE RESTRICT,
  reason          text        NOT NULL DEFAULT '',
  created_at      timestamptz NOT NULL DEFAULT now(),
  CHECK (to_status <> 'rejected' OR length(trim(reason)) > 0)
);
CREATE INDEX application_status_history_application_idx ON application_status_history (application_id, created_at);
CREATE TRIGGER application_status_history_append_only BEFORE UPDATE OR DELETE ON application_status_history
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();

CREATE TABLE offers (
  id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id        uuid        NOT NULL UNIQUE REFERENCES applications (id) ON DELETE RESTRICT,
  letter_file_id        uuid        REFERENCES files (id) ON DELETE RESTRICT,
  conditions            text        NOT NULL DEFAULT '',
  issued_by             uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  issued_at             timestamptz NOT NULL DEFAULT now(),
  respond_by            date,
  accepted_at           timestamptz,
  declined_at           timestamptz,
  CHECK (accepted_at IS NULL OR declined_at IS NULL)
);

-- +goose Down
DROP TABLE offers;
DROP TABLE application_status_history;
DROP TABLE application_documents;
DROP TABLE application_education;
DROP TABLE application_guardians;
DROP TABLE applications;
DROP TABLE applicants;
DROP TABLE enquiry_notes;
DROP TABLE enquiries;
