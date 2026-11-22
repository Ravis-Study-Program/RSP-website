-- +goose Up

-- What the program expects of each student every week besides their hours:
-- LeetCode problems solved by difficulty and mock interviews. Set by
-- coordinators and administrators with the weekly hours. A target of 0
-- means none.
ALTER TABLE app.program_practice_goals
  ADD COLUMN weekly_easy integer NOT NULL DEFAULT 3 CHECK (weekly_easy BETWEEN 0 AND 50),
  ADD COLUMN weekly_medium integer NOT NULL DEFAULT 5 CHECK (weekly_medium BETWEEN 0 AND 50),
  ADD COLUMN weekly_hard integer NOT NULL DEFAULT 2 CHECK (weekly_hard BETWEEN 0 AND 50),
  ADD COLUMN weekly_mocks integer NOT NULL DEFAULT 4 CHECK (weekly_mocks BETWEEN 0 AND 50);

-- A student's own weekly targets, set by their mentor or a coordinator. A
-- blank value follows the program's.
ALTER TABLE app.student_requirements
  ADD COLUMN weekly_easy integer CHECK (weekly_easy BETWEEN 0 AND 50),
  ADD COLUMN weekly_medium integer CHECK (weekly_medium BETWEEN 0 AND 50),
  ADD COLUMN weekly_hard integer CHECK (weekly_hard BETWEEN 0 AND 50),
  ADD COLUMN weekly_mocks integer CHECK (weekly_mocks BETWEEN 0 AND 50);

-- +goose Down
ALTER TABLE app.student_requirements
  DROP COLUMN weekly_easy, DROP COLUMN weekly_medium, DROP COLUMN weekly_hard, DROP COLUMN weekly_mocks;
ALTER TABLE app.program_practice_goals
  DROP COLUMN weekly_easy, DROP COLUMN weekly_medium, DROP COLUMN weekly_hard, DROP COLUMN weekly_mocks;
