-- Results (Stage 13a, brief 7 and 10, AC-09). A class (offering) has one
-- result sheet: the trainer enters marks, submits; the Head of School
-- approves or returns; the registrar releases. Students see only released
-- snapshots. Marks, sheet events and releases are append-only: a correction
-- is a new mark row with a reason, and needs approval and release again.

-- +goose Up
-- How a programme assesses its units (DQ-07). No row means the default:
-- marks, CAT 30% and exam 70%, pass mark 40, bands A 70, B 60, C 50, D 40.
CREATE TABLE assessment_schemes (
  programme_id  uuid        PRIMARY KEY REFERENCES programmes (id) ON DELETE CASCADE,
  kind          text        NOT NULL CHECK (kind IN ('marks', 'competency')),
  -- [{"key": "cat", "name": "Continuous assessment", "weight": 30}, ...]
  components    jsonb       NOT NULL CHECK (jsonb_typeof(components) = 'array'),
  pass_mark     smallint    NOT NULL DEFAULT 40 CHECK (pass_mark BETWEEN 0 AND 100),
  -- [{"grade": "A", "min": 70}, ...], highest first
  grade_bands   jsonb       NOT NULL CHECK (jsonb_typeof(grade_bands) = 'array'),
  updated_by    uuid        REFERENCES users (id) ON DELETE RESTRICT,
  updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE result_sheets (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  offering_id   uuid        NOT NULL UNIQUE REFERENCES unit_offerings (id) ON DELETE RESTRICT,
  status        text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'submitted', 'approved', 'released')),
  -- The scheme in force for this sheet, fixed once marks are entered.
  scheme        jsonb       NOT NULL CHECK (jsonb_typeof(scheme) = 'object'),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX result_sheets_status_idx ON result_sheets (status);
CREATE TRIGGER result_sheets_updated_at BEFORE UPDATE ON result_sheets FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Every status change of a sheet: who, when, why. Append-only.
CREATE TABLE result_sheet_events (
  id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  sheet_id     uuid        NOT NULL REFERENCES result_sheets (id) ON DELETE RESTRICT,
  from_status  text        NOT NULL,
  to_status    text        NOT NULL,
  reason       text        NOT NULL DEFAULT '',
  actor_id     uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX result_sheet_events_sheet_idx ON result_sheet_events (sheet_id, created_at);
CREATE TRIGGER result_sheet_events_append_only BEFORE UPDATE OR DELETE ON result_sheet_events
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();

-- Marks, one row per entry; the latest row per student and component is
-- the current mark. Append-only (NN-05).
CREATE TABLE marks (
  id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  sheet_id     uuid        NOT NULL REFERENCES result_sheets (id) ON DELETE RESTRICT,
  student_id   uuid        NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
  component    text        NOT NULL CHECK (component ~ '^[a-z][a-z0-9_]{0,29}$'),
  score        smallint    CHECK (score BETWEEN 0 AND 100),
  outcome      text        CHECK (outcome IN ('competent', 'not_yet_competent')),
  -- Required for a correction after approval.
  reason       text        NOT NULL DEFAULT '' CHECK (length(reason) <= 500),
  entered_by   uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  entered_at   timestamptz NOT NULL DEFAULT now(),
  CHECK ((score IS NULL) <> (outcome IS NULL))
);
CREATE INDEX marks_current_idx ON marks (sheet_id, student_id, component, id DESC);
CREATE TRIGGER marks_append_only BEFORE UPDATE OR DELETE ON marks
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();

-- What students see: each release freezes the computed results. Append-only;
-- the newest release of a sheet is the one shown.
CREATE TABLE result_releases (
  id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  sheet_id     uuid        NOT NULL REFERENCES result_sheets (id) ON DELETE RESTRICT,
  version      integer     NOT NULL CHECK (version >= 1),
  -- [{"student_id": ..., "total": 72, "grade": "A", "passed": true, "outcome": null, "components": {...}}]
  results      jsonb       NOT NULL CHECK (jsonb_typeof(results) = 'array'),
  released_by  uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  released_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (sheet_id, version)
);
CREATE TRIGGER result_releases_append_only BEFORE UPDATE OR DELETE ON result_releases
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();

-- Official transcripts issued by the registrar's office: numbered (gapless,
-- per year), kept as a private PDF.
CREATE TABLE transcripts (
  id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  number              text        NOT NULL UNIQUE,
  student_id          uuid        NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
  file_id             uuid        NOT NULL REFERENCES files (id) ON DELETE RESTRICT,
  service_request_id  uuid        REFERENCES service_requests (id) ON DELETE RESTRICT,
  issued_by           uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  issued_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX transcripts_student_idx ON transcripts (student_id, issued_at DESC);

-- +goose Down
DROP TABLE transcripts;
DROP TABLE result_releases;
DROP TABLE marks;
DROP TABLE result_sheet_events;
DROP TABLE result_sheets;
DROP TABLE assessment_schemes;
