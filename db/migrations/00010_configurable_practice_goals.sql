-- +goose Up

-- Programme time goals apply to every member unless a student has an override.
-- The table holds at most one row, which Directors and System Admins edit;
-- until it is first saved the backend's built-in defaults apply.
CREATE TABLE app.programme_practice_goals (
  singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  easy_minutes integer NOT NULL CHECK (easy_minutes BETWEEN 1 AND 180),
  medium_minutes integer NOT NULL CHECK (medium_minutes BETWEEN 1 AND 180),
  hard_minutes integer NOT NULL CHECK (hard_minutes BETWEEN 1 AND 180),
  updated_by_user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- A mentor or season administrator can replace the programme goals for one
-- student. The override follows the student across seasons until it is cleared.
-- Legacy personal goals in app.practice_goals remain historical data only.
CREATE TABLE app.student_practice_goals (
  user_id uuid PRIMARY KEY REFERENCES app.users(id) ON DELETE RESTRICT,
  easy_minutes integer NOT NULL CHECK (easy_minutes BETWEEN 1 AND 180),
  medium_minutes integer NOT NULL CHECK (medium_minutes BETWEEN 1 AND 180),
  hard_minutes integer NOT NULL CHECK (hard_minutes BETWEEN 1 AND 180),
  set_by_user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER programme_practice_goals_touch_updated_at
  BEFORE UPDATE ON app.programme_practice_goals FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER student_practice_goals_touch_updated_at
  BEFORE UPDATE ON app.student_practice_goals FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();

GRANT SELECT,INSERT,UPDATE ON app.programme_practice_goals TO rsp_app;
GRANT SELECT,INSERT,UPDATE,DELETE ON app.student_practice_goals TO rsp_app;

-- +goose Down
DROP TABLE app.student_practice_goals;
DROP TABLE app.programme_practice_goals;
