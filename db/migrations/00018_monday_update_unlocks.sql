-- +goose Up

-- A Monday update locks once its due date (the Monday after its week) has
-- passed: the student can no longer post, edit or delete it. To change a
-- late week they ask their mentor, with a reason, to unlock it.
CREATE TYPE app.monday_update_unlock_state AS ENUM ('pending', 'approved', 'declined');

CREATE TABLE app.monday_update_unlocks (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  season_id uuid NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  student_user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  start_date date NOT NULL,
  end_date date NOT NULL,
  reason text NOT NULL CHECK (length(btrim(reason)) BETWEEN 1 AND 1000),
  state app.monday_update_unlock_state NOT NULL DEFAULT 'pending',
  requested_at timestamptz NOT NULL DEFAULT now(),
  decided_by_user_id uuid REFERENCES app.users(id) ON DELETE RESTRICT,
  decided_at timestamptz,
  CONSTRAINT monday_update_unlocks_week CHECK (end_date = start_date + 6),
  CONSTRAINT monday_update_unlocks_decided CHECK ((state = 'pending') = (decided_at IS NULL))
);
-- One open request or unlock per week; a declined one can be asked again.
CREATE UNIQUE INDEX monday_update_unlocks_open
  ON app.monday_update_unlocks (season_id, student_user_id, start_date)
  WHERE state IN ('pending', 'approved');
GRANT SELECT, INSERT, UPDATE ON app.monday_update_unlocks TO rsp_app;

-- +goose Down
DROP TABLE app.monday_update_unlocks;
DROP TYPE app.monday_update_unlock_state;
