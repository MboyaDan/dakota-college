-- Data protection tools (Stage 17c, brief 13): the retention schedule, the
-- register of data subject requests, and erasure of applicants who did not
-- enrol. Student, academic and financial records are never erased here.

-- +goose Up
INSERT INTO permissions (key, module, description) VALUES
  ('privacy.manage', 'admin', 'Record data protection requests, export a person''s data and erase applicants who did not enrol');

INSERT INTO role_permissions (role_id, permission_key)
SELECT r.id, 'privacy.manage' FROM roles r WHERE r.key IN ('super_admin', 'principal', 'registrar')
ON CONFLICT DO NOTHING;

-- Set when an applicant's personal details were erased (by the retention
-- schedule or on request). The row stays so references and counts survive.
ALTER TABLE applicants ADD COLUMN erased_at timestamptz;

-- Set once the object behind a deleted file is gone from storage. Deleted
-- files without it are retried by the next retention run.
ALTER TABLE files ADD COLUMN storage_removed_at timestamptz;
ALTER TABLE files ADD CONSTRAINT files_removed_after_deleted
  CHECK (storage_removed_at IS NULL OR deleted_at IS NOT NULL);
CREATE INDEX files_storage_pending_idx ON files (deleted_at) WHERE deleted_at IS NOT NULL AND storage_removed_at IS NULL;

-- Append-only tables that can hold free text about a person still refuse
-- deletes and ordinary updates. Erasure may blank the listed columns, and
-- nothing else, inside a transaction that set dtc.erasure to 'on'.
-- +goose StatementBegin
CREATE FUNCTION append_only_except_erasure() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'UPDATE'
     AND current_setting('dtc.erasure', true) = 'on'
     AND (to_jsonb(NEW) - TG_ARGV) = (to_jsonb(OLD) - TG_ARGV) THEN
    RETURN NEW;
  END IF;
  RAISE EXCEPTION '% is append-only: % is not allowed', TG_TABLE_NAME, TG_OP
    USING ERRCODE = 'restrict_violation';
END;
$$;
-- +goose StatementEnd

DROP TRIGGER application_notes_append_only ON application_notes;
CREATE TRIGGER application_notes_append_only BEFORE UPDATE OR DELETE ON application_notes
  FOR EACH ROW EXECUTE FUNCTION append_only_except_erasure('body');
DROP TRIGGER application_status_history_append_only ON application_status_history;
CREATE TRIGGER application_status_history_append_only BEFORE UPDATE OR DELETE ON application_status_history
  FOR EACH ROW EXECUTE FUNCTION append_only_except_erasure('reason');
DROP TRIGGER consents_append_only ON consents;
CREATE TRIGGER consents_append_only BEFORE UPDATE OR DELETE ON consents
  FOR EACH ROW EXECUTE FUNCTION append_only_except_erasure('given_by_name', 'ip', 'user_agent');

-- One row per retention run, scheduled or from the command line.
CREATE TABLE retention_runs (
  id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  mode         text        NOT NULL CHECK (mode IN ('report', 'apply')),
  started_at   timestamptz NOT NULL DEFAULT now(),
  finished_at  timestamptz,
  -- [{"rule": "...", "due": n, "done": n}]
  results      jsonb       NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(results) = 'array'),
  error        text        NOT NULL DEFAULT ''
);
CREATE INDEX retention_runs_started_idx ON retention_runs (started_at DESC);

-- Register of requests from people about their own data (Data Protection
-- Act, 2019). Staff record each request when it arrives.
CREATE TABLE data_requests (
  id                   uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  -- DTC-DSR-2026-0001, from document_counters.
  reference            text        NOT NULL UNIQUE,
  kind                 text        NOT NULL CHECK (kind IN (
                         'access', 'correction', 'erasure', 'objection', 'portability', 'withdraw_consent', 'other')),
  requester_name       text        NOT NULL CHECK (length(trim(requester_name)) BETWEEN 1 AND 200),
  requester_contact    text        NOT NULL CHECK (length(trim(requester_contact)) BETWEEN 1 AND 200),
  -- Empty when the requester is the person; otherwise, for example, "parent".
  acting_for           text        NOT NULL DEFAULT '' CHECK (length(acting_for) <= 200),
  -- How to find the person's records.
  subject_email        citext,
  subject_phone        text,
  details              text        NOT NULL DEFAULT '' CHECK (length(details) <= 5000),
  received_on          date        NOT NULL,
  due_on               date        NOT NULL,
  status               text        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'completed', 'refused')),
  identity_checked_at  timestamptz,
  identity_checked_by  uuid        REFERENCES users (id) ON DELETE RESTRICT,
  identity_method      text        NOT NULL DEFAULT '' CHECK (length(identity_method) <= 500),
  outcome              text        NOT NULL DEFAULT '' CHECK (length(outcome) <= 5000),
  closed_at            timestamptz,
  closed_by            uuid        REFERENCES users (id) ON DELETE RESTRICT,
  created_by           uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  CHECK (subject_email IS NOT NULL OR subject_phone IS NOT NULL),
  CHECK (due_on >= received_on),
  CHECK ((status = 'open') = (closed_at IS NULL)),
  CHECK ((closed_at IS NULL) = (closed_by IS NULL)),
  CHECK (status = 'open' OR length(trim(outcome)) > 0),
  CHECK ((identity_checked_at IS NULL) = (identity_checked_by IS NULL))
);
CREATE TRIGGER data_requests_updated_at BEFORE UPDATE ON data_requests FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX data_requests_open_idx ON data_requests (due_on) WHERE status = 'open';

-- What was done on a request, in order. Never edited.
CREATE TABLE data_request_events (
  id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  request_id  uuid        NOT NULL REFERENCES data_requests (id) ON DELETE RESTRICT,
  action      text        NOT NULL CHECK (action IN (
                'recorded', 'identity_checked', 'note', 'exported', 'erased', 'completed', 'refused')),
  note        text        NOT NULL DEFAULT '' CHECK (length(note) <= 5000),
  actor_id    uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX data_request_events_request_idx ON data_request_events (request_id, id);
CREATE TRIGGER data_request_events_append_only BEFORE UPDATE OR DELETE ON data_request_events
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();

-- +goose Down
DROP TABLE data_request_events;
DROP TABLE data_requests;
DROP TABLE retention_runs;

DROP TRIGGER consents_append_only ON consents;
CREATE TRIGGER consents_append_only BEFORE UPDATE OR DELETE ON consents
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();
DROP TRIGGER application_status_history_append_only ON application_status_history;
CREATE TRIGGER application_status_history_append_only BEFORE UPDATE OR DELETE ON application_status_history
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();
DROP TRIGGER application_notes_append_only ON application_notes;
CREATE TRIGGER application_notes_append_only BEFORE UPDATE OR DELETE ON application_notes
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();
DROP FUNCTION append_only_except_erasure();

DROP INDEX files_storage_pending_idx;
ALTER TABLE files DROP CONSTRAINT files_removed_after_deleted;
ALTER TABLE files DROP COLUMN storage_removed_at;
ALTER TABLE applicants DROP COLUMN erased_at;

DELETE FROM role_permissions WHERE permission_key = 'privacy.manage';
DELETE FROM permissions WHERE key = 'privacy.manage';
