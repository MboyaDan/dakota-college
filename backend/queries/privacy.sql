-- Retention schedule, data subject requests and erasure (Stage 17c).

-- name: AllowErasure :exec
-- Lets this transaction blank the free-text columns of append-only tables
-- that may name a person (migration 00024). Ends with the transaction.
SELECT set_config('dtc.erasure', 'on', true);

-- name: RetentionLock :one
-- One retention run at a time across every API instance.
SELECT pg_try_advisory_xact_lock(1702170217);

-- name: InsertRetentionRun :one
INSERT INTO retention_runs (mode) VALUES (@mode) RETURNING id;

-- name: FinishRetentionRun :exec
UPDATE retention_runs SET finished_at = now(), results = @results, error = @error WHERE id = @id;

-- name: ListRetentionRuns :many
SELECT id, mode, started_at, finished_at, results, error
FROM retention_runs ORDER BY started_at DESC LIMIT @max_rows;

-- name: DeleteOldRetentionRuns :execrows
DELETE FROM retention_runs WHERE started_at < @before;

-- name: DeleteSpentSignInLinks :execrows
DELETE FROM applicant_sign_in_links WHERE expires_at < @before;

-- name: DeleteSpentPasswordResets :execrows
DELETE FROM password_resets WHERE expires_at < @before;

-- name: DeleteSpentFileLinks :execrows
DELETE FROM file_links WHERE expires_at < @before;

-- name: DeleteSpentLoginChallenges :execrows
DELETE FROM login_challenges WHERE expires_at < @before;

-- name: DeleteEndedSessions :execrows
DELETE FROM sessions WHERE expires_at < @before OR revoked_at < @before;

-- name: DeleteSpentIdempotencyRecords :execrows
DELETE FROM idempotency_records WHERE expires_at < @before;

-- name: ExpireOverdueDrafts :execrows
-- Drafts past their expiry date become expired, with a history row (D-107
-- did this only when the draft was next opened).
WITH e AS (
  UPDATE applications SET status = 'expired'
  WHERE status = 'draft' AND draft_expires_at <= @now
  RETURNING id
)
INSERT INTO application_status_history (application_id, from_status, to_status, reason)
SELECT id, 'draft', 'expired', 'Draft not saved before its expiry date' FROM e;

-- name: DeleteOldMessages :execrows
-- The message log: who was sent what and when. The text itself is already
-- gone once a message is finished (Stage 16a).
DELETE FROM notifications WHERE status IN ('sent', 'failed', 'skipped') AND created_at < @before;

-- name: DeleteClosedEnquiries :execrows
-- Notes go with the enquiry (ON DELETE CASCADE).
DELETE FROM enquiries WHERE status = 'closed' AND closed_at < @before;

-- name: ApplicantsDueForErasure :many
-- Applicants who never became students, with nothing in progress, whose
-- last application ended (or whose intake started, if later) before
-- @before. Applicants with no application count from when they signed up.
SELECT a.id
FROM applicants a
WHERE a.erased_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM students s WHERE s.applicant_id = a.id)
  AND NOT EXISTS (
    SELECT 1 FROM applications p
    WHERE p.applicant_id = a.id AND p.status NOT IN ('rejected', 'withdrawn', 'expired'))
  AND GREATEST(
        a.created_at,
        (SELECT max(GREATEST(p.updated_at, i.starts_on::timestamptz))
         FROM applications p LEFT JOIN programme_intakes i ON i.id = p.intake_id
         WHERE p.applicant_id = a.id)
      ) < @before
ORDER BY a.created_at
LIMIT @max_rows;

-- name: FilesAwaitingRemoval :many
SELECT id, bucket, object_key FROM files
WHERE deleted_at IS NOT NULL AND storage_removed_at IS NULL
ORDER BY deleted_at
LIMIT @max_rows;

-- name: CountFilesAwaitingRemoval :one
SELECT count(*) FROM files WHERE deleted_at IS NOT NULL AND storage_removed_at IS NULL;

-- name: MarkFileRemoved :exec
UPDATE files SET storage_removed_at = now() WHERE id = @id AND deleted_at IS NOT NULL;

