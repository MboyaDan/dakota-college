-- Dashboards and count exports (Stage 16b, brief 7.2). Registrar and Finance
-- Officer may export the figures they can already see (D-183); exports with
-- personal data, and their approval, come later (D-184).

-- +goose Up
INSERT INTO role_permissions (role_id, permission_key)
SELECT r.id, 'reports.export' FROM roles r WHERE r.key IN ('registrar', 'finance_officer')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions
WHERE permission_key = 'reports.export'
  AND role_id IN (SELECT id FROM roles WHERE key IN ('registrar', 'finance_officer'));
