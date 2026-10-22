-- Enquiry inbox (Stage 8a). Fee enquiries are emailed to the finance office,
-- so Finance Officers also see and manage the inbox (owner, 2026-10-23).

-- +goose Up
INSERT INTO role_permissions (role_id, permission_key)
SELECT r.id, p.key FROM roles r CROSS JOIN permissions p
WHERE r.key = 'finance_officer' AND p.key IN ('enquiries.view', 'enquiries.manage')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions
WHERE role_id = (SELECT id FROM roles WHERE key = 'finance_officer')
  AND permission_key IN ('enquiries.view', 'enquiries.manage');