-- Erasure of one applicant. Run in this order inside one transaction.

-- name: LockApplicantForErasure :one
SELECT a.id, a.user_id, a.email, a.phone, a.erased_at,
       EXISTS (SELECT 1 FROM students s WHERE s.applicant_id = a.id) AS is_student,
       (SELECT count(*) FROM applications p
        WHERE p.applicant_id = a.id AND p.status NOT IN ('draft', 'rejected', 'withdrawn', 'expired')) AS in_progress
FROM applicants a WHERE a.id = @id
FOR UPDATE;

-- name: EraseApplicantConsents :execrows
UPDATE consents SET given_by_name = '[Erased]', ip = NULL, user_agent = NULL
WHERE (subject_type = 'applicant' AND subject_id = @applicant_id)
   OR (subject_type = 'guardian' AND subject_id IN (
        SELECT g.id FROM application_guardians g JOIN applications p ON p.id = g.application_id
        WHERE p.applicant_id = @applicant_id));

-- name: MarkApplicantFilesDeleted :execrows
-- Uploaded documents, offer letters and anything else the account uploaded.
UPDATE files SET deleted_at = now()
WHERE deleted_at IS NULL AND (
     id IN (SELECT d.file_id FROM application_documents d JOIN applications p ON p.id = d.application_id
            WHERE p.applicant_id = @applicant_id)
  OR id IN (SELECT o.letter_file_id FROM offers o JOIN applications p ON p.id = o.application_id
            WHERE p.applicant_id = @applicant_id AND o.letter_file_id IS NOT NULL)
  OR (sqlc.narg(user_id)::uuid IS NOT NULL AND uploaded_by_user_id = sqlc.narg(user_id)::uuid));

-- name: DeleteApplicantGuardians :exec
DELETE FROM application_guardians WHERE application_id IN (SELECT id FROM applications WHERE applicant_id = @applicant_id);

-- name: DeleteApplicantEducation :exec
DELETE FROM application_education WHERE application_id IN (SELECT id FROM applications WHERE applicant_id = @applicant_id);

-- name: DeleteApplicantDocuments :exec
DELETE FROM application_documents WHERE application_id IN (SELECT id FROM applications WHERE applicant_id = @applicant_id);

-- name: EraseApplicantOffers :exec
UPDATE offers SET letter_file_id = NULL, conditions = ''
WHERE application_id IN (SELECT id FROM applications WHERE applicant_id = @applicant_id);

-- name: EraseApplicantInfoRequests :exec
UPDATE application_info_requests SET message = '[Erased]', response = ''
WHERE application_id IN (SELECT id FROM applications WHERE applicant_id = @applicant_id);

-- name: EraseApplicantNotes :exec
UPDATE application_notes SET body = '[Erased]'
WHERE application_id IN (SELECT id FROM applications WHERE applicant_id = @applicant_id);

-- name: EraseApplicantStatusReasons :exec
UPDATE application_status_history SET reason = '[Erased]'
WHERE reason <> '' AND application_id IN (SELECT id FROM applications WHERE applicant_id = @applicant_id);

-- name: EraseApplicantApplications :execrows
-- Reference, programme, intake, status and dates stay for counts.
UPDATE applications
SET sponsor_name = '', sponsor_phone = '',
    submitted_snapshot = CASE WHEN submitted_snapshot IS NULL THEN NULL ELSE '{"erased": true}'::jsonb END
WHERE applicant_id = @applicant_id;

-- name: DeleteApplicantSignInLinks :exec
DELETE FROM applicant_sign_in_links
WHERE email = @email OR application_id IN (SELECT id FROM applications WHERE applicant_id = @applicant_id);

-- name: DeleteMessagesTo :execrows
-- Messages being sent right now are left for the worker to finish.
DELETE FROM notifications
WHERE status <> 'sending' AND (recipient = @email::text OR (sqlc.narg(phone)::text IS NOT NULL AND recipient = sqlc.narg(phone)::text));

-- name: EraseApplicantRow :exec
UPDATE applicants
SET email = @erased_email, phone = NULL, first_name = 'Erased', middle_name = '', last_name = 'applicant',
    date_of_birth = NULL, identity_document = '', identity_document_type = NULL,
    nationality = '', county = '', erased_at = now()
