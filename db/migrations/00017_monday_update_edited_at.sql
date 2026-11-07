-- +goose Up

-- When a student last edited a Monday update. Updates can be edited, and an
-- edit after the due date makes the update late.
ALTER TABLE app.monday_updates ADD COLUMN edited_at timestamptz;

-- +goose Down
ALTER TABLE app.monday_updates DROP COLUMN edited_at;
