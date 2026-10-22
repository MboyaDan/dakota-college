-- name: GetUserForLogin :one
SELECT id, email, full_name, kind, status, password_hash, failed_login_count, locked_until
FROM users WHERE email = @email;

-- name: RecordLoginFailure :one
-- Counts a failed password and locks the account once the count reaches
-- @max_failures (lockout, brief 5.1).
UPDATE users
SET failed_login_count = failed_login_count + 1,
    locked_until = CASE WHEN failed_login_count + 1 >= @max_failures::int THEN @lock_until::timestamptz ELSE locked_until END
WHERE id = @id
RETURNING failed_login_count, locked_until;

-- name: RecordLoginSuccess :exec
UPDATE users SET failed_login_count = 0, locked_until = NULL, last_login_at = now() WHERE id = @id;

-- name: CreateSession :one
INSERT INTO sessions (token_hash, user_id, expires_at, ip, user_agent)
VALUES (@token_hash, @user_id, @expires_at, @ip, @user_agent)
RETURNING id;

-- name: GetSessionByTokenHash :one
SELECT s.id, s.user_id, s.created_at, s.last_seen_at, s.expires_at, s.revoked_at,
       u.email, u.full_name, u.kind, u.status
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = @token_hash;

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = now() WHERE id = @id;

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = now() WHERE id = @id AND revoked_at IS NULL;

-- name: RevokeUserSessions :execrows
UPDATE sessions SET revoked_at = now() WHERE user_id = @user_id AND revoked_at IS NULL;

-- name: CreatePasswordReset :exec
INSERT INTO password_resets (user_id, token_hash, expires_at, requested_ip)
VALUES (@user_id, @token_hash, @expires_at, @requested_ip);

-- name: GetPasswordResetForUpdate :one
SELECT pr.id, pr.user_id, pr.expires_at, pr.used_at, u.status
FROM password_resets pr
JOIN users u ON u.id = pr.user_id
WHERE pr.token_hash = @token_hash
FOR UPDATE OF pr;

-- name: UseUserPasswordResets :exec
-- Marks every outstanding reset link of the user as used: one reset spends them all.
UPDATE password_resets SET used_at = now() WHERE user_id = @user_id AND used_at IS NULL;

-- name: SetUserPassword :exec
UPDATE users
SET password_hash = @password_hash, password_changed_at = now(), failed_login_count = 0, locked_until = NULL
WHERE id = @id;
