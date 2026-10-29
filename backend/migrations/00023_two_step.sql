-- Optional two-step sign-in for staff (Stage 17b, brief 13). A staff member
-- who turns it on signs in with their password and then a six-digit code
-- from an authenticator app, or a single-use recovery code (D-206).

-- +goose Up
-- One authenticator per person. The shared secret is stored encrypted with
-- TOTP_KEY (AES-256-GCM), never in plain text. Until confirmed_at is set the
-- person is still setting it up and sign-in does not ask for a code.
-- last_step is the newest 30-second step accepted, so a code cannot be used
-- twice.
CREATE TABLE user_totp (
  user_id       uuid        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
  secret_sealed bytea       NOT NULL,
  confirmed_at  timestamptz,
  last_step     bigint      NOT NULL DEFAULT 0,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER user_totp_updated_at BEFORE UPDATE ON user_totp
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Recovery codes are shown once; only their SHA-256 hash is kept.
CREATE TABLE user_recovery_codes (
  id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  code_hash  bytea       NOT NULL UNIQUE,
  used_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_recovery_codes_user_idx ON user_recovery_codes (user_id) WHERE used_at IS NULL;

-- A password that was right, waiting for the code. The token lives in a
-- short-lived HttpOnly cookie; only its hash is stored. It is spent by one
-- successful code and dies after a few wrong ones or a few minutes.
CREATE TABLE login_challenges (
  id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  token_hash bytea       NOT NULL UNIQUE,
  user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  attempts   integer     NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  used_at    timestamptz,
  ip         inet,
  user_agent text,
  CHECK (expires_at > created_at)
);
CREATE INDEX login_challenges_user_idx ON login_challenges (user_id);

-- +goose Down
DROP TABLE login_challenges;
DROP TABLE user_recovery_codes;
DROP TABLE user_totp;
