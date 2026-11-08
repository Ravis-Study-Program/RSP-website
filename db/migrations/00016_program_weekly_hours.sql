-- +goose Up

-- The hours a week the program expects of each student, shown beside the
-- practice goals and on Monday updates. Set by coordinators and
-- administrators with the practice goals.
ALTER TABLE app.program_practice_goals
  ADD COLUMN weekly_hours integer NOT NULL DEFAULT 30
  CHECK (weekly_hours BETWEEN 1 AND 168);

-- +goose Down
ALTER TABLE app.program_practice_goals DROP COLUMN weekly_hours;
