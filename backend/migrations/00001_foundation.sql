-- Shared helpers used by later migrations.

-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

-- Keeps updated_at current on tables that have it.
-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- Append-only tables (audit log, status history, consents) reject UPDATE and
-- DELETE. Corrections are new rows (NN-05).
-- +goose StatementBegin
CREATE FUNCTION forbid_update_delete() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION '% is append-only: % is not allowed', TG_TABLE_NAME, TG_OP
    USING ERRCODE = 'restrict_violation';
END;
$$;
-- +goose StatementEnd

-- Gapless counters for application references, student numbers and receipts
-- (A-06). A row is locked with UPDATE ... RETURNING inside the caller's
-- transaction, so concurrent callers queue instead of skipping numbers.
CREATE TABLE document_counters (
  name        text    NOT NULL,
  period      integer NOT NULL,
  last_value  bigint  NOT NULL DEFAULT 0 CHECK (last_value >= 0),
  PRIMARY KEY (name, period)
);

-- +goose Down
DROP TABLE document_counters;
DROP FUNCTION forbid_update_delete();
DROP FUNCTION set_updated_at();
-- citext is left installed: other schemas in a shared database may use it.
