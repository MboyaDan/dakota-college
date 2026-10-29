-- Photos on the home page and a founders page (owner, 2026-10-29).
--
-- Schools gain an optional photo, shown behind the school's card. Until the
-- college uploads one, the card uses a plain branded design. Photos of
-- recognisable people are published only with written consent (NN-08), so a
-- school with a photo cannot be published until consent is confirmed.
--
-- A gallery photo can be chosen as the home page photo, so the hero reuses
-- the gallery's consent rule.
--
-- Founders are the people who founded the college: photo, name, title and
-- their story. A profile is published only with the person's consent to
-- publish it (brief section 17, leadership profiles).

-- +goose Up
ALTER TABLE schools
  ADD COLUMN photo_file_id           uuid    REFERENCES files (id) ON DELETE RESTRICT,
  ADD COLUMN photo_alt               text    NOT NULL DEFAULT '',
  ADD COLUMN photo_consent_confirmed boolean NOT NULL DEFAULT false,
  ADD CONSTRAINT schools_photo_alt_check CHECK (photo_file_id IS NULL OR length(trim(photo_alt)) > 0),
  ADD CONSTRAINT schools_photo_consent_check
    CHECK (status <> 'published' OR photo_file_id IS NULL OR photo_consent_confirmed);

ALTER TABLE gallery_items ADD COLUMN home_hero boolean NOT NULL DEFAULT false;

CREATE TABLE founders (
  id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  name               text        NOT NULL CHECK (length(trim(name)) > 0),
  role_title         text        NOT NULL DEFAULT '',
  -- One or two sentences for the home page.
  intro              text        NOT NULL DEFAULT '',
  -- Plain text with the same rules as pages (blank line between paragraphs).
  story              text        NOT NULL DEFAULT '',
  photo_file_id      uuid        REFERENCES files (id) ON DELETE RESTRICT,
  photo_alt          text        NOT NULL DEFAULT '',
  consent_confirmed  boolean     NOT NULL DEFAULT false,
  consent_note       text        NOT NULL DEFAULT '',
  sort_order         integer     NOT NULL DEFAULT 0,
  status             text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
  published_at       timestamptz,
  is_sample          boolean     NOT NULL DEFAULT false,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  created_by         uuid        REFERENCES users (id) ON DELETE RESTRICT,
  updated_by         uuid        REFERENCES users (id) ON DELETE RESTRICT,
  CHECK (photo_file_id IS NULL OR length(trim(photo_alt)) > 0),
  CHECK (status <> 'published' OR published_at IS NOT NULL),
  CHECK (status <> 'published' OR (consent_confirmed AND length(trim(story)) > 0))
);
CREATE TRIGGER founders_updated_at BEFORE UPDATE ON founders FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE founders;
ALTER TABLE gallery_items DROP COLUMN home_hero;
ALTER TABLE schools
  DROP CONSTRAINT schools_photo_consent_check,
  DROP CONSTRAINT schools_photo_alt_check,
  DROP COLUMN photo_consent_confirmed,
  DROP COLUMN photo_alt,
  DROP COLUMN photo_file_id;
