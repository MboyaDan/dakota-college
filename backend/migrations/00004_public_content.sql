-- Public website content (brief sections 6 and 10). Every content table has:
--   status      draft | published | archived (publishing needs content.publish)
--   is_sample   true only for rows made by the dev seed; launch removes them (AC-17)
-- Rules that protect honesty are enforced here as well as in the API:
--   statistics and accreditations cannot be published without a verification
--   date (NN-01, AC-16); gallery items cannot be published without consent (NN-08).

-- +goose Up
CREATE TABLE schools (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  slug          text        NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  name          text        NOT NULL,
  short_name    text        NOT NULL,
  accent_key    text        NOT NULL CHECK (accent_key IN ('medical', 'hospitality', 'ict', 'business', 'pool')),
  summary       text        NOT NULL DEFAULT '',
  description   text        NOT NULL DEFAULT '',
  head_message  text        NOT NULL DEFAULT '',
  sort_order    integer     NOT NULL DEFAULT 0,
  status        text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
  published_at  timestamptz,
  is_sample     boolean     NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  created_by    uuid        REFERENCES users (id) ON DELETE RESTRICT,
  updated_by    uuid        REFERENCES users (id) ON DELETE RESTRICT,
  CHECK (status <> 'published' OR published_at IS NOT NULL)
);
CREATE TRIGGER schools_updated_at BEFORE UPDATE ON schools FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE programmes (
  id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  school_id         uuid        NOT NULL REFERENCES schools (id) ON DELETE RESTRICT,
  slug              text        NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  title             text        NOT NULL,
  level             text        NOT NULL
                      CHECK (level IN ('short_course', 'artisan', 'certificate', 'diploma', 'higher_diploma')),
  mode              text        NOT NULL DEFAULT 'full_time' CHECK (mode IN ('full_time', 'part_time', 'blended')),
  duration_months   integer     CHECK (duration_months BETWEEN 1 AND 72),
  duration_note     text        NOT NULL DEFAULT '',
  -- Minimum KCSE mean grade, if the programme has one (course finder filter).
  min_kcse_grade    text        CHECK (min_kcse_grade IN ('A', 'A-', 'B+', 'B', 'B-', 'C+', 'C', 'C-', 'D+', 'D', 'D-', 'E')),
  summary           text        NOT NULL DEFAULT '',
  overview          text        NOT NULL DEFAULT '',
  what_you_learn    text        NOT NULL DEFAULT '',
  career_outcomes   text        NOT NULL DEFAULT '',
  assessment_model  text        NOT NULL DEFAULT 'marks' CHECK (assessment_model IN ('marks', 'competency', 'both')),
  seo_title         text        NOT NULL DEFAULT '',
  seo_description   text        NOT NULL DEFAULT '',
  sort_order        integer     NOT NULL DEFAULT 0,
  status            text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
  published_at      timestamptz,
  is_sample         boolean     NOT NULL DEFAULT false,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  created_by        uuid        REFERENCES users (id) ON DELETE RESTRICT,
  updated_by        uuid        REFERENCES users (id) ON DELETE RESTRICT,
  CHECK (status <> 'published' OR published_at IS NOT NULL)
);
CREATE INDEX programmes_school_idx ON programmes (school_id);
CREATE INDEX programmes_published_idx ON programmes (level, sort_order) WHERE status = 'published';
CREATE TRIGGER programmes_updated_at BEFORE UPDATE ON programmes FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE programme_requirements (
  id            uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
  programme_id  uuid    NOT NULL REFERENCES programmes (id) ON DELETE CASCADE,
  kind          text    NOT NULL DEFAULT 'academic' CHECK (kind IN ('academic', 'document', 'other')),
  requirement   text    NOT NULL CHECK (length(trim(requirement)) > 0),
  sort_order    integer NOT NULL DEFAULT 0
);
CREATE INDEX programme_requirements_programme_idx ON programme_requirements (programme_id, sort_order);

