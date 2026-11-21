-- +goose Up
ALTER TABLE app.job_applications
  ADD COLUMN share_progress boolean NOT NULL DEFAULT false;

-- Existing histories were private. Share new applications by default.
ALTER TABLE app.job_applications ALTER COLUMN share_progress SET DEFAULT true;

-- +goose Down
ALTER TABLE app.job_applications DROP COLUMN share_progress;
