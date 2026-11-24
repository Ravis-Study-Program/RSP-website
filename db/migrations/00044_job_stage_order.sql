-- +goose Up
-- Existing histories retain chronological order until explicitly reordered.
-- New stages append after manually ordered stages.
ALTER TABLE app.job_application_stages ADD COLUMN sort_order integer;

-- +goose Down
ALTER TABLE app.job_application_stages DROP COLUMN sort_order;
