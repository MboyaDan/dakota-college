-- Content workflow (Stage 5). Editing a published item must not change the
-- live site until someone with content.publish approves it (DQ-14). Those
-- edits wait here as a full document; publishing applies them and deletes
-- the row. Draft items are edited in place (they are not public).

-- +goose Up
CREATE TABLE content_pending (
  entity_type  text        NOT NULL,
  entity_id    uuid        NOT NULL,
  document     jsonb       NOT NULL,
  updated_by   uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (entity_type, entity_id)
);

-- Uploaded images are re-encoded into these widths (smallest first); the
-- largest is the stored object, the others share its key prefix.
ALTER TABLE files
  ADD COLUMN width integer CHECK (width > 0),
  ADD COLUMN height integer CHECK (height > 0),
  ADD COLUMN image_widths integer[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE files DROP COLUMN image_widths, DROP COLUMN height, DROP COLUMN width;
DROP TABLE content_pending;
