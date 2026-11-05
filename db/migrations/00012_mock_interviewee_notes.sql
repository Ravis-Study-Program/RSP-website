-- +goose Up

-- The interviewee's notes on a mock interview as a whole, beside the
-- interviewer's overall notes. Per-round notes stay on each round.
ALTER TABLE app.mock_interviews ADD COLUMN interviewee_notes_html text;

-- The scoped view selects m.*, which Postgres expanded when the view was
-- created, so it must be rebuilt to include the new column.
DROP VIEW app.scoped_mock_interviews;
CREATE VIEW app.scoped_mock_interviews AS
  SELECT m.*,scope.season_id AS activity_season_id,scope.week_id AS activity_week_id
  FROM app.mock_interviews m LEFT JOIN LATERAL app.activity_scope(m.interviewee_user_id,m.scheduled_at) scope ON true;
GRANT SELECT ON app.scoped_mock_interviews TO rsp_app;

-- +goose Down
DROP VIEW app.scoped_mock_interviews;
ALTER TABLE app.mock_interviews DROP COLUMN interviewee_notes_html;
CREATE VIEW app.scoped_mock_interviews AS
  SELECT m.*,scope.season_id AS activity_season_id,scope.week_id AS activity_week_id
  FROM app.mock_interviews m LEFT JOIN LATERAL app.activity_scope(m.interviewee_user_id,m.scheduled_at) scope ON true;
GRANT SELECT ON app.scoped_mock_interviews TO rsp_app;
