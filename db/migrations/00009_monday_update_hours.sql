-- +goose Up

-- Hours of work a student reports with each Monday update, in half-hour steps.
-- Updates submitted before the field existed are backfilled with 0; every new
-- update must supply its own value, so the default is dropped after the backfill.
ALTER TABLE app.monday_updates
  ADD COLUMN hours numeric(4,1) NOT NULL DEFAULT 0
  CONSTRAINT monday_updates_hours_range
    CHECK (hours >= 0 AND hours <= 168 AND hours * 2 = trunc(hours * 2));
ALTER TABLE app.monday_updates ALTER COLUMN hours DROP DEFAULT;

-- +goose Down
ALTER TABLE app.monday_updates DROP COLUMN hours;
