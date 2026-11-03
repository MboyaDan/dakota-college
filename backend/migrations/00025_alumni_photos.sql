-- Photos on the Alumni page (owner request, 2026-10-30). Like the home page
-- photo (00022), a gallery photo is ticked to appear there; only published
-- photos, so only with recorded consent (NN-08), are shown.

-- +goose Up
ALTER TABLE gallery_items ADD COLUMN alumni boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE gallery_items DROP COLUMN alumni;