WHERE id = @id;

-- name: EraseApplicantUser :exec
UPDATE users
SET email = @erased_email, full_name = 'Erased applicant', phone = NULL, password_hash = NULL, status = 'disabled'
WHERE id = @id AND kind = 'applicant';

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = @user_id;

-- name: DeleteUserPasswordResets :exec
DELETE FROM password_resets WHERE user_id = @user_id;

-- name: DeleteEnquiryByID :execrows
DELETE FROM enquiries WHERE id = @id;

-- Finding a person's records. Phone numbers match on their last nine
-- digits, so 0712..., 254712... and +254 712 ... are the same number.

-- name: FindApplicantsByContact :many
SELECT a.id, a.user_id, a.first_name, a.middle_name, a.last_name, a.email, a.phone, a.created_at, a.erased_at,
       EXISTS (SELECT 1 FROM students s WHERE s.applicant_id = a.id) AS is_student,
       (SELECT count(*) FROM applications p WHERE p.applicant_id = a.id) AS applications,
       (SELECT count(*) FROM applications p
        WHERE p.applicant_id = a.id AND p.status NOT IN ('draft', 'rejected', 'withdrawn', 'expired')) AS in_progress
FROM applicants a
WHERE (sqlc.narg(email)::citext IS NOT NULL AND a.email = sqlc.narg(email)::citext)
   OR (sqlc.narg(phone_tail)::text IS NOT NULL AND right(regexp_replace(coalesce(a.phone, ''), '\D', '', 'g'), 9) = sqlc.narg(phone_tail)::text)
ORDER BY a.created_at;

-- name: FindStudentsByContact :many
SELECT s.id, s.user_id, s.student_number, s.first_name, s.middle_name, s.last_name, s.email, s.phone, s.status, s.enrolled_at
FROM students s
WHERE (sqlc.narg(email)::citext IS NOT NULL AND s.email = sqlc.narg(email)::citext)
   OR (sqlc.narg(phone_tail)::text IS NOT NULL AND right(regexp_replace(coalesce(s.phone, ''), '\D', '', 'g'), 9) = sqlc.narg(phone_tail)::text)
ORDER BY s.enrolled_at;

-- name: FindEnquiriesByContact :many
SELECT e.id, e.full_name, e.email, e.phone, e.query_type, e.status, e.created_at
FROM enquiries e
WHERE (sqlc.narg(email)::citext IS NOT NULL AND e.email = sqlc.narg(email)::citext)
   OR (sqlc.narg(phone_tail)::text IS NOT NULL AND right(regexp_replace(coalesce(e.phone, ''), '\D', '', 'g'), 9) = sqlc.narg(phone_tail)::text)
ORDER BY e.created_at;

-- name: FindGuardianRecordsByContact :many
-- The person may be named as someone's parent or guardian.
SELECT 'application' AS source, g.id, g.full_name, g.email, g.phone, p.reference AS record
FROM application_guardians g JOIN applications p ON p.id = g.application_id
WHERE (sqlc.narg(email)::citext IS NOT NULL AND g.email = sqlc.narg(email)::citext)
   OR (sqlc.narg(phone_tail)::text IS NOT NULL AND right(regexp_replace(g.phone, '\D', '', 'g'), 9) = sqlc.narg(phone_tail)::text)
UNION ALL
SELECT 'student', g.id, g.full_name, g.email, g.phone, s.student_number
FROM student_guardians g JOIN students s ON s.id = g.student_id
WHERE (sqlc.narg(email)::citext IS NOT NULL AND g.email = sqlc.narg(email)::citext)
   OR (sqlc.narg(phone_tail)::text IS NOT NULL AND right(regexp_replace(coalesce(g.phone, ''), '\D', '', 'g'), 9) = sqlc.narg(phone_tail)::text);

-- name: FindStaffByEmail :many
SELECT u.id, u.full_name, u.email, u.status, u.created_at
FROM users u WHERE u.kind = 'staff' AND sqlc.narg(email)::citext IS NOT NULL AND u.email = sqlc.narg(email)::citext;

