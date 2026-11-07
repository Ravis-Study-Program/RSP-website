-- +goose Up

-- A student's own practice time goals and weekly hours, set by their mentor
-- or a coordinator. A blank value follows the program's default
-- (app.program_practice_goals).
CREATE TABLE app.student_requirements (
  enrollment_id uuid PRIMARY KEY REFERENCES app.enrollments(id) ON DELETE CASCADE,
  easy_minutes integer CHECK (easy_minutes BETWEEN 1 AND 180),
  medium_minutes integer CHECK (medium_minutes BETWEEN 1 AND 180),
  hard_minutes integer CHECK (hard_minutes BETWEEN 1 AND 180),
  weekly_hours integer CHECK (weekly_hours BETWEEN 1 AND 168),
  updated_by_user_id uuid REFERENCES app.users(id) ON DELETE RESTRICT,
  updated_at timestamptz NOT NULL DEFAULT now()
);
GRANT SELECT, INSERT, UPDATE ON app.student_requirements TO rsp_app;

-- +goose Down
DROP TABLE app.student_requirements;
