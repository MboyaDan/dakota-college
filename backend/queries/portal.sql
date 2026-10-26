-- ---------- student portal (Stage 12a) ----------
-- Student queries take the student id found from the signed-in user, never
-- an id from the request, so a student can only ever reach their own data.

-- name: GetStudentLoginEmail :one
SELECT u.email FROM students s JOIN users u ON u.id = s.user_id
WHERE s.student_number = @student_number AND u.kind = 'student';

-- name: GetStudentByUser :one
SELECT s.id, s.student_number, s.first_name, s.middle_name, s.last_name, s.date_of_birth, s.email, s.phone,
       s.identity_document_type, s.identity_document, s.status, s.enrolled_at,
       s.programme_id, p.title AS programme_title, p.school_id, sc.name AS school_name,
       i.name AS intake_name, s.cohort_id, c.name AS cohort_name
FROM students s
JOIN programmes p ON p.id = s.programme_id
JOIN schools sc ON sc.id = p.school_id
LEFT JOIN programme_intakes i ON i.id = s.intake_id
LEFT JOIN cohorts c ON c.id = s.cohort_id
WHERE s.user_id = @user_id;

-- name: SetStudentPhone :exec
UPDATE students SET phone = @phone WHERE id = @id;

-- name: InsertProfileChangeRequest :one
INSERT INTO profile_change_requests (student_id, old_values, new_values, reason)
VALUES (@student_id, @old_values, @new_values, @reason)
RETURNING id;

-- name: ListOwnProfileChangeRequests :many
SELECT id, old_values, new_values, reason, status, decision, decided_at, created_at
FROM profile_change_requests WHERE student_id = @student_id ORDER BY created_at DESC LIMIT 20;

-- name: StaffListProfileChangeRequests :many
SELECT r.id, r.old_values, r.new_values, r.reason, r.status, r.decision, r.decided_at, r.created_at,
       s.id AS student_id, s.student_number, s.first_name, s.middle_name, s.last_name, u.full_name AS decided_by_name
FROM profile_change_requests r
JOIN students s ON s.id = r.student_id
JOIN programmes p ON p.id = s.programme_id
LEFT JOIN users u ON u.id = r.decided_by
WHERE (@all_schools::boolean OR p.school_id = ANY(@schools::uuid[]))
  AND (sqlc.narg('status')::text IS NULL OR r.status = sqlc.narg('status'))
ORDER BY r.created_at DESC
LIMIT 100;

-- name: LockProfileChangeRequest :one
SELECT r.id, r.student_id, r.new_values, r.status, p.school_id, s.user_id, s.email, s.first_name
FROM profile_change_requests r
JOIN students s ON s.id = r.student_id
JOIN programmes p ON p.id = s.programme_id
WHERE r.id = @id
FOR UPDATE OF r;

-- name: DecideProfileChangeRequest :exec
UPDATE profile_change_requests SET status = @status, decision = @decision, decided_by = @decided_by, decided_at = now()
WHERE id = @id;

-- name: ApplyProfileChange :exec
UPDATE students SET
  first_name = COALESCE(sqlc.narg('first_name')::text, first_name),
  middle_name = COALESCE(sqlc.narg('middle_name')::text, middle_name),
  last_name = COALESCE(sqlc.narg('last_name')::text, last_name),
  date_of_birth = COALESCE(sqlc.narg('date_of_birth')::date, date_of_birth),
  identity_document = COALESCE(sqlc.narg('identity_document')::text, identity_document)
WHERE id = @id;

-- name: SyncStudentAccountName :exec
UPDATE users u SET full_name = trim(s.first_name || ' ' || s.middle_name || ' ' || s.last_name)
FROM students s WHERE s.id = @student_id AND u.id = s.user_id;

-- name: StudentRegistrationOptions :many
-- Classes this student may register for today: terms whose registration
-- window is open (Nairobi @today), units of their programme.
SELECT o.id, t.id AS term_id, t.name AS term_name, t.registration_closes_on, u.code AS unit_code, u.title AS unit_title,
       pu.stage, pu.is_core, r.status AS registration_status
FROM unit_offerings o
JOIN academic_terms t ON t.id = o.term_id
JOIN units u ON u.id = o.unit_id
JOIN programme_units pu ON pu.unit_id = o.unit_id AND pu.programme_id = @programme_id
LEFT JOIN unit_registrations r ON r.offering_id = o.id AND r.student_id = @student_id
WHERE t.registration_opens_on <= @today::date AND t.registration_closes_on >= @today::date
ORDER BY t.starts_on, pu.stage, u.code;

-- name: StudentOfferingOpen :one
-- The offering, if this student may register for it or drop it today.
SELECT o.id FROM unit_offerings o
JOIN academic_terms t ON t.id = o.term_id
JOIN programme_units pu ON pu.unit_id = o.unit_id AND pu.programme_id = @programme_id
WHERE o.id = @offering_id
  AND t.registration_opens_on <= @today::date AND t.registration_closes_on >= @today::date;

-- name: GetOwnRegistration :one
SELECT id, status FROM unit_registrations WHERE offering_id = @offering_id AND student_id = @student_id;

-- name: StudentTimetable :many
SELECT e.id, e.kind, e.weekday, e.on_date, e.starts_at, e.ends_at, e.room, o.id AS offering_id,
       u.code AS unit_code, u.title AS unit_title, t.name AS term_name
