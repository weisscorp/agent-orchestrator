-- +goose Up
-- NULL identifies legacy sessions. The versioned JSON is fixed at creation.
ALTER TABLE sessions ADD COLUMN workspace_scope TEXT;

-- +goose Down
ALTER TABLE sessions DROP COLUMN workspace_scope;
