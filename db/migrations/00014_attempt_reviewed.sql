-- +goose Up

-- Whether the attempt's owner has reviewed it, as interviewees mark mock
-- interview rounds reviewed.
ALTER TABLE app.problem_attempts ADD COLUMN reviewed boolean NOT NULL DEFAULT false;

-- The scoped view selects a.*, which Postgres expanded when the view was
-- created, so it must be rebuilt to include the new column.
DROP VIEW app.scoped_problem_attempts;
CREATE VIEW app.scoped_problem_attempts AS
  SELECT a.*,scope.season_id AS activity_season_id,scope.week_id AS activity_week_id
  FROM app.problem_attempts a LEFT JOIN LATERAL app.activity_scope(a.user_id,a.attempted_at) scope ON true;
GRANT SELECT ON app.scoped_problem_attempts TO rsp_app;

-- +goose Down
DROP VIEW app.scoped_problem_attempts;
ALTER TABLE app.problem_attempts DROP COLUMN reviewed;
CREATE VIEW app.scoped_problem_attempts AS
  SELECT a.*,scope.season_id AS activity_season_id,scope.week_id AS activity_week_id
  FROM app.problem_attempts a LEFT JOIN LATERAL app.activity_scope(a.user_id,a.attempted_at) scope ON true;
GRANT SELECT ON app.scoped_problem_attempts TO rsp_app;
