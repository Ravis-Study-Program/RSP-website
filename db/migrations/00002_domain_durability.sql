-- +goose Up

ALTER TABLE app.season_weeks
  ADD COLUMN resource_url text NOT NULL DEFAULT '';

ALTER TABLE app.users
  ADD COLUMN security_version bigint NOT NULL DEFAULT 1
  CHECK (security_version > 0);

-- +goose Down

ALTER TABLE app.users DROP COLUMN security_version;
ALTER TABLE app.season_weeks DROP COLUMN resource_url;
