-- +goose Up
-- Season and week dates are whole days in Australia/Adelaide. The web app
-- used to save them as midnight in the editor's own timezone, so a week
-- entered as 30/11 from Sydney starts at 23:30 on 29/11 in Adelaide, and its
-- Monday update covers the week before. Move every bound that sits within
-- three hours of an Adelaide day boundary onto it: starts to 00:00:00 and ends
-- to 23:59:59. Three hours covers every Australian and New Zealand editor;
-- bounds further away were set on purpose and stay as they are.

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION pg_temp.adelaide_day_start(value timestamptz) RETURNS timestamptz
LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE
    WHEN abs(extract(epoch FROM (value AT TIME ZONE 'Australia/Adelaide')
        - date_trunc('day', (value AT TIME ZONE 'Australia/Adelaide') + interval '12 hours'))) <= 3 * 3600
      THEN date_trunc('day', (value AT TIME ZONE 'Australia/Adelaide') + interval '12 hours')
        AT TIME ZONE 'Australia/Adelaide'
    ELSE value
  END
$$;
-- +goose StatementEnd

-- An end is the second before a day boundary.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION pg_temp.adelaide_day_end(value timestamptz) RETURNS timestamptz
LANGUAGE sql IMMUTABLE AS $$
  SELECT pg_temp.adelaide_day_start(value + interval '1 second') - interval '1 second'
$$;
-- +goose StatementEnd

-- Weeks must stay inside their season, and a season cannot shrink past its
-- weeks, so: widen each season to cover both its old and new bounds, move the
-- weeks, then move the seasons.
UPDATE app.seasons
SET start_at = least(start_at, pg_temp.adelaide_day_start(start_at)),
    end_at = greatest(end_at, pg_temp.adelaide_day_end(end_at))
WHERE start_at <> pg_temp.adelaide_day_start(start_at)
   OR end_at <> pg_temp.adelaide_day_end(end_at);

UPDATE app.season_weeks
SET start_at = pg_temp.adelaide_day_start(start_at),
    end_at = pg_temp.adelaide_day_end(end_at)
WHERE start_at <> pg_temp.adelaide_day_start(start_at)
   OR end_at <> pg_temp.adelaide_day_end(end_at);

UPDATE app.seasons
SET start_at = pg_temp.adelaide_day_start(start_at),
    end_at = pg_temp.adelaide_day_end(end_at)
WHERE start_at <> pg_temp.adelaide_day_start(start_at)
   OR end_at <> pg_temp.adelaide_day_end(end_at);

-- Temporary functions last as long as the connection, which a migration
-- runner may reuse (for example to migrate down and up again).
DROP FUNCTION pg_temp.adelaide_day_end(timestamptz);
DROP FUNCTION pg_temp.adelaide_day_start(timestamptz);

-- +goose Down
-- The editors' original timezones were not recorded, so there is nothing to
-- restore; the Adelaide days are kept.
SELECT 1;