-- Exports for an access or portability request: everything held about one
-- record, as JSON. Secrets (password hashes, tokens, message text) are left
-- out; staff notes are included because they are the person's data too.

-- name: ExportApplicant :one
SELECT jsonb_build_object(
  'applicant', to_jsonb(a) - 'user_id',
  'applications', COALESCE((
    SELECT jsonb_agg(jsonb_build_object(
      'reference', p.reference, 'programme', pr.title, 'intake', i.name, 'status', p.status,
      'sponsor_type', p.sponsor_type, 'sponsor_name', p.sponsor_name, 'sponsor_phone', p.sponsor_phone,
      'declaration_accepted_at', p.declaration_accepted_at, 'submitted_at', p.submitted_at,
      'decided_at', p.decided_at, 'created_at', p.created_at, 'submitted_answers', p.submitted_snapshot,
      'guardians', COALESCE((SELECT jsonb_agg(to_jsonb(g) - 'application_id') FROM application_guardians g WHERE g.application_id = p.id), '[]'),
      'education', COALESCE((SELECT jsonb_agg(to_jsonb(e) - 'application_id' ORDER BY e.sort_order) FROM application_education e WHERE e.application_id = p.id), '[]'),
      'documents', COALESCE((SELECT jsonb_agg(jsonb_build_object('kind', d.kind, 'file_name', f.original_name, 'uploaded_at', d.created_at))
                             FROM application_documents d JOIN files f ON f.id = d.file_id WHERE d.application_id = p.id), '[]'),
      'status_history', COALESCE((SELECT jsonb_agg(jsonb_build_object('from', h.from_status, 'to', h.to_status, 'reason', h.reason, 'at', h.created_at) ORDER BY h.id)
                                  FROM application_status_history h WHERE h.application_id = p.id), '[]'),
      'information_requests', COALESCE((SELECT jsonb_agg(jsonb_build_object('message', r.message, 'response', r.response, 'requested_at', r.requested_at, 'closed_at', r.closed_at) ORDER BY r.requested_at)
                                        FROM application_info_requests r WHERE r.application_id = p.id), '[]'),
      'offer', (SELECT jsonb_build_object('conditions', o.conditions, 'issued_at', o.issued_at, 'respond_by', o.respond_by, 'accepted_at', o.accepted_at, 'declined_at', o.declined_at)
                FROM offers o WHERE o.application_id = p.id),
      'staff_notes', COALESCE((SELECT jsonb_agg(jsonb_build_object('note', n.body, 'at', n.created_at) ORDER BY n.created_at)
                               FROM application_notes n WHERE n.application_id = p.id), '[]')
    ) ORDER BY p.created_at)
    FROM applications p JOIN programmes pr ON pr.id = p.programme_id LEFT JOIN programme_intakes i ON i.id = p.intake_id
    WHERE p.applicant_id = a.id), '[]'),
  'consents', COALESCE((
    SELECT jsonb_agg(jsonb_build_object('purpose', c.purpose, 'given', c.given, 'given_by', c.given_by_name, 'notice_version', c.notice_version, 'at', c.captured_at) ORDER BY c.captured_at)
    FROM consents c
    WHERE (c.subject_type = 'applicant' AND c.subject_id = a.id)
       OR (c.subject_type = 'guardian' AND c.subject_id IN (
            SELECT g.id FROM application_guardians g JOIN applications p ON p.id = g.application_id WHERE p.applicant_id = a.id))), '[]')
)::jsonb
FROM applicants a WHERE a.id = @id;

