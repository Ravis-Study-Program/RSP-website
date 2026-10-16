-- +goose Up

-- Monday updates are written content a student submits for a calendar-date range
-- they choose; the LeetCode attempts and mock interviews shown with an update are
-- always derived from that range at read time, never copied into the record.
CREATE TYPE app.monday_update_mark AS ENUM (
  'way_ahead',
  'doing_good',
  'on_track',
  'a_bit_behind',
  'at_risk'
);

-- The season is an explicit input here, not a derived read-time fact: the student
-- writes the update inside a season page and every mentor/reviewer list is
-- season-scoped.
CREATE TABLE app.monday_updates (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  season_id uuid NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  student_user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  start_date date NOT NULL,
  end_date date NOT NULL,
  content_html text NOT NULL,
  content_sanitization_changed boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT monday_updates_date_order CHECK (end_date >= start_date)
);
CREATE INDEX monday_updates_season_student
  ON app.monday_updates (season_id, student_user_id) WHERE deleted_at IS NULL;
CREATE INDEX monday_updates_season_dates
  ON app.monday_updates (season_id, start_date, end_date) WHERE deleted_at IS NULL;

-- One mark per (update, mentor); a mentor revises their own mark in place.
CREATE TABLE app.monday_update_marks (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  update_id uuid NOT NULL REFERENCES app.monday_updates(id) ON DELETE CASCADE,
  mentor_user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  mark app.monday_update_mark NOT NULL,
  comment_html text,
  comment_sanitization_changed boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (update_id, mentor_user_id)
);
CREATE INDEX monday_update_marks_update ON app.monday_update_marks(update_id);

GRANT SELECT,INSERT,UPDATE ON app.monday_updates TO rsp_app;
GRANT SELECT,INSERT,UPDATE ON app.monday_update_marks TO rsp_app;

-- +goose Down
DROP TABLE app.monday_update_marks;
DROP TABLE app.monday_updates;
DROP TYPE app.monday_update_mark;
