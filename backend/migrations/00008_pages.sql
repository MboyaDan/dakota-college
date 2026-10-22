-- Text pages (Stage 7): student life, alumni, how to apply and the legal
-- pages are written and approved in the staff portal like other content
-- (owner, 2026-10-22). The website shows a page at a fixed address once a
-- page with that slug is published.
--
-- Enquiries gain an "alumni" type for transcript and certificate requests.

-- +goose Up
CREATE TABLE pages (
  id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  slug             text        NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  title            text        NOT NULL,
  summary          text        NOT NULL DEFAULT '',
  body             text        NOT NULL DEFAULT '',
  seo_title        text        NOT NULL DEFAULT '',
  seo_description  text        NOT NULL DEFAULT '',
  status           text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
  published_at     timestamptz,
  is_sample        boolean     NOT NULL DEFAULT false,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  created_by       uuid        REFERENCES users (id) ON DELETE RESTRICT,
  updated_by       uuid        REFERENCES users (id) ON DELETE RESTRICT,
  CHECK (status <> 'published' OR published_at IS NOT NULL),
  CHECK (status <> 'published' OR length(trim(body)) > 0)
);
CREATE TRIGGER pages_updated_at BEFORE UPDATE ON pages FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE enquiries DROP CONSTRAINT enquiries_query_type_check;
ALTER TABLE enquiries ADD CONSTRAINT enquiries_query_type_check
  CHECK (query_type IN ('admissions', 'programmes', 'fees', 'student_life', 'alumni', 'other'));

-- +goose Down
-- Alumni enquiries become "other" so the older rule can be restored.
UPDATE enquiries SET query_type = 'other' WHERE query_type = 'alumni';
ALTER TABLE enquiries DROP CONSTRAINT enquiries_query_type_check;
ALTER TABLE enquiries ADD CONSTRAINT enquiries_query_type_check
  CHECK (query_type IN ('admissions', 'programmes', 'fees', 'student_life', 'other'));
DROP TABLE pages;
