-- ---------- student records and academics (Stage 11a) ----------
-- Scope: @all_schools (schools.all) or the schools in @schools (the user's
-- school assignments). A student, cohort or unit belongs to the school of
-- its programme or unit.

-- name: ListUserSchoolIDs :many
SELECT school_id FROM school_assignments WHERE user_id = @user_id;

-- name: ListUserSchools :many
SELECT s.id, s.name FROM school_assignments a JOIN schools s ON s.id = a.school_id
WHERE a.user_id = @user_id ORDER BY s.sort_order, s.name;

-- name: DeleteUserSchools :exec
DELETE FROM school_assignments WHERE user_id = @user_id;

-- name: InsertUserSchool :exec
INSERT INTO school_assignments (user_id, school_id) VALUES (@user_id, @school_id);

-- name: CountSchools :one
SELECT count(*) FROM schools WHERE id = ANY(@ids::uuid[]);

-- ---------- students ----------

-- name: StaffListStudents :many
SELECT s.id, s.student_number, s.first_name, s.middle_name, s.last_name, s.status, s.email, s.enrolled_at,
       p.title AS programme_title, c.name AS cohort_name
FROM students s
JOIN programmes p ON p.id = s.programme_id
LEFT JOIN cohorts c ON c.id = s.cohort_id
WHERE (@all_schools::boolean OR p.school_id = ANY(@schools::uuid[]))
  AND (sqlc.narg('status')::text IS NULL OR s.status = sqlc.narg('status'))
  AND (sqlc.narg('programme_id')::uuid IS NULL OR s.programme_id = sqlc.narg('programme_id'))
  AND (sqlc.narg('cohort_id')::uuid IS NULL OR s.cohort_id = sqlc.narg('cohort_id'))
  AND (sqlc.narg('search')::text IS NULL
       OR s.student_number ILIKE '%' || sqlc.narg('search') || '%'
       OR (s.first_name || ' ' || s.middle_name || ' ' || s.last_name) ILIKE '%' || sqlc.narg('search') || '%'
       OR s.email ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY s.student_number
LIMIT @max_items OFFSET @skip;

-- name: StaffCountStudents :one
SELECT count(*)
FROM students s
JOIN programmes p ON p.id = s.programme_id
WHERE (@all_schools::boolean OR p.school_id = ANY(@schools::uuid[]))
  AND (sqlc.narg('status')::text IS NULL OR s.status = sqlc.narg('status'))
  AND (sqlc.narg('programme_id')::uuid IS NULL OR s.programme_id = sqlc.narg('programme_id'))
  AND (sqlc.narg('cohort_id')::uuid IS NULL OR s.cohort_id = sqlc.narg('cohort_id'))
  AND (sqlc.narg('search')::text IS NULL
       OR s.student_number ILIKE '%' || sqlc.narg('search') || '%'
       OR (s.first_name || ' ' || s.middle_name || ' ' || s.last_name) ILIKE '%' || sqlc.narg('search') || '%'
       OR s.email ILIKE '%' || sqlc.narg('search') || '%');

-- name: StaffGetStudent :one
SELECT s.id, s.student_number, s.first_name, s.middle_name, s.last_name, s.date_of_birth, s.email, s.phone,
       s.identity_document_type, s.identity_document, s.status, s.enrolled_at, s.application_id,
       s.programme_id, p.title AS programme_title, p.school_id, s.intake_id, i.name AS intake_name,
       s.cohort_id, c.name AS cohort_name
FROM students s
JOIN programmes p ON p.id = s.programme_id
LEFT JOIN programme_intakes i ON i.id = s.intake_id
LEFT JOIN cohorts c ON c.id = s.cohort_id
WHERE s.id = @id AND (@all_schools::boolean OR p.school_id = ANY(@schools::uuid[]));

-- name: LockStudent :one
SELECT s.id, s.status, s.programme_id, s.cohort_id, p.school_id
FROM students s JOIN programmes p ON p.id = s.programme_id
WHERE s.id = @id
FOR UPDATE OF s;

-- name: SetStudentStatus :exec
UPDATE students SET status = @status WHERE id = @id;

-- name: SetStudentCohort :exec
UPDATE students SET cohort_id = @cohort_id WHERE id = @id;

-- name: InsertStudentStatusHistory :exec
INSERT INTO student_status_history (student_id, from_status, to_status, reason, changed_by)
VALUES (@student_id, @from_status, @to_status, @reason, @changed_by);

-- name: ListStudentStatusHistory :many
SELECT h.from_status, h.to_status, h.reason, h.created_at, u.full_name AS changed_by_name
FROM student_status_history h LEFT JOIN users u ON u.id = h.changed_by
WHERE h.student_id = @student_id ORDER BY h.created_at, h.id;

-- name: ListStudentGuardians :many
SELECT id, full_name, relationship, phone, email, is_primary FROM student_guardians
WHERE student_id = @student_id ORDER BY is_primary DESC, created_at;

-- name: InsertStudentGuardian :one
INSERT INTO student_guardians (student_id, full_name, relationship, phone, email, is_primary)
VALUES (@student_id, @full_name, @relationship, @phone, @email, @is_primary)
RETURNING id;

-- name: UpdateStudentGuardian :execrows
UPDATE student_guardians SET full_name = @full_name, relationship = @relationship, phone = @phone, email = @email, is_primary = @is_primary
WHERE id = @id AND student_id = @student_id;

-- name: ClearPrimaryGuardian :exec
UPDATE student_guardians SET is_primary = false WHERE student_id = @student_id AND id <> @keep AND is_primary;

-- name: DeleteStudentGuardian :execrows
DELETE FROM student_guardians WHERE id = @id AND student_id = @student_id;

-- name: ListStudentRegistrations :many
SELECT r.id, r.status, r.registered_at, r.dropped_at, o.id AS offering_id, t.name AS term_name, t.starts_on,
       un.code AS unit_code, un.title AS unit_title
FROM unit_registrations r
JOIN unit_offerings o ON o.id = r.offering_id
JOIN academic_terms t ON t.id = o.term_id
JOIN units un ON un.id = o.unit_id
WHERE r.student_id = @student_id
ORDER BY t.starts_on DESC, un.code;

-- ---------- cohorts ----------

-- name: StaffListCohorts :many
SELECT c.id, c.name, c.programme_id, p.title AS programme_title, c.intake_id, i.name AS intake_name, i.starts_on,
       (SELECT count(*) FROM students s WHERE s.cohort_id = c.id) AS students,
       (SELECT count(*) FROM students s WHERE s.cohort_id = c.id AND s.status = 'active') AS active_students
FROM cohorts c
JOIN programmes p ON p.id = c.programme_id
LEFT JOIN programme_intakes i ON i.id = c.intake_id
WHERE (@all_schools::boolean OR p.school_id = ANY(@schools::uuid[]))
  AND (sqlc.narg('programme_id')::uuid IS NULL OR c.programme_id = sqlc.narg('programme_id'))
ORDER BY p.title, i.starts_on NULLS LAST, c.name;

-- name: StaffGetCohort :one
SELECT c.id, c.name, c.programme_id, p.title AS programme_title, p.school_id, c.intake_id, i.name AS intake_name
FROM cohorts c
JOIN programmes p ON p.id = c.programme_id
LEFT JOIN programme_intakes i ON i.id = c.intake_id
WHERE c.id = @id AND (@all_schools::boolean OR p.school_id = ANY(@schools::uuid[]));

-- name: InsertCohort :one
INSERT INTO cohorts (programme_id, intake_id, name) VALUES (@programme_id, @intake_id, @name)
RETURNING id;

-- name: CohortForIntake :one
-- The cohort of a programme's intake, made the first time someone enrols.
INSERT INTO cohorts (programme_id, intake_id, name) VALUES (@programme_id, @intake_id, @name)
ON CONFLICT (programme_id, intake_id) WHERE intake_id IS NOT NULL DO UPDATE SET name = cohorts.name
RETURNING id;

-- name: ListCohortStudentIDs :many
SELECT id FROM students WHERE cohort_id = @cohort_id AND status = 'active';

-- ---------- terms ----------

-- name: ListTerms :many
SELECT id, name, starts_on, ends_on, registration_opens_on, registration_closes_on, is_sample
FROM academic_terms ORDER BY starts_on DESC;

-- name: GetTerm :one
SELECT id, name, starts_on, ends_on, registration_opens_on, registration_closes_on, is_sample
FROM academic_terms WHERE id = @id;

-- name: InsertTerm :one
INSERT INTO academic_terms (name, starts_on, ends_on, registration_opens_on, registration_closes_on)
VALUES (@name, @starts_on, @ends_on, @registration_opens_on, @registration_closes_on)
RETURNING id;

-- name: UpdateTerm :execrows
UPDATE academic_terms SET name = @name, starts_on = @starts_on, ends_on = @ends_on,
  registration_opens_on = @registration_opens_on, registration_closes_on = @registration_closes_on
WHERE id = @id;

-- ---------- units ----------

-- name: ListUnits :many
SELECT u.id, u.school_id, s.name AS school_name, u.code, u.title, u.description, u.hours, u.active, u.is_sample
FROM units u JOIN schools s ON s.id = u.school_id
WHERE (sqlc.narg('school_id')::uuid IS NULL OR u.school_id = sqlc.narg('school_id'))
  AND (sqlc.narg('search')::text IS NULL OR u.code ILIKE '%' || sqlc.narg('search') || '%' OR u.title ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY u.code;

-- name: GetUnit :one
SELECT u.id, u.school_id, s.name AS school_name, u.code, u.title, u.description, u.hours, u.active, u.is_sample
FROM units u JOIN schools s ON s.id = u.school_id WHERE u.id = @id;

-- name: InsertUnit :one
INSERT INTO units (school_id, code, title, description, hours) VALUES (@school_id, @code, @title, @description, @hours)
RETURNING id;

-- name: UpdateUnit :execrows
UPDATE units SET code = @code, title = @title, description = @description, hours = @hours, active = @active
WHERE id = @id;

-- name: GetProgrammeSchool :one
SELECT id, title, school_id FROM programmes WHERE id = @id;

-- name: ListProgrammeUnits :many
SELECT pu.unit_id, u.code, u.title, u.hours, u.school_id, pu.stage, pu.is_core
FROM programme_units pu JOIN units u ON u.id = pu.unit_id
WHERE pu.programme_id = @programme_id ORDER BY pu.stage, pu.sort_order, u.code;

-- name: DeleteProgrammeUnits :exec
DELETE FROM programme_units WHERE programme_id = @programme_id;

-- name: InsertProgrammeUnit :exec
INSERT INTO programme_units (programme_id, unit_id, stage, is_core, sort_order)
VALUES (@programme_id, @unit_id, @stage, @is_core, @sort_order);

-- name: CountUnits :one
SELECT count(*) FROM units WHERE id = ANY(@ids::uuid[]);

-- ---------- offerings, trainers, timetable, registration ----------

-- name: StaffListOfferings :many
SELECT o.id, t.id AS term_id, t.name AS term_name, t.starts_on, u.id AS unit_id, u.code AS unit_code, u.title AS unit_title,
       u.school_id,
       COALESCE((SELECT string_agg(us.full_name, ', ' ORDER BY us.full_name) FROM trainer_assignments ta
                 JOIN users us ON us.id = ta.user_id WHERE ta.offering_id = o.id), '')::text AS trainer_names,
       (SELECT count(*) FROM unit_registrations r WHERE r.offering_id = o.id AND r.status = 'registered') AS registered
FROM unit_offerings o
JOIN academic_terms t ON t.id = o.term_id
JOIN units u ON u.id = o.unit_id
WHERE (@all_schools::boolean OR u.school_id = ANY(@schools::uuid[]))
  AND (sqlc.narg('term_id')::uuid IS NULL OR o.term_id = sqlc.narg('term_id'))
  AND (sqlc.narg('unit_id')::uuid IS NULL OR o.unit_id = sqlc.narg('unit_id'))
ORDER BY t.starts_on DESC, u.code;

-- name: GetOffering :one
SELECT o.id, t.id AS term_id, t.name AS term_name, t.starts_on, t.ends_on, u.id AS unit_id, u.code AS unit_code,
       u.title AS unit_title, u.school_id
FROM unit_offerings o
JOIN academic_terms t ON t.id = o.term_id
JOIN units u ON u.id = o.unit_id
WHERE o.id = @id;

-- name: InsertOffering :one
INSERT INTO unit_offerings (term_id, unit_id) VALUES (@term_id, @unit_id) RETURNING id;

-- name: ListOfferingTrainers :many
SELECT u.id, u.full_name FROM trainer_assignments ta JOIN users u ON u.id = ta.user_id
WHERE ta.offering_id = @offering_id ORDER BY u.full_name;

-- name: DeleteOfferingTrainers :exec
DELETE FROM trainer_assignments WHERE offering_id = @offering_id;

-- name: InsertTrainerAssignment :exec
INSERT INTO trainer_assignments (offering_id, user_id, assigned_by) VALUES (@offering_id, @user_id, @assigned_by);

-- name: ListTrainers :many
-- Active staff whose roles let them teach.
SELECT DISTINCT u.id, u.full_name
FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN role_permissions rp ON rp.role_id = ur.role_id
WHERE u.kind = 'staff' AND u.status = 'active' AND rp.permission_key = 'academics.teach'
ORDER BY u.full_name;

-- name: ListTimetable :many
SELECT id, kind, weekday, on_date, starts_at, ends_at, room FROM timetable_entries
WHERE offering_id = @offering_id
ORDER BY kind, on_date NULLS FIRST, weekday NULLS FIRST, starts_at;

-- name: InsertTimetableEntry :one
INSERT INTO timetable_entries (offering_id, kind, weekday, on_date, starts_at, ends_at, room)
VALUES (@offering_id, @kind, @weekday, @on_date, @starts_at, @ends_at, @room)
RETURNING id;

-- name: DeleteTimetableEntry :execrows
DELETE FROM timetable_entries WHERE id = @id AND offering_id = @offering_id;

-- name: ListClassList :many
-- The class list: what a trainer needs, and nothing more (no contact details).
SELECT r.id, r.status, r.registered_at, s.id AS student_id, s.student_number, s.first_name, s.middle_name, s.last_name,
       c.name AS cohort_name
FROM unit_registrations r
JOIN students s ON s.id = r.student_id
LEFT JOIN cohorts c ON c.id = s.cohort_id
WHERE r.offering_id = @offering_id
ORDER BY r.status, s.last_name, s.first_name, s.student_number;

-- name: EligibleStudents :many
-- Of @ids, the active students whose programme includes @unit_id.
SELECT s.id FROM students s
JOIN programme_units pu ON pu.programme_id = s.programme_id AND pu.unit_id = @unit_id
WHERE s.id = ANY(@ids::uuid[]) AND s.status = 'active';

-- name: RegisterStudent :execrows
-- Registers, or registers again after a drop; 0 rows if already registered.
INSERT INTO unit_registrations (offering_id, student_id, registered_by)
VALUES (@offering_id, @student_id, @registered_by)
ON CONFLICT (offering_id, student_id) DO UPDATE
  SET status = 'registered', registered_at = now(), registered_by = EXCLUDED.registered_by, dropped_at = NULL, dropped_by = NULL
  WHERE unit_registrations.status = 'dropped';

-- name: DropRegistration :execrows
UPDATE unit_registrations SET status = 'dropped', dropped_at = now(), dropped_by = @dropped_by
WHERE id = @id AND offering_id = @offering_id AND status = 'registered';

-- ---------- teaching (academics.teach) ----------

-- name: ListMyOfferings :many
SELECT o.id, t.name AS term_name, t.starts_on, t.ends_on, u.code AS unit_code, u.title AS unit_title,
       (SELECT count(*) FROM unit_registrations r WHERE r.offering_id = o.id AND r.status = 'registered') AS registered
FROM trainer_assignments ta
JOIN unit_offerings o ON o.id = ta.offering_id
JOIN academic_terms t ON t.id = o.term_id
JOIN units u ON u.id = o.unit_id
WHERE ta.user_id = @user_id
ORDER BY t.starts_on DESC, u.code;

-- name: IsAssignedTrainer :one
SELECT EXISTS (SELECT 1 FROM trainer_assignments WHERE offering_id = @offering_id AND user_id = @user_id);

-- name: ListMyTimetable :many
SELECT e.id, e.kind, e.weekday, e.on_date, e.starts_at, e.ends_at, e.room, o.id AS offering_id,
       u.code AS unit_code, u.title AS unit_title, t.name AS term_name
FROM trainer_assignments ta
JOIN unit_offerings o ON o.id = ta.offering_id
JOIN academic_terms t ON t.id = o.term_id
JOIN units u ON u.id = o.unit_id
JOIN timetable_entries e ON e.offering_id = o.id
WHERE ta.user_id = @user_id AND t.ends_on >= @today::date
ORDER BY e.kind, e.on_date NULLS FIRST, e.weekday NULLS FIRST, e.starts_at;
