-- name: UpsertUserByEmail :one
-- Used by the dev seed and, from Stage 4, the first-run setup command.
INSERT INTO users (email, full_name, kind, password_hash, password_changed_at)
VALUES (@email, @full_name, @kind, @password_hash, now())
ON CONFLICT (email) DO UPDATE
  SET full_name = EXCLUDED.full_name,
      kind = EXCLUDED.kind,
      password_hash = EXCLUDED.password_hash,
      password_changed_at = now()
RETURNING id;

-- name: GetRoleByKey :one
SELECT id, key, name, description, is_system FROM roles WHERE key = @key;

-- name: AssignRole :exec
INSERT INTO user_roles (user_id, role_id, granted_by)
VALUES (@user_id, @role_id, @granted_by)
ON CONFLICT (user_id, role_id) DO NOTHING;

-- name: ListUserPermissionKeys :many
-- Effective permissions of an active user, across all their roles.
SELECT DISTINCT rp.permission_key
FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN role_permissions rp ON rp.role_id = ur.role_id
WHERE u.id = @user_id AND u.status = 'active'
ORDER BY rp.permission_key;

-- name: ListRolesWithPermission :many
SELECT r.key FROM roles r
JOIN role_permissions rp ON rp.role_id = r.id
WHERE rp.permission_key = @permission_key
ORDER BY r.key;

-- name: InsertAuditLog :exec
INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, request_id, ip, user_agent, details)
VALUES (@actor_user_id, @action, @entity_type, @entity_id, @request_id, @ip, @user_agent, @details);
