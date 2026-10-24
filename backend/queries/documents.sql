-- ---------- application documents, submission, file links (Stage 9b) ----------

-- name: InsertPrivateFile :one
INSERT INTO files (bucket, object_key, original_name, content_type, size_bytes, sha256, scan_status, uploaded_by_user_id)
VALUES (@bucket, @object_key, @original_name, @content_type, @size_bytes, @sha256, @scan_status, @uploaded_by_user_id)
RETURNING id;

-- name: InsertApplicationDocument :one
INSERT INTO application_documents (application_id, kind, file_id) VALUES (@application_id, @kind, @file_id)
RETURNING id;

-- name: ListApplicationDocuments :many
SELECT d.id, d.kind, d.created_at, f.original_name, f.content_type, f.size_bytes, f.scan_status
FROM application_documents d
JOIN files f ON f.id = d.file_id
WHERE d.application_id = @application_id AND f.deleted_at IS NULL
ORDER BY d.created_at;

-- name: GetApplicationDocument :one
SELECT d.id, d.file_id, f.bucket, f.object_key
FROM application_documents d
JOIN files f ON f.id = d.file_id
WHERE d.id = @id AND d.application_id = @application_id AND f.deleted_at IS NULL;

-- name: DeleteApplicationDocument :exec
DELETE FROM application_documents WHERE id = @id;

-- name: MarkFileDeleted :exec
UPDATE files SET deleted_at = now() WHERE id = @id;

-- name: GetSubmittedApplicationDocument :one
-- Staff access: only documents of applications that were submitted.
SELECT d.id, d.file_id, a.reference
FROM application_documents d
JOIN applications a ON a.id = d.application_id
JOIN files f ON f.id = d.file_id
WHERE d.id = @id AND d.application_id = @application_id AND a.submitted_at IS NOT NULL AND f.deleted_at IS NULL;

-- name: CreateFileLink :exec
INSERT INTO file_links (token_hash, file_id, issued_to, expires_at)
VALUES (@token_hash, @file_id, @issued_to, @expires_at);

-- name: GetFileLink :one
SELECT l.file_id, l.issued_to, l.expires_at, f.bucket, f.object_key, f.original_name, f.content_type, f.size_bytes, f.deleted_at
FROM file_links l
JOIN files f ON f.id = l.file_id
WHERE l.token_hash = @token_hash;

-- name: SubmitApplication :execrows
UPDATE applications
SET status = 'submitted', submitted_at = now(), declaration_accepted_at = now(), submitted_snapshot = @snapshot
WHERE id = @id AND status = 'draft';

-- name: ListStatusHistory :many
SELECT from_status, to_status, reason, created_at
FROM application_status_history WHERE application_id = @application_id ORDER BY created_at, id;

-- name: ClaimIdempotencyKey :execrows
-- Claims a key; 0 rows means it was used before (or is being used now).
INSERT INTO idempotency_records (scope, key, request_hash, expires_at)
VALUES (@scope, @key, @request_hash, @expires_at)
ON CONFLICT (scope, key) DO NOTHING;

-- name: GetIdempotencyRecord :one
-- Waits for a concurrent request with the same key to finish.
SELECT request_hash, response_status, response_body, completed_at
FROM idempotency_records WHERE scope = @scope AND key = @key
FOR UPDATE;

-- name: CompleteIdempotencyKey :exec
UPDATE idempotency_records
SET response_status = @response_status, response_body = @response_body, completed_at = now()
WHERE scope = @scope AND key = @key;
