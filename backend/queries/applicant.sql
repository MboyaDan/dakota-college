-- ---------- applicant sign-in (Stage 8b, D-96) ----------

-- name: CreateApplicantSignInLink :exec
INSERT INTO applicant_sign_in_links (email, full_name, application_id, next_path, token_hash, expires_at, requested_ip)
VALUES (@email, @full_name, @application_id, @next_path, @token_hash, @expires_at, @requested_ip);

-- name: GetApplicantSignInLinkForUpdate :one
SELECT id, email, full_name, application_id, next_path, expires_at, used_at
FROM applicant_sign_in_links
WHERE token_hash = @token_hash
FOR UPDATE;

-- name: UseApplicantSignInLinks :exec
-- One sign-in spends every outstanding link for the email address.
UPDATE applicant_sign_in_links SET used_at = now() WHERE email = @email AND used_at IS NULL;

-- name: CreateApplicantUser :one
INSERT INTO users (email, full_name, kind) VALUES (@email, @full_name, 'applicant')
RETURNING id;

-- name: GetApplicationForResume :one
-- The account that owns an application, found by its reference number.
SELECT a.id, a.reference, u.email, u.full_name, u.kind, u.status
FROM applications a
JOIN applicants ap ON ap.id = a.applicant_id
JOIN users u ON u.id = ap.user_id
WHERE a.reference = @reference;

-- name: ListApplicantApplications :many
SELECT a.id, a.reference, a.status, a.created_at, a.updated_at, a.submitted_at,
       p.title AS programme_title, p.slug AS programme_slug,
       i.name AS intake_name, i.starts_on AS intake_starts_on
FROM applications a
JOIN applicants ap ON ap.id = a.applicant_id
JOIN programmes p ON p.id = a.programme_id
LEFT JOIN programme_intakes i ON i.id = a.intake_id
WHERE ap.user_id = @user_id
ORDER BY a.created_at DESC;

-- name: LockApplicantEmail :exec
-- Serialises sign-ins for one address, so two links used at once cannot
-- both create the account.
SELECT pg_advisory_xact_lock(hashtextextended(lower(@email::text), 0));

-- ---------- dev seed only ----------

-- name: SeedApplicantUser :one
INSERT INTO users (email, full_name, kind) VALUES (@email, @full_name, 'applicant')
ON CONFLICT (email) DO UPDATE SET full_name = EXCLUDED.full_name, kind = 'applicant'
RETURNING id;

-- name: SeedApplicant :one
INSERT INTO applicants (user_id, email, first_name, last_name) VALUES (@user_id, @email, @first_name, @last_name)
ON CONFLICT (user_id) DO UPDATE SET email = EXCLUDED.email
RETURNING id;

-- name: SeedDraftApplication :exec
INSERT INTO applications (reference, applicant_id, programme_id) VALUES (@reference, @applicant_id, @programme_id)
ON CONFLICT (reference) DO NOTHING;