CREATE TABLE programme_intakes (
  id                     uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  programme_id           uuid        NOT NULL REFERENCES programmes (id) ON DELETE RESTRICT,
  name                   text        NOT NULL,
  starts_on              date        NOT NULL,
  applications_open_on   date,
  application_deadline   date,
  capacity               integer     CHECK (capacity > 0),
  status                 text        NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'open', 'closed')),
  is_sample              boolean     NOT NULL DEFAULT false,
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  CHECK (application_deadline IS NULL OR application_deadline <= starts_on),
  CHECK (applications_open_on IS NULL OR application_deadline IS NULL OR applications_open_on <= application_deadline),
  UNIQUE (programme_id, starts_on)
);
CREATE INDEX programme_intakes_starts_idx ON programme_intakes (starts_on);
CREATE TRIGGER programme_intakes_updated_at BEFORE UPDATE ON programme_intakes FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Published fee lines. Money is integer cents of KES (ADR-05, NN-03).
CREATE TABLE fee_items (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  programme_id  uuid        NOT NULL REFERENCES programmes (id) ON DELETE CASCADE,
  intake_id     uuid        REFERENCES programme_intakes (id) ON DELETE CASCADE,
  label         text        NOT NULL,
  category      text        NOT NULL CHECK (category IN ('tuition', 'registration', 'assessment', 'caution', 'other')),
  amount_cents  bigint      NOT NULL CHECK (amount_cents >= 0),
  currency      text        NOT NULL DEFAULT 'KES' CHECK (currency = 'KES'),
  frequency     text        NOT NULL CHECK (frequency IN ('once', 'per_term', 'per_year', 'whole_programme')),
  notes         text        NOT NULL DEFAULT '',
  sort_order    integer     NOT NULL DEFAULT 0,
  is_sample     boolean     NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX fee_items_programme_idx ON fee_items (programme_id, sort_order);
CREATE TRIGGER fee_items_updated_at BEFORE UPDATE ON fee_items FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE news_posts (
  id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  slug            text        NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  title           text        NOT NULL,
  summary         text        NOT NULL DEFAULT '',
  body            text        NOT NULL DEFAULT '',
  cover_file_id   uuid        REFERENCES files (id) ON DELETE RESTRICT,
  cover_alt       text        NOT NULL DEFAULT '',
  seo_title       text        NOT NULL DEFAULT '',
  seo_description text        NOT NULL DEFAULT '',
  status          text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
  published_at    timestamptz,
  is_sample       boolean     NOT NULL DEFAULT false,
  author_id       uuid        REFERENCES users (id) ON DELETE RESTRICT,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CHECK (status <> 'published' OR published_at IS NOT NULL),
  CHECK (cover_file_id IS NULL OR length(trim(cover_alt)) > 0)
);
CREATE INDEX news_posts_published_idx ON news_posts (published_at DESC) WHERE status = 'published';
CREATE TRIGGER news_posts_updated_at BEFORE UPDATE ON news_posts FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE events (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  slug          text        NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  title         text        NOT NULL,
  summary       text        NOT NULL DEFAULT '',
  body          text        NOT NULL DEFAULT '',
  location      text        NOT NULL DEFAULT '',
  starts_at     timestamptz NOT NULL,
  ends_at       timestamptz,
  status        text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
  published_at  timestamptz,
  is_sample     boolean     NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  CHECK (ends_at IS NULL OR ends_at >= starts_at),
  CHECK (status <> 'published' OR published_at IS NOT NULL)
);
CREATE INDEX events_starts_idx ON events (starts_at) WHERE status = 'published';
CREATE TRIGGER events_updated_at BEFORE UPDATE ON events FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE faq_categories (
  id          uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
  slug        text    NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  name        text    NOT NULL,
  sort_order  integer NOT NULL DEFAULT 0,
  is_sample   boolean NOT NULL DEFAULT false
);

CREATE TABLE faqs (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  category_id   uuid        NOT NULL REFERENCES faq_categories (id) ON DELETE RESTRICT,
  programme_id  uuid        REFERENCES programmes (id) ON DELETE CASCADE,
  question      text        NOT NULL,
  answer        text        NOT NULL,
  sort_order    integer     NOT NULL DEFAULT 0,
  status        text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
  published_at  timestamptz,
  is_sample     boolean     NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  -- Full-text search for the FAQ page (brief 6.2).
  search        tsvector    GENERATED ALWAYS AS (to_tsvector('english', question || ' ' || answer)) STORED,
  CHECK (status <> 'published' OR published_at IS NOT NULL)
);
CREATE INDEX faqs_category_idx ON faqs (category_id, sort_order);
CREATE INDEX faqs_search_idx ON faqs USING gin (search);
CREATE TRIGGER faqs_updated_at BEFORE UPDATE ON faqs FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Announcement bar entries, with an expiry (brief 5.1).
CREATE TABLE banners (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  message       text        NOT NULL CHECK (length(message) BETWEEN 1 AND 200),
  link_label    text,
  link_href     text,
  starts_at     timestamptz NOT NULL DEFAULT now(),
  ends_at       timestamptz NOT NULL,
  status        text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
  is_sample     boolean     NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  CHECK (ends_at > starts_at),
  CHECK ((link_label IS NULL) = (link_href IS NULL))
);
CREATE INDEX banners_active_idx ON banners (starts_at, ends_at) WHERE status = 'published';
CREATE TRIGGER banners_updated_at BEFORE UPDATE ON banners FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE gallery_items (
  id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  file_id            uuid        NOT NULL REFERENCES files (id) ON DELETE RESTRICT,
  caption            text        NOT NULL DEFAULT '',
  alt_text           text        NOT NULL CHECK (length(trim(alt_text)) > 0),
  -- Photos of people are published only with written consent (NN-08).
  consent_confirmed  boolean     NOT NULL DEFAULT false,
  consent_note       text        NOT NULL DEFAULT '',
  sort_order         integer     NOT NULL DEFAULT 0,
  status             text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
  is_sample          boolean     NOT NULL DEFAULT false,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  CHECK (status <> 'published' OR consent_confirmed)
);
CREATE TRIGGER gallery_items_updated_at BEFORE UPDATE ON gallery_items FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- "Why Dakota" figures. Shown only when verified (NN-01, AC-16).
CREATE TABLE statistics (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  label         text        NOT NULL,
  value         text        NOT NULL,
  source_note   text        NOT NULL DEFAULT '',
  verified_on   date,
  verified_by   uuid        REFERENCES users (id) ON DELETE RESTRICT,
  sort_order    integer     NOT NULL DEFAULT 0,
  status        text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
  is_sample     boolean     NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  CHECK (status <> 'published' OR (verified_on IS NOT NULL AND length(trim(source_note)) > 0))
);
CREATE TRIGGER statistics_updated_at BEFORE UPDATE ON statistics FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Registration and accreditation (brief 6.1 About, DQ-04, D-34). Same rule as statistics.
CREATE TABLE accreditations (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  body          text        NOT NULL,
  reference     text        NOT NULL,
  description   text        NOT NULL DEFAULT '',
  evidence_note text        NOT NULL DEFAULT '',
  verified_on   date,
  verified_by   uuid        REFERENCES users (id) ON DELETE RESTRICT,
  sort_order    integer     NOT NULL DEFAULT 0,
  status        text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
  is_sample     boolean     NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  CHECK (status <> 'published' OR (verified_on IS NOT NULL AND length(trim(evidence_note)) > 0))
);
CREATE TRIGGER accreditations_updated_at BEFORE UPDATE ON accreditations FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Site-wide settings. Keys are created here with empty values; the UI hides
-- anything empty (DQ-01). The dev seed fills obviously fake sample values.
CREATE TABLE site_settings (
  key          text        PRIMARY KEY CHECK (key ~ '^[a-z][a-z_]*(\.[a-z_]+)+$'),
  value        text        NOT NULL DEFAULT '',
  description  text        NOT NULL,
  updated_at   timestamptz NOT NULL DEFAULT now(),
  updated_by   uuid        REFERENCES users (id) ON DELETE RESTRICT
);
CREATE TRIGGER site_settings_updated_at BEFORE UPDATE ON site_settings FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO site_settings (key, description) VALUES
  ('contact.phone',          'Main phone number, shown in the footer and contact page'),
  ('contact.email',          'General enquiries email address'),
  ('contact.admissions_email', 'Admissions office email; enquiries about admissions go here'),
  ('contact.finance_email',  'Finance office email; enquiries about fees go here'),
  ('contact.whatsapp',       'WhatsApp number for click-to-chat, in international format'),
  ('contact.address',        'Postal or physical address'),
  ('contact.office_hours',   'Office hours, in plain words'),
  ('campus.map_url',         'Link to the campus on a map'),
  ('campus.directions',      'How to get to the campus'),
  ('campus.visiting_hours',  'Visiting hours for the campus'),
  ('about.motto',            'Motto, in the college''s own words'),
  ('about.mission',          'Mission statement, in the college''s own words'),
  ('about.vision',           'Vision statement, in the college''s own words'),
  ('social.facebook',        'Facebook page URL'),
  ('social.instagram',       'Instagram profile URL'),
  ('social.x',               'X (Twitter) profile URL'),
  ('social.youtube',         'YouTube channel URL'),
  ('student_number.format',  'Student number format; default DTC/YYYY/NNNN (DQ-08)'),
  ('applications.draft_expiry_days', 'Days before an unsubmitted application draft expires');

UPDATE site_settings SET value = 'DTC/{YYYY}/{NNNN}' WHERE key = 'student_number.format';
UPDATE site_settings SET value = '60' WHERE key = 'applications.draft_expiry_days';

-- +goose Down
DROP TABLE site_settings;
DROP TABLE accreditations;
DROP TABLE statistics;
DROP TABLE gallery_items;
DROP TABLE banners;
DROP TABLE faqs;
DROP TABLE faq_categories;
DROP TABLE events;
DROP TABLE news_posts;
DROP TABLE fee_items;
DROP TABLE programme_intakes;
DROP TABLE programme_requirements;
DROP TABLE programmes;
DROP TABLE schools;
