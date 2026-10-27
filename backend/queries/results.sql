-- ---------- results (Stage 13a) ----------

-- name: GetAssessmentScheme :one
SELECT programme_id, kind, components, pass_mark, grade_bands, updated_at FROM assessment_schemes WHERE programme_id = @programme_id;

-- name: UpsertAssessmentScheme :exec
INSERT INTO assessment_schemes (programme_id, kind, components, pass_mark, grade_bands, updated_by, updated_at)
VALUES (@programme_id, @kind, @components, @pass_mark, @grade_bands, @updated_by, now())
ON CONFLICT (programme_id) DO UPDATE SET kind = EXCLUDED.kind, components = EXCLUDED.components, pass_mark = EXCLUDED.pass_mark,
  grade_bands = EXCLUDED.grade_bands, updated_by = EXCLUDED.updated_by, updated_at = now();

-- name: ProgrammeForOffering :one
-- The programme whose scheme a class uses: the first (by title) that lists its unit.
SELECT p.id FROM unit_offerings o
JOIN programme_units pu ON pu.unit_id = o.unit_id
JOIN programmes p ON p.id = pu.programme_id
WHERE o.id = @offering_id
ORDER BY p.title, p.id
LIMIT 1;

-- name: GetSheetByOffering :one
SELECT id, offering_id, status, scheme, created_at, updated_at FROM result_sheets WHERE offering_id = @offering_id;

-- name: InsertSheet :exec
INSERT INTO result_sheets (offering_id, scheme) VALUES (@offering_id, @scheme) ON CONFLICT (offering_id) DO NOTHING;

-- name: LockSheet :one
SELECT s.id, s.offering_id, s.status, s.scheme, u.school_id, u.code AS unit_code
FROM result_sheets s
JOIN unit_offerings o ON o.id = s.offering_id
JOIN units u ON u.id = o.unit_id
WHERE s.id = @id
FOR UPDATE OF s;

-- name: SetSheetScheme :exec
UPDATE result_sheets SET scheme = @scheme WHERE id = @id;

-- name: SetSheetStatus :exec
UPDATE result_sheets SET status = @status WHERE id = @id;

-- name: InsertSheetEvent :exec
INSERT INTO result_sheet_events (sheet_id, from_status, to_status, reason, actor_id)
VALUES (@sheet_id, @from_status, @to_status, @reason, @actor_id);

-- name: ListSheetEvents :many
SELECT e.from_status, e.to_status, e.reason, e.created_at, u.full_name AS actor_name
FROM result_sheet_events e JOIN users u ON u.id = e.actor_id
WHERE e.sheet_id = @sheet_id ORDER BY e.created_at, e.id;

-- name: CountMarks :one
SELECT count(*) FROM marks WHERE sheet_id = @sheet_id;

-- name: InsertMark :exec
INSERT INTO marks (sheet_id, student_id, component, score, outcome, reason, entered_by)
VALUES (@sheet_id, @student_id, @component, @score, @outcome, @reason, @entered_by);

-- name: CurrentMarks :many
-- The latest entry per student and component.
SELECT DISTINCT ON (m.student_id, m.component) m.student_id, m.component, m.score, m.outcome, m.entered_at, u.full_name AS entered_by_name
FROM marks m JOIN users u ON u.id = m.entered_by
WHERE m.sheet_id = @sheet_id
ORDER BY m.student_id, m.component, m.id DESC;

-- name: ListClassStudents :many
-- Students registered for the class, the rows of its result sheet.
SELECT s.id, s.student_number, s.first_name, s.middle_name, s.last_name
FROM unit_registrations r JOIN students s ON s.id = r.student_id
WHERE r.offering_id = @offering_id AND r.status = 'registered'
ORDER BY s.last_name, s.first_name, s.student_number;

-- name: StaffListSheets :many
SELECT s.id, s.status, s.updated_at, o.id AS offering_id, t.name AS term_name, t.starts_on, u.code AS unit_code, u.title AS unit_title,
       (SELECT count(*) FROM unit_registrations r WHERE r.offering_id = o.id AND r.status = 'registered') AS students,
       COALESCE((SELECT max(version) FROM result_releases rr WHERE rr.sheet_id = s.id), 0)::int AS released_version
FROM result_sheets s
JOIN unit_offerings o ON o.id = s.offering_id
JOIN academic_terms t ON t.id = o.term_id
JOIN units u ON u.id = o.unit_id
WHERE (@all_schools::boolean OR u.school_id = ANY(@schools::uuid[]))
  AND s.status = ANY(@statuses::text[])
ORDER BY s.updated_at DESC
LIMIT 200;

-- name: GetSheetOffering :one
SELECT s.id, s.offering_id, s.status, s.scheme, s.updated_at, u.school_id, u.code AS unit_code, u.title AS unit_title,
       t.name AS term_name, t.starts_on, t.ends_on
FROM result_sheets s
JOIN unit_offerings o ON o.id = s.offering_id
JOIN units u ON u.id = o.unit_id
JOIN academic_terms t ON t.id = o.term_id
WHERE s.id = @id;

-- name: NextReleaseVersion :one
SELECT (COALESCE(max(version), 0) + 1)::int FROM result_releases WHERE sheet_id = @sheet_id;

-- name: InsertRelease :exec
INSERT INTO result_releases (sheet_id, version, results, released_by) VALUES (@sheet_id, @version, @results, @released_by);

-- name: StudentReleasedResults :many
-- The student's row of the newest release of each class (AC-09).
WITH latest AS (
  SELECT DISTINCT ON (sheet_id) sheet_id, version, results, released_at
  FROM result_releases ORDER BY sheet_id, version DESC
)
SELECT l.sheet_id, l.version, l.released_at, t.name AS term_name, t.starts_on, u.code AS unit_code, u.title AS unit_title,
       u.hours, e.value::text AS result
FROM latest l
JOIN result_sheets s ON s.id = l.sheet_id
JOIN unit_offerings o ON o.id = s.offering_id
JOIN academic_terms t ON t.id = o.term_id
JOIN units u ON u.id = o.unit_id
CROSS JOIN LATERAL jsonb_array_elements(l.results) AS e(value)
WHERE e.value->>'student_id' = @student_id::text
ORDER BY t.starts_on, u.code;

-- name: InsertTranscript :one
INSERT INTO transcripts (number, student_id, file_id, service_request_id, issued_by)
VALUES (@number, @student_id, @file_id, @service_request_id, @issued_by)
RETURNING id, issued_at;

-- name: ListTranscripts :many
SELECT t.id, t.number, t.issued_at, u.full_name AS issued_by_name
FROM transcripts t JOIN users u ON u.id = t.issued_by
WHERE t.student_id = @student_id ORDER BY t.issued_at DESC;

-- name: GetTranscriptFile :one
SELECT file_id FROM transcripts WHERE id = @id AND student_id = @student_id;
