-- Student records and academics, core (Stage 11a): terms, cohorts, units,
-- programme units, unit offerings with trainers and a timetable, unit
-- registration, guardians and student status history.
--
-- Access (AC-08, D-133): trainers lose students.view and get academics.teach,
-- which shows only the offerings they are assigned to. Work across every
-- school needs schools.all; without it, students.view and academics.manage
-- reach only the schools the person is assigned to (Heads of School).

-- +goose Up
INSERT INTO permissions (key, module, description) VALUES
  ('academics.teach',  'academics', 'See your assigned units, their timetable and class lists'),
  ('academics.manage', 'academics', 'Manage units, cohorts, terms, timetables, trainer assignments and unit registration'),
  ('schools.all',      'students',  'Work across all schools; without it, student and academic access is limited to your assigned schools');

INSERT INTO role_permissions (role_id, permission_key)
SELECT r.id, g.permission_key
FROM (VALUES
  ('super_admin', 'academics.teach'), ('super_admin', 'academics.manage'), ('super_admin', 'schools.all'),
  ('trainer', 'academics.teach'),
  ('registrar', 'academics.manage'), ('registrar', 'schools.all'),
  ('head_of_school', 'academics.manage'),
  ('principal', 'schools.all'), ('finance_officer', 'schools.all')
) AS g (role_key, permission_key)
JOIN roles r ON r.key = g.role_key
ON CONFLICT DO NOTHING;

DELETE FROM role_permissions
WHERE permission_key = 'students.view' AND role_id = (SELECT id FROM roles WHERE key = 'trainer');

-- Which schools a member of staff works in (Heads of School).
CREATE TABLE school_assignments (
  user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  school_id   uuid        NOT NULL REFERENCES schools (id) ON DELETE CASCADE,
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, school_id)
);
CREATE INDEX school_assignments_school_idx ON school_assignments (school_id);

-- Academic periods, set by the registrar (D-132).
CREATE TABLE academic_terms (
  id                      uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  name                    text        NOT NULL UNIQUE CHECK (length(trim(name)) BETWEEN 1 AND 80),
  starts_on               date        NOT NULL,
  ends_on                 date        NOT NULL,
  -- Window for students to register for units themselves (Stage 12).
  registration_opens_on   date,
  registration_closes_on  date,
  is_sample               boolean     NOT NULL DEFAULT false,
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),
  CHECK (ends_on > starts_on),
  CHECK (registration_opens_on IS NULL OR registration_closes_on IS NULL OR registration_closes_on >= registration_opens_on)
);
CREATE TRIGGER academic_terms_updated_at BEFORE UPDATE ON academic_terms FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- A cohort is the group of students who started a programme together.
CREATE TABLE cohorts (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  programme_id  uuid        NOT NULL REFERENCES programmes (id) ON DELETE RESTRICT,
  intake_id     uuid        REFERENCES programme_intakes (id) ON DELETE RESTRICT,
  name          text        NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 120),
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (programme_id, name)
);
-- Enrolment files students into the cohort of their programme and intake.
CREATE UNIQUE INDEX cohorts_intake_idx ON cohorts (programme_id, intake_id) WHERE intake_id IS NOT NULL;

ALTER TABLE students ADD COLUMN cohort_id uuid REFERENCES cohorts (id) ON DELETE RESTRICT;
CREATE INDEX students_cohort_idx ON students (cohort_id);