-- name: ExportStudent :one
SELECT jsonb_build_object(
  'student', to_jsonb(s) - 'user_id' - 'applicant_id' - 'application_id' - 'enrolled_by',
  'programme', (SELECT pr.title FROM programmes pr WHERE pr.id = s.programme_id),
  'guardians', COALESCE((SELECT jsonb_agg(to_jsonb(g) - 'student_id') FROM student_guardians g WHERE g.student_id = s.id), '[]'),
  'status_history', COALESCE((SELECT jsonb_agg(jsonb_build_object('from', h.from_status, 'to', h.to_status, 'reason', h.reason, 'at', h.created_at) ORDER BY h.id)
                              FROM student_status_history h WHERE h.student_id = s.id), '[]'),
  'unit_registrations', COALESCE((
    SELECT jsonb_agg(jsonb_build_object('unit', u.code || ' ' || u.title, 'term', t.name, 'status', r.status, 'registered_at', r.registered_at, 'dropped_at', r.dropped_at) ORDER BY r.registered_at)
    FROM unit_registrations r JOIN unit_offerings o ON o.id = r.offering_id JOIN units u ON u.id = o.unit_id JOIN academic_terms t ON t.id = o.term_id
    WHERE r.student_id = s.id), '[]'),
  'released_results', COALESCE((
    SELECT jsonb_agg(jsonb_build_object('unit', u.code || ' ' || u.title, 'term', t.name, 'released_at', l.released_at, 'result', e.value - 'student_id') ORDER BY l.released_at)
    FROM (SELECT DISTINCT ON (sheet_id) sheet_id, results, released_at FROM result_releases ORDER BY sheet_id, version DESC) l
    JOIN result_sheets rs ON rs.id = l.sheet_id JOIN unit_offerings o ON o.id = rs.offering_id
    JOIN units u ON u.id = o.unit_id JOIN academic_terms t ON t.id = o.term_id
    CROSS JOIN LATERAL jsonb_array_elements(l.results) AS e(value)
    WHERE e.value->>'student_id' = s.id::text), '[]'),
  'invoices', COALESCE((SELECT jsonb_agg(jsonb_build_object('number', i.number, 'status', i.status, 'note', i.note, 'created_at', i.created_at,
                          'lines', COALESCE((SELECT jsonb_agg(to_jsonb(l) - 'invoice_id') FROM invoice_lines l WHERE l.invoice_id = i.id), '[]')) ORDER BY i.created_at)
                        FROM invoices i WHERE i.student_id = s.id), '[]'),
  'payments', COALESCE((SELECT jsonb_agg(jsonb_build_object('method', p.method, 'amount_cents', p.amount_cents, 'paid_on', p.paid_on, 'reference', p.reference,
                          'payer_name', p.payer_name, 'status', p.status, 'receipt', (SELECT r.number FROM receipts r WHERE r.payment_id = p.id)) ORDER BY p.paid_on)
                        FROM payments p WHERE p.student_id = s.id), '[]'),
  'adjustments', COALESCE((SELECT jsonb_agg(jsonb_build_object('kind', a.kind, 'amount_cents', a.amount_cents, 'reason', a.reason, 'at', a.created_at) ORDER BY a.created_at)
                           FROM adjustments a WHERE a.student_id = s.id), '[]'),
  'service_requests', COALESCE((SELECT jsonb_agg(jsonb_build_object('kind', r.kind, 'details', r.details, 'status', r.status, 'response', r.response, 'at', r.created_at) ORDER BY r.created_at)
                                FROM service_requests r WHERE r.student_id = s.id), '[]'),
  'profile_change_requests', COALESCE((SELECT jsonb_agg(jsonb_build_object('old', r.old_values, 'new', r.new_values, 'reason', r.reason, 'status', r.status, 'decision', r.decision, 'at', r.created_at) ORDER BY r.created_at)
                                       FROM profile_change_requests r WHERE r.student_id = s.id), '[]'),
  'transcripts', COALESCE((SELECT jsonb_agg(jsonb_build_object('number', t.number, 'issued_at', t.issued_at) ORDER BY t.issued_at)
                           FROM transcripts t WHERE t.student_id = s.id), '[]')
)::jsonb
FROM students s WHERE s.id = @id;

-- name: ExportEnquiry :one
SELECT (to_jsonb(e) - 'assigned_to' - 'programme_id' || jsonb_build_object(
  'programme', (SELECT pr.title FROM programmes pr WHERE pr.id = e.programme_id),
  'staff_notes', COALESCE((SELECT jsonb_agg(jsonb_build_object('note', n.body, 'at', n.created_at) ORDER BY n.created_at)
                           FROM enquiry_notes n WHERE n.enquiry_id = e.id), '[]')
))::jsonb
FROM enquiries e WHERE e.id = @id;