FROM unit_registrations r
JOIN unit_offerings o ON o.id = r.offering_id
JOIN academic_terms t ON t.id = o.term_id
JOIN units u ON u.id = o.unit_id
JOIN timetable_entries e ON e.offering_id = o.id
WHERE r.student_id = @student_id AND r.status = 'registered' AND t.ends_on >= @today::date
ORDER BY e.kind, e.on_date NULLS FIRST, e.weekday NULLS FIRST, e.starts_at;

-- ---------- notices ----------

-- name: StudentNotices :many
-- What this student is shown: notices to everyone, their school, their
-- cohort or a class they are registered for, not withdrawn or past their end.
SELECT n.id, n.audience, n.title, n.body, n.ends_on, n.created_at, u.full_name AS author_name,
       COALESCE(sc.name, c.name, un.code || ' ' || un.title, '')::text AS audience_name
FROM notices n
JOIN users u ON u.id = n.created_by
LEFT JOIN schools sc ON sc.id = n.school_id
LEFT JOIN cohorts c ON c.id = n.cohort_id
LEFT JOIN unit_offerings o ON o.id = n.offering_id
LEFT JOIN units un ON un.id = o.unit_id
WHERE n.withdrawn_at IS NULL AND (n.ends_on IS NULL OR n.ends_on >= @today::date)
  AND (n.audience = 'all'
       OR n.school_id = @school_id
       OR (n.cohort_id IS NOT NULL AND n.cohort_id = sqlc.narg('cohort_id')::uuid)
       OR n.offering_id IN (SELECT offering_id FROM unit_registrations WHERE student_id = @student_id AND status = 'registered'))
ORDER BY n.created_at DESC
LIMIT @max_items;

-- name: StaffListNotices :many
-- Notices within scope (by school, cohort's programme or class unit); with
-- @offering_id set, only that class's.
SELECT n.id, n.audience, n.title, n.body, n.ends_on, n.created_at, n.withdrawn_at, n.created_by, u.full_name AS author_name,
       COALESCE(sc.name, c.name, un.code || ' ' || un.title, '')::text AS audience_name
FROM notices n
JOIN users u ON u.id = n.created_by
LEFT JOIN schools sc ON sc.id = n.school_id
LEFT JOIN cohorts c ON c.id = n.cohort_id
LEFT JOIN programmes cp ON cp.id = c.programme_id
LEFT JOIN unit_offerings o ON o.id = n.offering_id
LEFT JOIN units un ON un.id = o.unit_id
WHERE (sqlc.narg('offering_id')::uuid IS NULL OR n.offering_id = sqlc.narg('offering_id'))
  AND (@all_schools::boolean
       OR n.audience = 'all'
       OR n.school_id = ANY(@schools::uuid[])
       OR cp.school_id = ANY(@schools::uuid[])
       OR un.school_id = ANY(@schools::uuid[]))
ORDER BY n.created_at DESC
LIMIT 100;

-- name: InsertNotice :one
INSERT INTO notices (audience, school_id, cohort_id, offering_id, title, body, ends_on, created_by)
VALUES (@audience, @school_id, @cohort_id, @offering_id, @title, @body, @ends_on, @created_by)
RETURNING id;

-- name: LockNotice :one
SELECT n.id, n.audience, n.school_id, n.offering_id, n.created_by, n.withdrawn_at,
       cp.school_id AS cohort_school_id, un.school_id AS unit_school_id
FROM notices n
LEFT JOIN cohorts c ON c.id = n.cohort_id
LEFT JOIN programmes cp ON cp.id = c.programme_id
LEFT JOIN unit_offerings o ON o.id = n.offering_id
LEFT JOIN units un ON un.id = o.unit_id
WHERE n.id = @id
FOR UPDATE OF n;

-- name: WithdrawNotice :exec
UPDATE notices SET withdrawn_at = now(), withdrawn_by = @withdrawn_by WHERE id = @id;

-- ---------- service requests ----------

-- name: InsertServiceRequest :one
INSERT INTO service_requests (student_id, kind, details) VALUES (@student_id, @kind, @details)
RETURNING id;

-- name: CountOpenServiceRequests :one
SELECT count(*) FROM service_requests WHERE student_id = @student_id AND status IN ('open', 'in_progress');

-- name: ListOwnServiceRequests :many
SELECT id, kind, details, status, response, created_at, updated_at
FROM service_requests WHERE student_id = @student_id ORDER BY created_at DESC LIMIT 50;

-- name: StaffListServiceRequests :many
SELECT r.id, r.kind, r.details, r.status, r.response, r.created_at, r.updated_at,
       s.id AS student_id, s.student_number, s.first_name, s.middle_name, s.last_name, u.full_name AS handled_by_name
FROM service_requests r
JOIN students s ON s.id = r.student_id
JOIN programmes p ON p.id = s.programme_id
LEFT JOIN users u ON u.id = r.handled_by
WHERE (@all_schools::boolean OR p.school_id = ANY(@schools::uuid[]))
  AND (sqlc.narg('status')::text IS NULL OR r.status = sqlc.narg('status'))
ORDER BY r.created_at DESC
LIMIT 100;

-- name: LockServiceRequest :one
SELECT r.id, r.kind, r.status, p.school_id, s.email, s.first_name
FROM service_requests r
JOIN students s ON s.id = r.student_id
JOIN programmes p ON p.id = s.programme_id
WHERE r.id = @id
FOR UPDATE OF r;

-- name: UpdateServiceRequest :exec
UPDATE service_requests SET status = @status, response = @response, handled_by = @handled_by WHERE id = @id;
