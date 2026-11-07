-- +goose Up

-- A mentor marks every mentee's week, posted or not; the coordinator reads
-- each mentor's marks for the week. Posted updates keep their marks in
-- app.monday_update_marks; a week with no update is marked here.
CREATE TABLE app.monday_week_marks (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  season_id uuid NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  student_user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  mentor_user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  start_date date NOT NULL,
  end_date date NOT NULL,
  mark app.monday_update_mark NOT NULL,
  comment_html text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT monday_week_marks_week CHECK (end_date = start_date + 6),
  CONSTRAINT monday_week_marks_one UNIQUE (season_id, student_user_id, start_date, mentor_user_id)
);

GRANT SELECT, INSERT, UPDATE ON app.monday_week_marks TO rsp_app;

-- +goose Down
DROP TABLE app.monday_week_marks;
