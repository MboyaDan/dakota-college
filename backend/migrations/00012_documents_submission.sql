-- Application documents, submission and short-lived file links (Stage 9b).

-- +goose Up
ALTER TABLE application_documents DROP CONSTRAINT application_documents_kind_check;
ALTER TABLE application_documents ADD CONSTRAINT application_documents_kind_check CHECK (kind IN (
  'national_id', 'birth_certificate', 'passport', 'kcse_certificate', 'other_certificate', 'passport_photo', 'other'));

-- The application exactly as submitted. Later corrections never change it.
ALTER TABLE applications ADD COLUMN submitted_snapshot jsonb;
ALTER TABLE applications ADD CONSTRAINT applications_snapshot_on_submit
  CHECK (submitted_at IS NULL OR submitted_snapshot IS NOT NULL);

-- Short-lived links to private files (AC-05). Only a hash of the token is
-- stored; a link works only for the account it was issued to.
CREATE TABLE file_links (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  token_hash    bytea       NOT NULL UNIQUE,
  file_id       uuid        NOT NULL REFERENCES files (id) ON DELETE CASCADE,
  issued_to     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  expires_at    timestamptz NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  CHECK (expires_at > created_at)
);
CREATE INDEX file_links_expires_idx ON file_links (expires_at);

-- +goose Down
DROP TABLE file_links;
ALTER TABLE applications DROP CONSTRAINT applications_snapshot_on_submit;
ALTER TABLE applications DROP COLUMN submitted_snapshot;
DELETE FROM application_documents WHERE kind = 'passport';
ALTER TABLE application_documents DROP CONSTRAINT application_documents_kind_check;
ALTER TABLE application_documents ADD CONSTRAINT application_documents_kind_check CHECK (kind IN (
  'national_id', 'birth_certificate', 'kcse_certificate', 'other_certificate', 'passport_photo', 'other'));