CREATE TABLE units (
  id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  school_id    uuid        NOT NULL REFERENCES schools (id) ON DELETE RESTRICT,
  code         text        NOT NULL UNIQUE CHECK (code ~ '^[A-Z0-9][A-Z0-9 /-]{1,19}$'),
  title        text        NOT NULL CHECK (length(trim(title)) BETWEEN 1 AND 160),
  description  text        NOT NULL DEFAULT '' CHECK (length(description) <= 2000),
  hours        integer     CHECK (hours IS NULL OR hours BETWEEN 1 AND 2000),
  active       boolean     NOT NULL DEFAULT true,
  is_sample    boolean     NOT NULL DEFAULT false,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX units_school_idx ON units (school_id, code);
CREATE TRIGGER units_updated_at BEFORE UPDATE ON units FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- The units of a programme, by stage (year or level of study).
CREATE TABLE programme_units (
  programme_id  uuid     NOT NULL REFERENCES programmes (id) ON DELETE CASCADE,
  unit_id       uuid     NOT NULL REFERENCES units (id) ON DELETE RESTRICT,
  stage         smallint NOT NULL DEFAULT 1 CHECK (stage BETWEEN 1 AND 10),
  is_core       boolean  NOT NULL DEFAULT true,
  sort_order    integer  NOT NULL DEFAULT 0,
  PRIMARY KEY (programme_id, unit_id)
);
CREATE INDEX programme_units_unit_idx ON programme_units (unit_id);

-- A unit taught in a term: it has trainers, a timetable and a class list.
CREATE TABLE unit_offerings (
  id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  term_id     uuid        NOT NULL REFERENCES academic_terms (id) ON DELETE RESTRICT,
  unit_id     uuid        NOT NULL REFERENCES units (id) ON DELETE RESTRICT,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (term_id, unit_id)
);
CREATE INDEX unit_offerings_unit_idx ON unit_offerings (unit_id);

CREATE TABLE trainer_assignments (
  offering_id  uuid        NOT NULL REFERENCES unit_offerings (id) ON DELETE CASCADE,
  user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  assigned_by  uuid        REFERENCES users (id) ON DELETE RESTRICT,
  created_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (offering_id, user_id)
);
CREATE INDEX trainer_assignments_user_idx ON trainer_assignments (user_id);

-- Registration of a student for an offering. Dropping keeps the row.
CREATE TABLE unit_registrations (
  id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  offering_id    uuid        NOT NULL REFERENCES unit_offerings (id) ON DELETE RESTRICT,
  student_id     uuid        NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
  status         text        NOT NULL DEFAULT 'registered' CHECK (status IN ('registered', 'dropped')),
  registered_at  timestamptz NOT NULL DEFAULT now(),
  registered_by  uuid        REFERENCES users (id) ON DELETE RESTRICT,
  dropped_at     timestamptz,
  dropped_by     uuid        REFERENCES users (id) ON DELETE RESTRICT,
  UNIQUE (offering_id, student_id),
  CHECK ((status = 'dropped') = (dropped_at IS NOT NULL))
);
CREATE INDEX unit_registrations_student_idx ON unit_registrations (student_id);

-- Weekly classes (by weekday) and exams (by date).
CREATE TABLE timetable_entries (
  id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  offering_id  uuid        NOT NULL REFERENCES unit_offerings (id) ON DELETE CASCADE,
  kind         text        NOT NULL CHECK (kind IN ('class', 'exam')),
  weekday      smallint    CHECK (weekday BETWEEN 1 AND 7), -- 1 Monday
  on_date      date,
  starts_at    time        NOT NULL,
  ends_at      time        NOT NULL,
  room         text        NOT NULL DEFAULT '' CHECK (length(room) <= 60),
  created_at   timestamptz NOT NULL DEFAULT now(),
  CHECK (ends_at > starts_at),
  CHECK ((kind = 'class' AND weekday IS NOT NULL AND on_date IS NULL) OR (kind = 'exam' AND on_date IS NOT NULL AND weekday IS NULL))
);
CREATE INDEX timetable_entries_offering_idx ON timetable_entries (offering_id);

-- Parents, guardians and next of kin.
CREATE TABLE student_guardians (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  student_id    uuid        NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
  full_name     text        NOT NULL CHECK (length(trim(full_name)) BETWEEN 1 AND 120),
  relationship  text        NOT NULL CHECK (relationship IN ('mother', 'father', 'guardian', 'other_relative', 'spouse', 'other')),
  phone         text        NOT NULL DEFAULT '',
  email         citext,
  is_primary    boolean     NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX student_guardians_student_idx ON student_guardians (student_id);
CREATE TRIGGER student_guardians_updated_at BEFORE UPDATE ON student_guardians FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Every change of a student's status: who, when, why. Append-only.
CREATE TABLE student_status_history (
  id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  student_id   uuid        NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
  from_status  text,
  to_status    text        NOT NULL,
  reason       text        NOT NULL DEFAULT '',
  changed_by   uuid        REFERENCES users (id) ON DELETE RESTRICT,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX student_status_history_student_idx ON student_status_history (student_id, created_at);
CREATE TRIGGER student_status_history_append_only BEFORE UPDATE OR DELETE ON student_status_history
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();

-- Existing students (enrolled in Stage 10): file them into cohorts, copy the
-- guardian from their application and start their status history.
INSERT INTO cohorts (programme_id, intake_id, name)
SELECT DISTINCT s.programme_id, s.intake_id, left(p.title || ' · ' || COALESCE(i.name, 'no intake'), 120)
FROM students s
JOIN programmes p ON p.id = s.programme_id
LEFT JOIN programme_intakes i ON i.id = s.intake_id
WHERE s.intake_id IS NOT NULL
ON CONFLICT DO NOTHING;
UPDATE students s SET cohort_id = c.id
FROM cohorts c WHERE c.programme_id = s.programme_id AND c.intake_id = s.intake_id;
INSERT INTO student_guardians (student_id, full_name, relationship, phone, email, is_primary)
SELECT s.id, g.full_name, g.relationship, g.phone, g.email, true
FROM students s JOIN application_guardians g ON g.application_id = s.application_id
WHERE g.relationship IN ('mother', 'father', 'guardian', 'other_relative', 'other');
INSERT INTO student_status_history (student_id, to_status, reason, changed_by, created_at)
SELECT id, status, 'Enrolled', enrolled_by, enrolled_at FROM students;

-- +goose Down
DROP TABLE student_status_history;
DROP TABLE student_guardians;
DROP TABLE timetable_entries;
DROP TABLE unit_registrations;
DROP TABLE trainer_assignments;
DROP TABLE unit_offerings;
DROP TABLE programme_units;
DROP TABLE units;
DROP INDEX students_cohort_idx;
ALTER TABLE students DROP COLUMN cohort_id;
DROP TABLE cohorts;
DROP TABLE academic_terms;
DROP TABLE school_assignments;
INSERT INTO role_permissions (role_id, permission_key)
SELECT id, 'students.view' FROM roles WHERE key = 'trainer'
ON CONFLICT DO NOTHING;
DELETE FROM role_permissions WHERE permission_key IN ('academics.teach', 'academics.manage', 'schools.all');
DELETE FROM permissions WHERE key IN ('academics.teach', 'academics.manage', 'schools.all');