-- name: ExportUserAccount :one
-- The sign-in account behind an applicant, student or member of staff.
SELECT jsonb_build_object(
  'email', u.email, 'full_name', u.full_name, 'phone', u.phone, 'kind', u.kind, 'status', u.status,
  'created_at', u.created_at, 'last_login_at', u.last_login_at,
  'roles', COALESCE((SELECT jsonb_agg(r.name ORDER BY r.name) FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE ur.user_id = u.id), '[]'),
  'recent_activity', COALESCE((
    SELECT jsonb_agg(jsonb_build_object('action', l.action, 'at', l.occurred_at, 'ip', l.ip) ORDER BY l.occurred_at DESC)
    FROM (SELECT action, occurred_at, ip FROM audit_logs WHERE actor_user_id = u.id ORDER BY occurred_at DESC LIMIT 500) l), '[]')
)::jsonb
FROM users u WHERE u.id = @id;

-- name: ExportMessagesTo :one
SELECT COALESCE(jsonb_agg(jsonb_build_object(
  'message', COALESCE(n.template_key, n.system_key), 'channel', n.channel, 'to', n.recipient,
  'status', n.status, 'queued_at', n.created_at, 'sent_at', n.sent_at) ORDER BY n.created_at), '[]')::jsonb
FROM notifications n
WHERE (sqlc.narg(email)::text IS NOT NULL AND n.recipient = sqlc.narg(email)::text)
   OR (sqlc.narg(phone_tail)::text IS NOT NULL AND right(regexp_replace(n.recipient, '\D', '', 'g'), 9) = sqlc.narg(phone_tail)::text AND n.channel = 'sms');

-- Register of requests.

-- name: InsertDataRequest :one
INSERT INTO data_requests (reference, kind, requester_name, requester_contact, acting_for, subject_email, subject_phone,
                           details, received_on, due_on, created_by)
VALUES (@reference, @kind, @requester_name, @requester_contact, @acting_for, sqlc.narg(subject_email), sqlc.narg(subject_phone),
        @details, @received_on, @due_on, @created_by)
RETURNING id;

-- name: InsertDataRequestEvent :exec
INSERT INTO data_request_events (request_id, action, note, actor_id) VALUES (@request_id, @action, @note, @actor_id);

-- name: ListDataRequests :many
SELECT d.id, d.reference, d.kind, d.requester_name, d.status, d.received_on, d.due_on, d.closed_at
FROM data_requests d
WHERE (sqlc.narg(status)::text IS NULL OR d.status = sqlc.narg(status)::text)
ORDER BY (d.status = 'open') DESC, d.due_on, d.created_at DESC
LIMIT @max_rows OFFSET @skip;

-- name: CountDataRequests :one
SELECT count(*) FROM data_requests d WHERE (sqlc.narg(status)::text IS NULL OR d.status = sqlc.narg(status)::text);

-- name: GetDataRequest :one
SELECT d.*, cu.full_name AS created_by_name, iu.full_name AS identity_checked_by_name, xu.full_name AS closed_by_name
FROM data_requests d
JOIN users cu ON cu.id = d.created_by
LEFT JOIN users iu ON iu.id = d.identity_checked_by
LEFT JOIN users xu ON xu.id = d.closed_by
WHERE d.id = @id;

-- name: LockDataRequest :one
SELECT id, reference, kind, status, subject_email, subject_phone, identity_checked_at
FROM data_requests WHERE id = @id FOR UPDATE;

-- name: ListDataRequestEvents :many
SELECT e.id, e.action, e.note, e.created_at, u.full_name AS actor_name
FROM data_request_events e JOIN users u ON u.id = e.actor_id
WHERE e.request_id = @request_id ORDER BY e.id;

-- name: CheckDataRequestIdentity :exec
UPDATE data_requests SET identity_checked_at = now(), identity_checked_by = @actor_id, identity_method = @method WHERE id = @id;

-- name: CloseDataRequest :exec
UPDATE data_requests SET status = @status, outcome = @outcome, closed_at = now(), closed_by = @actor_id WHERE id = @id AND status = 'open';
