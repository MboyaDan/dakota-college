-- name: InsertEnquiry :one
INSERT INTO enquiries (full_name, email, phone, query_type, programme_id, message, source, ip, user_agent)
VALUES (@full_name, @email, @phone, @query_type, @programme_id, @message, 'website', @ip, @user_agent)
RETURNING id, created_at;

-- name: GetPublishedProgrammeBySlug :one
SELECT p.id, p.title
FROM programmes p
JOIN schools s ON s.id = p.school_id
WHERE p.slug = @slug AND p.status = 'published' AND s.status = 'published';

-- name: GetSiteSettings :many
SELECT key, value FROM site_settings WHERE key = ANY(@keys::text[]);

-- ---------- staff inbox (Stage 8a) ----------
-- Filters are optional: a NULL argument means "any". Types select an office
-- (admissions, finance, general); assigned_to and unassigned narrow by owner.

-- name: StaffListEnquiries :many
SELECT e.id, e.full_name, e.email, e.phone, e.query_type, e.status, e.created_at, e.updated_at,
       e.assigned_to, u.full_name AS assignee_name, p.title AS programme_title
FROM enquiries e
LEFT JOIN users u ON u.id = e.assigned_to
LEFT JOIN programmes p ON p.id = e.programme_id
WHERE (sqlc.narg('status')::text IS NULL OR e.status = sqlc.narg('status'))
  AND (sqlc.narg('types')::text[] IS NULL OR e.query_type = ANY(sqlc.narg('types')::text[]))
  AND (sqlc.narg('assigned_to')::uuid IS NULL OR e.assigned_to = sqlc.narg('assigned_to'))
  AND (NOT @unassigned::boolean OR e.assigned_to IS NULL)
  AND (sqlc.narg('search')::text IS NULL
       OR e.full_name ILIKE '%' || sqlc.narg('search') || '%'
       OR e.email ILIKE '%' || sqlc.narg('search') || '%'
       OR e.phone ILIKE '%' || sqlc.narg('search') || '%'
       OR e.message ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY e.created_at DESC
LIMIT @max_items OFFSET @skip;

-- name: StaffCountEnquiries :one
SELECT count(*) FROM enquiries e
WHERE (sqlc.narg('status')::text IS NULL OR e.status = sqlc.narg('status'))
  AND (sqlc.narg('types')::text[] IS NULL OR e.query_type = ANY(sqlc.narg('types')::text[]))
  AND (sqlc.narg('assigned_to')::uuid IS NULL OR e.assigned_to = sqlc.narg('assigned_to'))
  AND (NOT @unassigned::boolean OR e.assigned_to IS NULL)
  AND (sqlc.narg('search')::text IS NULL
       OR e.full_name ILIKE '%' || sqlc.narg('search') || '%'
       OR e.email ILIKE '%' || sqlc.narg('search') || '%'
       OR e.phone ILIKE '%' || sqlc.narg('search') || '%'
       OR e.message ILIKE '%' || sqlc.narg('search') || '%');

-- name: StaffGetEnquiry :one
SELECT e.id, e.full_name, e.email, e.phone, e.query_type, e.message, e.source, e.status, e.created_at,
       e.updated_at, e.closed_at, e.assigned_to, u.full_name AS assignee_name,
       p.slug AS programme_slug, p.title AS programme_title
FROM enquiries e
LEFT JOIN users u ON u.id = e.assigned_to
LEFT JOIN programmes p ON p.id = e.programme_id
WHERE e.id = @id;

-- name: LockEnquiry :one
SELECT status, assigned_to FROM enquiries WHERE id = @id FOR UPDATE;

-- name: StaffListEnquiryNotes :many
SELECT n.id, n.body, n.created_at, u.full_name AS author_name
FROM enquiry_notes n
JOIN users u ON u.id = n.author_id
WHERE n.enquiry_id = @enquiry_id
ORDER BY n.created_at;

-- name: StaffEnquiryHistory :many
-- The audit log is the history: who assigned, changed status or added a note.
SELECT a.occurred_at, a.action, a.details, u.full_name AS actor_name
FROM audit_logs a
LEFT JOIN users u ON u.id = a.actor_user_id
WHERE a.entity_type = 'enquiry' AND a.entity_id = @enquiry_id::text
ORDER BY a.occurred_at, a.id;

-- name: SetEnquiryAssignee :exec
UPDATE enquiries SET assigned_to = @assigned_to WHERE id = @id;

-- name: SetEnquiryStatus :exec
UPDATE enquiries
SET status = @status, closed_at = CASE WHEN @status = 'closed' THEN now() ELSE NULL END
WHERE id = @id;

-- name: InsertEnquiryNote :one
INSERT INTO enquiry_notes (enquiry_id, author_id, body) VALUES (@enquiry_id, @author_id, @body)
RETURNING id, created_at;

-- name: ListEnquiryAssignees :many
-- Active staff whose roles let them see the inbox.
SELECT DISTINCT u.id, u.full_name, u.email
FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN role_permissions rp ON rp.role_id = ur.role_id
WHERE u.kind = 'staff' AND u.status = 'active' AND rp.permission_key = 'enquiries.view'
ORDER BY u.full_name;

-- name: CountOpenEnquiries :one
SELECT count(*) FILTER (WHERE status = 'new') AS new_count,
       count(*) FILTER (WHERE status <> 'closed' AND assigned_to = @user_id) AS mine_count
FROM enquiries;
