-- +goose Up

-- The program's practice time goals, which apply to every member and are
-- set by coordinators and administrators. A single row: `singleton` is
-- always true. Personal goals in app.practice_goals stay historical data.
CREATE TABLE app.program_practice_goals (
  singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  easy_minutes integer NOT NULL CHECK (easy_minutes BETWEEN 1 AND 180),
  medium_minutes integer NOT NULL CHECK (medium_minutes BETWEEN 1 AND 180),
  hard_minutes integer NOT NULL CHECK (hard_minutes BETWEEN 1 AND 180),
  updated_by_user_id uuid REFERENCES app.users(id) ON DELETE RESTRICT,
  updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO app.program_practice_goals(easy_minutes, medium_minutes, hard_minutes)
  VALUES (10, 25, 40);
GRANT SELECT, INSERT, UPDATE ON app.program_practice_goals TO rsp_app;

-- +goose Down
DROP TABLE app.program_practice_goals;
