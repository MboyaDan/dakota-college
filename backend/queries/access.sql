-- Users and roles administration (Stage 4).

-- name: LockSuperAdminRole :one
-- Serialises every change that could remove the last active Super Admin:
-- callers take this row lock first, inside their transaction.
SELECT id FROM roles WHERE key = 'super_admin' FOR UPDATE;

-- name: CountActiveSuperAdmins :one
SELECT count(DISTINCT u.id)
FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN roles r ON r.id = ur.role_id
WHERE r.key = 'super_admin' AND u.status = 'active';

-- name: CountSuperAdmins :one
-- Any status: the first-run setup refuses if a Super Admin was ever created.
SELECT count(*) FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE r.key = 'super_admin';

-- name: GetUserByID :one
SELECT id, email, full_name, kind, status, last_login_at, created_at FROM users WHERE id = @id;

-- name: ListUsers :many
SELECT u.id, u.email, u.full_name, u.kind, u.status, u.last_login_at, u.created_at,
       COALESCE(array_agg(r.key ORDER BY r.key) FILTER (WHERE r.key IS NOT NULL), '{}')::text[] AS role_keys
FROM users u
LEFT JOIN user_roles ur ON ur.user_id = u.id
LEFT JOIN roles r ON r.id = ur.role_id
WHERE u.kind = 'staff'
GROUP BY u.id
ORDER BY u.full_name;

-- name: ListUserRoleKeys :many
SELECT r.key FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE ur.user_id = @user_id ORDER BY r.key;

-- name: CreateStaffUser :one
INSERT INTO users (email, full_name, kind) VALUES (@email, @full_name, 'staff') RETURNING id;

-- name: DeleteUserRoles :exec
DELETE FROM user_roles WHERE user_id = @user_id;

-- name: SetUserStatus :exec
UPDATE users SET status = @status WHERE id = @id;

-- name: ListRoles :many
SELECT r.id, r.key, r.name, r.description, r.is_system,
       COALESCE(array_agg(rp.permission_key ORDER BY rp.permission_key) FILTER (WHERE rp.permission_key IS NOT NULL), '{}')::text[] AS permission_keys,
       (SELECT count(*) FROM user_roles ur WHERE ur.role_id = r.id) AS user_count
FROM roles r
LEFT JOIN role_permissions rp ON rp.role_id = r.id
GROUP BY r.id
ORDER BY r.name;

-- name: GetRoleByID :one
SELECT id, key, name, description, is_system FROM roles WHERE id = @id;

-- name: GetRolesByKeys :many
SELECT id, key FROM roles WHERE key = ANY(@keys::text[]);

-- name: CreateRole :one
INSERT INTO roles (key, name, description) VALUES (@key, @name, @description) RETURNING id;

-- name: DeleteRolePermissions :exec
DELETE FROM role_permissions WHERE role_id = @role_id;

-- name: InsertRolePermission :exec
INSERT INTO role_permissions (role_id, permission_key) VALUES (@role_id, @permission_key);

-- name: CountUsersWithRole :one
SELECT count(*) FROM user_roles WHERE role_id = @role_id;

-- name: DeleteRole :exec
DELETE FROM roles WHERE id = @id;

-- name: ListPermissions :many
SELECT key, module, description FROM permissions ORDER BY module, key;

-- name: CountPermissions :one
SELECT count(*) FROM permissions WHERE key = ANY(@keys::text[]);

-- name: RolesGrantPermission :one
-- Whether any of the given roles carries the permission.
SELECT EXISTS (
  SELECT 1 FROM role_permissions rp JOIN roles r ON r.id = rp.role_id
  WHERE r.key = ANY(@role_keys::text[]) AND rp.permission_key = @permission_key
);
