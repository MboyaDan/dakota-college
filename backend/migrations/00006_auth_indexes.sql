-- Index for finding a user's unused password reset links (Stage 4).

-- +goose Up
CREATE INDEX password_resets_active_idx ON password_resets (user_id) WHERE used_at IS NULL;

-- +goose Down
DROP INDEX password_resets_active_idx;
