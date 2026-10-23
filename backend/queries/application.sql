-- ---------- online application form (Stage 9a) ----------

-- name: ListOpenIntakes :many
-- Intakes taking applications on @today (Nairobi date).
SELECT id, name, starts_on, application_deadline
FROM programme_intakes
WHERE programme_id = @programme_id AND status = 'open'
  AND (applications_open_on IS NULL OR applications_open_on <= @today::date)
  AND (application_deadline IS NULL OR application_deadline >= @today::date)
ORDER BY starts_on;

-- name: GetApplicantByUser :one
SELECT id, user_id, email, phone, first_name, middle_name, last_name, date_of_birth,
       identity_document_type, identity_document, nationality, county
FROM applicants WHERE user_id = @user_id;

-- name: CreateApplicantProfile :one
INSERT INTO applicants (user_id, email, first_name, last_name)
VALUES (@user_id, @email, @first_name, @last_name)
ON CONFLICT (user_id) DO UPDATE SET email = applicants.email
RETURNING id;

-- name: FindLiveApplication :one
SELECT id FROM applications
WHERE applicant_id = @applicant_id AND programme_id = @programme_id
  AND COALESCE(intake_id, '00000000-0000-0000-0000-000000000000'::uuid)
    = COALESCE(sqlc.narg('intake_id')::uuid, '00000000-0000-0000-0000-000000000000'::uuid)
  AND status NOT IN ('rejected', 'withdrawn', 'expired');

-- name: InsertApplication :one
INSERT INTO applications (reference, applicant_id, programme_id, intake_id, draft_expires_at)
VALUES (@reference, @applicant_id, @programme_id, @intake_id, @draft_expires_at)
RETURNING id;

-- name: InsertStatusHistory :exec
INSERT INTO application_status_history (application_id, from_status, to_status, changed_by, reason)
VALUES (@application_id, @from_status, @to_status, @changed_by, @reason);

-- name: GetOwnApplication :one
-- An application, only if @user_id owns it.
SELECT a.id, a.reference, a.status, a.applicant_id, a.programme_id, a.intake_id,
       a.sponsor_type, a.sponsor_name, a.sponsor_phone, a.draft_expires_at,
       a.created_at, a.updated_at, a.submitted_at,
       p.title AS programme_title, p.slug AS programme_slug,
       i.name AS intake_name, i.starts_on AS intake_starts_on
FROM applications a
JOIN applicants ap ON ap.id = a.applicant_id
JOIN programmes p ON p.id = a.programme_id
LEFT JOIN programme_intakes i ON i.id = a.intake_id
WHERE a.id = @id AND ap.user_id = @user_id;

-- name: LockOwnApplication :one
SELECT a.id, a.status, a.applicant_id, a.programme_id, a.draft_expires_at
FROM applications a
JOIN applicants ap ON ap.id = a.applicant_id
WHERE a.id = @id AND ap.user_id = @user_id
FOR UPDATE OF a;

-- name: ExpireDraft :execrows
UPDATE applications SET status = 'expired' WHERE id = @id AND status = 'draft';

-- name: ExtendDraft :exec
UPDATE applications SET draft_expires_at = @draft_expires_at WHERE id = @id;

-- name: GetApplicant :one
SELECT id, email, phone, first_name, middle_name, last_name, date_of_birth,
       identity_document_type, identity_document, nationality, county
FROM applicants WHERE id = @id;

-- name: UpdateApplicantPersonal :exec
UPDATE applicants
SET first_name = @first_name, middle_name = @middle_name, last_name = @last_name,
    date_of_birth = @date_of_birth, identity_document_type = @identity_document_type,
    identity_document = @identity_document, nationality = @nationality, county = @county, phone = @phone
WHERE id = @id;

-- name: GetGuardian :one
SELECT id, full_name, relationship, phone, email FROM application_guardians WHERE application_id = @application_id;

-- name: UpsertGuardian :one
INSERT INTO application_guardians (application_id, full_name, relationship, phone, email)
VALUES (@application_id, @full_name, @relationship, @phone, @email)
ON CONFLICT (application_id) DO UPDATE
  SET full_name = EXCLUDED.full_name, relationship = EXCLUDED.relationship,
      phone = EXCLUDED.phone, email = EXCLUDED.email
RETURNING id;

-- name: ListEducation :many
SELECT level, institution, year_completed, qualification, mean_grade, subject_grades
FROM application_education WHERE application_id = @application_id ORDER BY sort_order;

-- name: DeleteEducation :exec
DELETE FROM application_education WHERE application_id = @application_id;

-- name: InsertEducation :exec
INSERT INTO application_education (application_id, level, institution, year_completed, qualification, mean_grade, subject_grades, sort_order)
VALUES (@application_id, @level, @institution, @year_completed, @qualification, @mean_grade, @subject_grades, @sort_order);

-- name: UpdateApplicationSponsor :exec
UPDATE applications SET sponsor_type = @sponsor_type, sponsor_name = @sponsor_name, sponsor_phone = @sponsor_phone
WHERE id = @id;

-- name: UpdateApplicationIntake :exec
UPDATE applications SET intake_id = @intake_id WHERE id = @id;

-- name: InsertConsent :exec
INSERT INTO consents (subject_type, subject_id, purpose, given, notice_version, given_by_name, ip, user_agent)
VALUES (@subject_type, @subject_id, @purpose, @given, @notice_version, @given_by_name, @ip, @user_agent);

-- name: LatestConsent :one
SELECT given, given_by_name, notice_version, captured_at FROM consents
WHERE subject_type = @subject_type AND subject_id = @subject_id AND purpose = @purpose
ORDER BY captured_at DESC, id DESC
LIMIT 1;
