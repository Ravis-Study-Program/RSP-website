-- +goose Up

ALTER TABLE app.practice_goals
  ADD CONSTRAINT practice_goals_easy_range CHECK (easy_minutes BETWEEN 5 AND 180),
  ADD CONSTRAINT practice_goals_medium_range CHECK (medium_minutes BETWEEN 5 AND 180),
  ADD CONSTRAINT practice_goals_hard_range CHECK (hard_minutes BETWEEN 5 AND 180);

-- +goose Down

ALTER TABLE app.practice_goals
  DROP CONSTRAINT practice_goals_hard_range,
  DROP CONSTRAINT practice_goals_medium_range,
  DROP CONSTRAINT practice_goals_easy_range;
