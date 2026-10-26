-- ---------- staff admissions workflow (Stage 10a) ----------
-- Staff see applications once submitted; drafts are the applicant's own.
-- Filters are optional: a NULL argument means "any".

-- name: StaffListApplications :many
SELECT a.id, a.reference, a.status, a.submitted_at, a.updated_at, a.assigned_to,
       ap.first_name, ap.middle_name, ap.last_name, ap.email,
       p.title AS programme_title, i.name AS intake_name, i.starts_on AS intake_starts_on,
       u.full_name AS assignee_name, o.respond_by
FROM applications a
JOIN applicants ap ON ap.id = a.applicant_id
JOIN programmes p ON p.id = a.programme_id
LEFT JOIN programme_intakes i ON i.id = a.intake_id
LEFT JOIN users u ON u.id = a.assigned_to
LEFT JOIN offers o ON o.application_id = a.id
WHERE a.submitted_at IS NOT NULL
  AND (sqlc.narg('status')::text IS NULL OR a.status = sqlc.narg('status'))
  AND (sqlc.narg('programme_id')::uuid IS NULL OR a.programme_id = sqlc.narg('programme_id'))
  AND (sqlc.narg('intake_id')::uuid IS NULL OR a.intake_id = sqlc.narg('intake_id'))
  AND (sqlc.narg('assigned_to')::uuid IS NULL OR a.assigned_to = sqlc.narg('assigned_to'))
  AND (NOT @unassigned::boolean OR a.assigned_to IS NULL)
  AND (sqlc.narg('search')::text IS NULL
       OR a.reference ILIKE '%' || sqlc.narg('search') || '%'
       OR (ap.first_name || ' ' || ap.middle_name || ' ' || ap.last_name) ILIKE '%' || sqlc.narg('search') || '%'
       OR ap.email ILIKE '%' || sqlc.narg('search') || '%'
       OR ap.phone ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY a.submitted_at DESC, a.id
LIMIT @max_items OFFSET @skip;

-- name: StaffCountApplications :one
SELECT count(*)
FROM applications a
JOIN applicants ap ON ap.id = a.applicant_id
WHERE a.submitted_at IS NOT NULL
  AND (sqlc.narg('status')::text IS NULL OR a.status = sqlc.narg('status'))
  AND (sqlc.narg('programme_id')::uuid IS NULL OR a.programme_id = sqlc.narg('programme_id'))
  AND (sqlc.narg('intake_id')::uuid IS NULL OR a.intake_id = sqlc.narg('intake_id'))
  AND (sqlc.narg('assigned_to')::uuid IS NULL OR a.assigned_to = sqlc.narg('assigned_to'))
  AND (NOT @unassigned::boolean OR a.assigned_to IS NULL)
  AND (sqlc.narg('search')::text IS NULL
       OR a.reference ILIKE '%' || sqlc.narg('search') || '%'
       OR (ap.first_name || ' ' || ap.middle_name || ' ' || ap.last_name) ILIKE '%' || sqlc.narg('search') || '%'
       OR ap.email ILIKE '%' || sqlc.narg('search') || '%'
       OR ap.phone ILIKE '%' || sqlc.narg('search') || '%');

-- name: CountApplicationsForInbox :one
SELECT count(*) FILTER (WHERE status = 'submitted') AS new_count,
       count(*) FILTER (WHERE status IN ('submitted', 'under_review', 'info_requested') AND assigned_to = @user_id) AS mine_count,
       count(*) FILTER (WHERE status = 'info_requested') AS info_requested_count,
       count(*) FILTER (WHERE status = 'offer_accepted') AS to_enrol_count
FROM applications
WHERE submitted_at IS NOT NULL;

-- name: StaffGetApplication :one
SELECT a.id, a.reference, a.status, a.applicant_id, a.programme_id, a.intake_id, a.assigned_to,
       a.submitted_at, a.decided_at, a.updated_at, a.submitted_snapshot,
       ap.user_id AS applicant_user_id, ap.email, ap.phone, ap.first_name, ap.last_name,
       p.title AS programme_title, i.name AS intake_name, i.starts_on AS intake_starts_on,
       u.full_name AS assignee_name
FROM applications a
JOIN applicants ap ON ap.id = a.applicant_id
JOIN programmes p ON p.id = a.programme_id
LEFT JOIN programme_intakes i ON i.id = a.intake_id
LEFT JOIN users u ON u.id = a.assigned_to
WHERE a.id = @id AND a.submitted_at IS NOT NULL;

-- name: LockApplication :one
SELECT a.id, a.status, a.reference, a.applicant_id, a.programme_id, a.intake_id, a.assigned_to, a.submitted_at
FROM applications a
WHERE a.id = @id
FOR UPDATE;

-- name: SetApplicationStatus :execrows
-- Moves from one status to another; 0 rows if it was not in @from_status.
-- The trigger refuses any move not in application_transitions.
UPDATE applications
SET status = @to_status,
    decided_at = CASE WHEN @to_status::text IN ('offered', 'rejected') THEN now() ELSE decided_at END
WHERE id = @id AND status = @from_status;

-- name: ListTransitions :many
SELECT from_status, to_status FROM application_transitions ORDER BY from_status, to_status;

-- name: SetApplicationAssignee :exec
UPDATE applications SET assigned_to = @assigned_to WHERE id = @id;

-- name: ListAdmissionsAssignees :many
-- Active staff whose roles let them review applications.
SELECT DISTINCT u.id, u.full_name, u.email
FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN role_permissions rp ON rp.role_id = ur.role_id
WHERE u.kind = 'staff' AND u.status = 'active' AND rp.permission_key = 'admissions.review'
ORDER BY u.full_name;

-- name: StaffStatusHistory :many
SELECT h.from_status, h.to_status, h.reason, h.created_at, u.full_name AS changed_by_name
FROM application_status_history h
LEFT JOIN users u ON u.id = h.changed_by
WHERE h.application_id = @application_id
ORDER BY h.created_at, h.id;

-- name: StaffListApplicationDocuments :many
-- Every document still on file, with whether it came after submission.
SELECT d.id, d.kind, d.created_at, f.original_name, f.content_type, f.size_bytes, f.scan_status
FROM application_documents d
JOIN files f ON f.id = d.file_id
WHERE d.application_id = @application_id AND f.deleted_at IS NULL
ORDER BY d.created_at;

-- name: InsertApplicationNote :one
INSERT INTO application_notes (application_id, author_id, body) VALUES (@application_id, @author_id, @body)
RETURNING id, created_at;

-- name: ListApplicationNotes :many
SELECT n.id, n.body, n.created_at, u.full_name AS author_name
FROM application_notes n
JOIN users u ON u.id = n.author_id
WHERE n.application_id = @application_id
ORDER BY n.created_at, n.id;

-- name: InsertInfoRequest :one
INSERT INTO application_info_requests (application_id, message, requested_by)
VALUES (@application_id, @message, @requested_by)
RETURNING id, requested_at;

-- name: GetOpenInfoRequest :one
SELECT id, message, requested_at FROM application_info_requests
WHERE application_id = @application_id AND closed_at IS NULL;

-- name: CloseInfoRequest :execrows
UPDATE application_info_requests
SET response = @response, closed_at = now(), closed_by = @closed_by
WHERE id = @id AND closed_at IS NULL;

-- name: ListInfoRequests :many
SELECT r.id, r.message, r.requested_at, r.response, r.closed_at,
       rb.full_name AS requested_by_name, cb.full_name AS closed_by_name
FROM application_info_requests r
JOIN users rb ON rb.id = r.requested_by
LEFT JOIN users cb ON cb.id = r.closed_by
WHERE r.application_id = @application_id
ORDER BY r.requested_at, r.id;

-- name: InsertOffer :one
INSERT INTO offers (application_id, letter_file_id, conditions, issued_by, respond_by)
VALUES (@application_id, @letter_file_id, @conditions, @issued_by, @respond_by)
RETURNING id, issued_at;

-- name: GetOffer :one
SELECT o.id, o.letter_file_id, o.conditions, o.issued_at, o.respond_by, o.accepted_at, o.declined_at,
       u.full_name AS issued_by_name
FROM offers o
JOIN users u ON u.id = o.issued_by
WHERE o.application_id = @application_id;

-- name: AcceptOffer :execrows
UPDATE offers SET accepted_at = now() WHERE application_id = @application_id AND accepted_at IS NULL AND declined_at IS NULL;

-- name: DeclineOffer :execrows
UPDATE offers SET declined_at = now() WHERE application_id = @application_id AND accepted_at IS NULL AND declined_at IS NULL;

-- name: ExpireOverdueOffers :execrows
-- Offers not answered by their respond-by date (Nairobi @today) lapse; the
-- history row is written in the same statement.
WITH expired AS (
  UPDATE applications a SET status = 'expired'
  FROM offers o
  WHERE o.application_id = a.id AND a.status = 'offered' AND o.respond_by < @today::date
  RETURNING a.id
)
INSERT INTO application_status_history (application_id, from_status, to_status, reason)
SELECT id, 'offered', 'expired', 'Offer not accepted by its respond-by date' FROM expired;

-- name: ListLetterFees :many
-- Fees to print on an offer letter: the programme's own and the intake's.
SELECT f.label, f.category, f.amount_cents, f.frequency, f.notes
FROM fee_items f
JOIN programmes p ON p.id = f.programme_id
WHERE f.programme_id = @programme_id AND p.status = 'published'
  AND (f.intake_id IS NULL OR f.intake_id = sqlc.narg('intake_id')::uuid)
ORDER BY f.sort_order, f.label;

-- name: GetApplicantForLetter :one
SELECT first_name, middle_name, last_name, email, phone FROM applicants WHERE id = @id;

-- name: GetProgrammeForLetter :one
SELECT p.title, p.is_sample, i.name AS intake_name, i.starts_on AS intake_starts_on
FROM programmes p
LEFT JOIN programme_intakes i ON i.id = sqlc.narg('intake_id')::uuid AND i.programme_id = p.id
WHERE p.id = @id;

-- ---------- enrolment ----------

-- name: GetApplicantForEnrolment :one
SELECT id, user_id, email, phone, first_name, middle_name, last_name, date_of_birth,
       identity_document_type, identity_document
FROM applicants WHERE id = @id;

-- name: MakeStudentAccount :execrows
-- One account per person: enrolment turns the applicant account into a
-- student account. Applicant links are never issued to student accounts.
UPDATE users SET kind = 'student' WHERE id = @id AND kind = 'applicant';

-- name: InsertStudent :one
INSERT INTO students (student_number, user_id, applicant_id, application_id, programme_id, intake_id, cohort_id,
                      first_name, middle_name, last_name, date_of_birth, email, phone,
                      identity_document_type, identity_document, enrolled_by)
VALUES (@student_number, @user_id, @applicant_id, @application_id, @programme_id, @intake_id, @cohort_id,
        @first_name, @middle_name, @last_name, @date_of_birth, @email, @phone,
        @identity_document_type, @identity_document, @enrolled_by)
RETURNING id, enrolled_at;

-- name: InsertDraftInvoice :one
INSERT INTO invoices (student_id, intake_id, note, created_by)
VALUES (@student_id, @intake_id, @note, @created_by)
RETURNING id;

-- name: GetStudentByApplication :one
SELECT s.id, s.student_number, s.status, s.enrolled_at, inv.id AS invoice_id, inv.status AS invoice_status
FROM students s
JOIN LATERAL (
  SELECT id, status FROM invoices WHERE student_id = s.id ORDER BY created_at LIMIT 1
) inv ON true
WHERE s.application_id = @application_id;

-- ---------- reports ----------

-- name: AdmissionsReport :many
-- Applications submitted from @from_date to @to_date (inclusive, Nairobi
-- dates), by programme and intake, with where each one is now.
SELECT p.id AS programme_id, p.title AS programme_title, i.id AS intake_id, i.name AS intake_name,
       count(*) AS submitted,
       count(*) FILTER (WHERE a.status IN ('submitted', 'under_review', 'info_requested')) AS in_review,
       count(*) FILTER (WHERE a.status = 'info_requested') AS info_requested,
       count(*) FILTER (WHERE a.status IN ('offered', 'offer_accepted', 'enrolled')
                           OR EXISTS (SELECT 1 FROM offers o WHERE o.application_id = a.id)) AS offered,
       count(*) FILTER (WHERE a.status IN ('offer_accepted', 'enrolled')) AS accepted,
       count(*) FILTER (WHERE a.status = 'enrolled') AS enrolled,
       count(*) FILTER (WHERE a.status = 'rejected') AS rejected,
       count(*) FILTER (WHERE a.status = 'withdrawn') AS withdrawn,
       count(*) FILTER (WHERE a.status = 'expired') AS expired
FROM applications a
JOIN programmes p ON p.id = a.programme_id
LEFT JOIN programme_intakes i ON i.id = a.intake_id
WHERE a.submitted_at IS NOT NULL
  AND (a.submitted_at AT TIME ZONE 'Africa/Nairobi')::date BETWEEN @from_date::date AND @to_date::date
GROUP BY p.id, p.title, i.id, i.name, i.starts_on
ORDER BY p.title, i.starts_on NULLS LAST;
