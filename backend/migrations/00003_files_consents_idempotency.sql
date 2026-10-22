-- Operations tables that content and admissions depend on (brief section 10).

-- +goose Up
-- Metadata for objects in private S3-compatible storage. Bytes never live in
-- the database.
CREATE TABLE files (
  id                   uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  bucket               text        NOT NULL,
  object_key           text        NOT NULL,
  original_name        text        NOT NULL,
  content_type         text        NOT NULL,
  size_bytes           bigint      NOT NULL CHECK (size_bytes > 0),
  sha256               bytea       NOT NULL CHECK (length(sha256) = 32),
  -- 'not_scanned' until a scanner actually runs (D-19).
  scan_status          text        NOT NULL DEFAULT 'not_scanned'
                         CHECK (scan_status IN ('not_scanned', 'clean', 'infected', 'error')),
  uploaded_by_user_id  uuid        REFERENCES users (id) ON DELETE RESTRICT,
  created_at           timestamptz NOT NULL DEFAULT now(),
  deleted_at           timestamptz,
  UNIQUE (bucket, object_key)
);

-- Consent records (brief 13, AC-13, NN-08). Append-only: withdrawing consent
-- is a new row with given = false.
CREATE TABLE consents (
  id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  subject_type    text        NOT NULL CHECK (subject_type IN ('applicant', 'guardian', 'enquiry', 'student', 'user')),
  subject_id      uuid        NOT NULL,
  purpose         text        NOT NULL CHECK (purpose IN ('data_processing', 'guardian_consent', 'contact', 'photo')),
  given           boolean     NOT NULL,
  notice_version  text        NOT NULL,
  given_by_name   text        NOT NULL,
  captured_at     timestamptz NOT NULL DEFAULT now(),
  ip              inet,
  user_agent      text
);
CREATE INDEX consents_subject_idx ON consents (subject_type, subject_id, purpose, captured_at DESC);
CREATE TRIGGER consents_append_only BEFORE UPDATE OR DELETE ON consents
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();

-- Idempotency keys for submissions and payments (ADR-07, NN-04). A repeated
-- key with the same request returns the stored response; with a different
-- request it is rejected.
CREATE TABLE idempotency_records (
  scope            text        NOT NULL,
  key              text        NOT NULL CHECK (length(key) BETWEEN 8 AND 128),
  request_hash     bytea       NOT NULL,
  response_status  integer,
  response_body    jsonb,
  created_at       timestamptz NOT NULL DEFAULT now(),
  completed_at     timestamptz,
  expires_at       timestamptz NOT NULL,
  PRIMARY KEY (scope, key)
);
CREATE INDEX idempotency_records_expires_idx ON idempotency_records (expires_at);

-- +goose Down
DROP TABLE idempotency_records;
DROP TABLE consents;
DROP TABLE files;
