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
