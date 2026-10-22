-- Identity and access (brief sections 10 and 11). Roles and permissions are
-- production data and are created here; users are not (the first Super Admin
-- comes from the setup command in Stage 4).

-- +goose Up
CREATE TABLE users (
  id                   uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  email                citext      NOT NULL UNIQUE,
  full_name            text        NOT NULL CHECK (length(trim(full_name)) > 0),
  phone                text,
  kind                 text        NOT NULL CHECK (kind IN ('staff', 'student', 'applicant')),
  -- Soft deactivation instead of deleting people (brief section 10).
  status               text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
  -- Argon2id PHC string. NULL means no password set yet (invited, or magic link only).
  password_hash        text,
  password_changed_at  timestamptz,
  failed_login_count   integer     NOT NULL DEFAULT 0 CHECK (failed_login_count >= 0),
  locked_until         timestamptz,
  last_login_at        timestamptz,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER users_updated_at BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE permissions (
  key          text PRIMARY KEY CHECK (key ~ '^[a-z]+(\.[a-z_]+)+$'),
  module       text NOT NULL,
  description  text NOT NULL
);

CREATE TABLE roles (
  id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  key          text        NOT NULL UNIQUE CHECK (key ~ '^[a-z][a-z_]*$'),
  name         text        NOT NULL,
  description  text        NOT NULL DEFAULT '',
  -- Starter templates from the brief. Editable, but not deletable.
  is_system    boolean     NOT NULL DEFAULT false,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER roles_updated_at BEFORE UPDATE ON roles
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE role_permissions (
  role_id         uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
  permission_key  text NOT NULL REFERENCES permissions (key) ON DELETE RESTRICT,
  PRIMARY KEY (role_id, permission_key)
);
CREATE INDEX role_permissions_permission_idx ON role_permissions (permission_key);

CREATE TABLE user_roles (
  user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  -- RESTRICT: a role in use cannot be deleted (brief 11, D-15).
  role_id     uuid        NOT NULL REFERENCES roles (id) ON DELETE RESTRICT,
  granted_by  uuid        REFERENCES users (id) ON DELETE RESTRICT,
  granted_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, role_id)
);
CREATE INDEX user_roles_role_idx ON user_roles (role_id);

-- Server-side sessions (ADR-04). Only a hash of the cookie token is stored.
CREATE TABLE sessions (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  token_hash    bytea       NOT NULL UNIQUE,
  user_id       uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  created_at    timestamptz NOT NULL DEFAULT now(),
  last_seen_at  timestamptz NOT NULL DEFAULT now(),
  expires_at    timestamptz NOT NULL,
  revoked_at    timestamptz,
  ip            inet,
  user_agent    text,
  CHECK (expires_at > created_at)
);
CREATE INDEX sessions_user_active_idx ON sessions (user_id) WHERE revoked_at IS NULL;

CREATE TABLE password_resets (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  token_hash    bytea       NOT NULL UNIQUE,
  created_at    timestamptz NOT NULL DEFAULT now(),
  expires_at    timestamptz NOT NULL,
  used_at       timestamptz,
  requested_ip  inet,
  CHECK (expires_at > created_at)
);
CREATE INDEX password_resets_user_idx ON password_resets (user_id);

-- Who did what, when (brief 13). Never stores secrets. Append-only.
CREATE TABLE audit_logs (
  id             bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  occurred_at    timestamptz NOT NULL DEFAULT now(),
  actor_user_id  uuid        REFERENCES users (id) ON DELETE RESTRICT,
  action         text        NOT NULL,
  entity_type    text        NOT NULL,
  entity_id      text,
  request_id     text,
  ip             inet,
  user_agent     text,
  details        jsonb       NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX audit_logs_entity_idx ON audit_logs (entity_type, entity_id, occurred_at DESC);
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_user_id, occurred_at DESC);
CREATE TRIGGER audit_logs_append_only BEFORE UPDATE OR DELETE ON audit_logs
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();

-- Permission keys from brief 11, plus enquiries.* (D-35).
INSERT INTO permissions (key, module, description) VALUES
  ('content.read',               'content',    'View content in the staff portal'),
  ('content.write',              'content',    'Create and edit draft content'),
  ('content.publish',            'content',    'Publish and unpublish content'),
  ('enquiries.view',             'enquiries',  'View enquiries'),
  ('enquiries.manage',           'enquiries',  'Assign, update and close enquiries'),
  ('admissions.view',            'admissions', 'View applications'),
  ('admissions.review',          'admissions', 'Review applications and request information'),
  ('admissions.offer',           'admissions', 'Make offers and reject applications'),
  ('admissions.enrol',           'admissions', 'Enrol applicants as students'),
  ('students.view',              'students',   'View student records'),
  ('students.update',            'students',   'Update student records'),
  ('academics.marks.enter',      'academics',  'Enter marks for assigned units'),
  ('academics.results.approve',  'academics',  'Approve results'),
  ('academics.results.release',  'academics',  'Release approved results to students'),
  ('finance.view',               'finance',    'View fees, invoices and payments'),
  ('finance.invoice',            'finance',    'Create invoices'),
  ('finance.payment.confirm',    'finance',    'Confirm payments'),
  ('finance.adjust',             'finance',    'Record compensating adjustments'),
  ('reports.view',               'reports',    'View reports and dashboards'),
  ('reports.export',             'reports',    'Export data'),
  ('users.manage',               'admin',      'Manage staff accounts'),
  ('roles.manage',               'admin',      'Manage roles and their permissions'),
  ('system.admin',               'admin',      'Protected owner capability (Super Admin only)');

INSERT INTO roles (key, name, description, is_system) VALUES
  ('super_admin',        'Super Admin',        'Protected owner role. Holds every permission.', true),
  ('ict_admin',          'ICT Admin',          'Users, roles, settings and audit review.', true),
  ('principal',          'Principal',          'Dashboards, reports and publishing approval.', true),
  ('registrar',          'Registrar',          'Admissions decisions, enrolment, records, publishing approval.', true),
  ('admissions_officer', 'Admissions Officer', 'Reviews applications and enquiries.', true),
  ('finance_officer',    'Finance Officer',    'Fees, invoices, payments and statements.', true),
  ('head_of_school',     'Head of School',     'Approves results and oversees a school.', true),
  ('trainer',            'Trainer',            'Enters marks for assigned units.', true),
  ('content_editor',     'Content Editor',     'Writes website content for approval.', true),
  ('student',            'Student',            'Student portal access to own records only.', true);

-- Starter grants. system.admin goes to super_admin only (AC-12); the API also
-- refuses to grant it through role configuration (Stage 4).
INSERT INTO role_permissions (role_id, permission_key)
SELECT r.id, p.key FROM roles r CROSS JOIN permissions p WHERE r.key = 'super_admin';

INSERT INTO role_permissions (role_id, permission_key)
SELECT r.id, g.permission_key
FROM (VALUES
  ('ict_admin', 'users.manage'), ('ict_admin', 'roles.manage'), ('ict_admin', 'reports.view'),
  ('ict_admin', 'content.read'),
  ('principal', 'content.read'), ('principal', 'content.publish'), ('principal', 'enquiries.view'),
  ('principal', 'admissions.view'), ('principal', 'students.view'), ('principal', 'finance.view'),
  ('principal', 'reports.view'), ('principal', 'reports.export'),
  ('registrar', 'content.read'), ('registrar', 'content.publish'), ('registrar', 'enquiries.view'),
  ('registrar', 'enquiries.manage'), ('registrar', 'admissions.view'), ('registrar', 'admissions.review'),
  ('registrar', 'admissions.offer'), ('registrar', 'admissions.enrol'), ('registrar', 'students.view'),
  ('registrar', 'students.update'), ('registrar', 'academics.results.release'), ('registrar', 'reports.view'),
  ('admissions_officer', 'content.read'), ('admissions_officer', 'enquiries.view'),
  ('admissions_officer', 'enquiries.manage'), ('admissions_officer', 'admissions.view'),
  ('admissions_officer', 'admissions.review'), ('admissions_officer', 'admissions.offer'),
  ('finance_officer', 'finance.view'), ('finance_officer', 'finance.invoice'),
  ('finance_officer', 'finance.payment.confirm'), ('finance_officer', 'finance.adjust'),
  ('finance_officer', 'students.view'), ('finance_officer', 'reports.view'),
  ('head_of_school', 'content.read'), ('head_of_school', 'admissions.view'), ('head_of_school', 'students.view'),
  ('head_of_school', 'academics.results.approve'), ('head_of_school', 'reports.view'),
  ('trainer', 'academics.marks.enter'), ('trainer', 'students.view'),
  ('content_editor', 'content.read'), ('content_editor', 'content.write')
) AS g (role_key, permission_key)
JOIN roles r ON r.key = g.role_key;

-- +goose Down
DROP TABLE audit_logs;
DROP TABLE password_resets;
DROP TABLE sessions;
DROP TABLE user_roles;
DROP TABLE role_permissions;
DROP TABLE roles;
DROP TABLE permissions;
DROP TABLE users;
