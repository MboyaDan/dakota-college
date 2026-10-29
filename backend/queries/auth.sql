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

-- Two-step sign-in (Stage 17b).

-- name: GetUserPasswordHash :one
SELECT password_hash FROM users WHERE id = @id;

-- name: GetUserTOTP :one
SELECT user_id, secret_sealed, confirmed_at, last_step FROM user_totp WHERE user_id = @user_id;

-- name: GetUserTOTPForUpdate :one
SELECT user_id, secret_sealed, confirmed_at, last_step FROM user_totp WHERE user_id = @user_id FOR UPDATE;

-- name: StartUserTOTP :exec
-- A new setup replaces an unfinished one. It never replaces a confirmed
-- authenticator: the caller turns that off first.
INSERT INTO user_totp (user_id, secret_sealed) VALUES (@user_id, @secret_sealed)
ON CONFLICT (user_id) DO UPDATE SET secret_sealed = EXCLUDED.secret_sealed, last_step = 0
WHERE user_totp.confirmed_at IS NULL;

-- name: ConfirmUserTOTP :exec
UPDATE user_totp SET confirmed_at = now(), last_step = @last_step WHERE user_id = @user_id AND confirmed_at IS NULL;

-- name: SetUserTOTPStep :exec
UPDATE user_totp SET last_step = @last_step WHERE user_id = @user_id AND last_step < @last_step;

-- name: DeleteUserTOTP :execrows
DELETE FROM user_totp WHERE user_id = @user_id;

-- name: DeleteUserRecoveryCodes :exec
DELETE FROM user_recovery_codes WHERE user_id = @user_id;

-- name: InsertUserRecoveryCode :exec
INSERT INTO user_recovery_codes (user_id, code_hash) VALUES (@user_id, @code_hash);

-- name: UseUserRecoveryCode :execrows
UPDATE user_recovery_codes SET used_at = now()
WHERE user_id = @user_id AND code_hash = @code_hash AND used_at IS NULL;

-- name: CountUserRecoveryCodes :one
SELECT count(*) FROM user_recovery_codes WHERE user_id = @user_id AND used_at IS NULL;

-- name: CreateLoginChallenge :exec
INSERT INTO login_challenges (token_hash, user_id, expires_at, ip, user_agent)
VALUES (@token_hash, @user_id, @expires_at, @ip, @user_agent);

-- name: GetLoginChallengeForUpdate :one
SELECT c.id, c.user_id, c.expires_at, c.attempts, c.used_at, u.email, u.status
FROM login_challenges c
JOIN users u ON u.id = c.user_id
WHERE c.token_hash = @token_hash
FOR UPDATE OF c;

-- name: CountLoginChallengeAttempt :exec
UPDATE login_challenges SET attempts = attempts + 1 WHERE id = @id;

-- name: UseLoginChallenge :exec
UPDATE login_challenges SET used_at = now() WHERE id = @id;

-- name: DeleteOldLoginChallenges :exec
-- Challenges are useless after a few minutes; old ones are cleared as new
-- ones are made.
DELETE FROM login_challenges WHERE expires_at < now() - interval '1 day';

-- name: RevokeOtherUserSessions :execrows
UPDATE sessions SET revoked_at = now()
WHERE user_id = @user_id AND id <> @keep_id AND revoked_at IS NULL;
