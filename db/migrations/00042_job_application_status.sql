-- +goose Up
ALTER TABLE app.job_applications
  ADD COLUMN status text NOT NULL DEFAULT 'active'
  CHECK (status IN ('active', 'inactive', 'offer'));

-- +goose Down
ALTER TABLE app.job_applications DROP COLUMN status;
