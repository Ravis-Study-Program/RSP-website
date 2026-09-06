-- +goose Up
-- Keep membership history independently of current role/state. Completing or
-- reopening a season does not change participation dates; season boundaries
-- remain editable. Kicks, withdrawals and role changes close a period.
CREATE TABLE app.enrollment_participation (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  enrollment_id uuid NOT NULL REFERENCES app.enrollments(id) ON DELETE RESTRICT,
  role app.season_role NOT NULL,
  started_at timestamptz NOT NULL,
  ended_at timestamptz,
  CHECK (ended_at IS NULL OR ended_at >= started_at)
);
CREATE UNIQUE INDEX enrollment_participation_open
  ON app.enrollment_participation(enrollment_id) WHERE ended_at IS NULL;
CREATE INDEX enrollment_participation_lookup ON app.enrollment_participation(enrollment_id, started_at);

-- Existing creation/removal timestamps are the available membership evidence.
-- Linked legacy activity can establish an earlier start than its import date.
WITH history AS (
  SELECT e.*,LEAST(e.created_at,
    (SELECT min(a.attempted_at) FROM app.problem_attempts a WHERE a.enrollment_id=e.id),
    (SELECT min(m.scheduled_at) FROM app.mock_interviews m WHERE m.season_id=e.season_id AND m.interviewee_user_id=e.user_id)) AS participation_start
  FROM app.enrollments e
)
INSERT INTO app.enrollment_participation(enrollment_id,role,started_at,ended_at)
SELECT e.id,e.role,e.participation_start,
  CASE WHEN e.state IN ('kicked','withdrawn') THEN GREATEST(e.participation_start,COALESCE(
    (SELECT min(r.occurred_at) FROM app.enrollment_removal_events r WHERE r.enrollment_id=e.id),e.state_changed_at,e.updated_at)) ELSE NULL END
FROM history e;

-- +goose StatementBegin
CREATE FUNCTION app.track_enrollment_participation() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE changed_at timestamptz;
BEGIN
  IF TG_OP = 'INSERT' THEN
    INSERT INTO app.enrollment_participation(enrollment_id,role,started_at,ended_at)
    VALUES(NEW.id,NEW.role,NEW.created_at,
      CASE WHEN NEW.state IN ('kicked','withdrawn') THEN GREATEST(NEW.created_at,COALESCE(NEW.state_changed_at,NEW.updated_at)) ELSE NULL END);
    RETURN NEW;
  END IF;
  IF OLD.role IS DISTINCT FROM NEW.role OR
     (OLD.state IN ('active','completed')) IS DISTINCT FROM (NEW.state IN ('active','completed')) THEN
    changed_at := CASE WHEN NEW.state_changed_at IS DISTINCT FROM OLD.state_changed_at
      THEN NEW.state_changed_at ELSE now() END;
    UPDATE app.enrollment_participation SET ended_at=GREATEST(started_at,changed_at)
      WHERE enrollment_id=NEW.id AND ended_at IS NULL;
    IF NEW.state IN ('active','completed') THEN
      INSERT INTO app.enrollment_participation(enrollment_id,role,started_at)
        VALUES(NEW.id,NEW.role,changed_at);
    END IF;
  END IF;
  RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER enrollments_track_participation AFTER INSERT OR UPDATE OF role,state
  ON app.enrollments FOR EACH ROW EXECUTE FUNCTION app.track_enrollment_participation();

-- A record has at most one derived season. If season dates overlap, the most
-- recently starting eligible season wins; the record is never counted twice.
-- +goose StatementBegin
CREATE FUNCTION app.activity_scope(subject_id uuid, activity_at timestamptz)
RETURNS TABLE(season_id uuid,week_id uuid) LANGUAGE sql STABLE AS $$
  SELECT s.id,w.id FROM app.enrollments e
  JOIN app.enrollment_participation p ON p.enrollment_id=e.id AND p.role='student'
  JOIN app.seasons s ON s.id=e.season_id AND s.deleted_at IS NULL
  LEFT JOIN LATERAL (
    SELECT sw.id FROM app.season_weeks sw WHERE sw.season_id=s.id AND sw.deleted_at IS NULL
      AND activity_at >= sw.start_at AND activity_at <= sw.end_at
    ORDER BY sw.start_at DESC,sw.id LIMIT 1
  ) w ON true
  WHERE e.user_id=subject_id
    AND activity_at >= s.start_at AND activity_at <= s.end_at
    AND activity_at >= p.started_at AND (p.ended_at IS NULL OR activity_at < p.ended_at)
  ORDER BY s.start_at DESC,p.started_at DESC,s.id LIMIT 1
$$;
-- +goose StatementEnd
CREATE VIEW app.scoped_problem_attempts AS
  SELECT a.*,scope.season_id AS activity_season_id,scope.week_id AS activity_week_id
  FROM app.problem_attempts a LEFT JOIN LATERAL app.activity_scope(a.user_id,a.attempted_at) scope ON true;
CREATE VIEW app.scoped_mock_interviews AS
  SELECT m.*,scope.season_id AS activity_season_id,scope.week_id AS activity_week_id
  FROM app.mock_interviews m LEFT JOIN LATERAL app.activity_scope(m.interviewee_user_id,m.scheduled_at) scope ON true;
GRANT SELECT ON app.scoped_problem_attempts,app.scoped_mock_interviews TO rsp_app;
GRANT SELECT,INSERT,UPDATE ON app.enrollment_participation TO rsp_app;

-- Stored season links remain as legacy evidence only. New records do not use
-- them, and changing a season's dates must not be blocked by old mock links.
DROP TRIGGER mock_interviews_validate_scope ON app.mock_interviews;
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app.validate_season_child_dates() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.season_weeks WHERE season_id=NEW.id AND deleted_at IS NULL
      AND (start_at<NEW.start_at OR end_at>NEW.end_at)) THEN
    RAISE EXCEPTION 'season dates cannot exclude existing weeks' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP VIEW app.scoped_mock_interviews;
DROP VIEW app.scoped_problem_attempts;
DROP FUNCTION app.activity_scope(uuid,timestamptz);
DROP TRIGGER enrollments_track_participation ON app.enrollments;
DROP FUNCTION app.track_enrollment_participation();
DROP TABLE app.enrollment_participation;
CREATE TRIGGER mock_interviews_validate_scope
  BEFORE INSERT OR UPDATE OF season_id,season_week_id,scheduled_at,interviewer_user_id,interviewee_user_id
  ON app.mock_interviews FOR EACH ROW EXECUTE FUNCTION app.validate_mock_interview_scope();
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app.validate_season_child_dates() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.season_weeks WHERE season_id=NEW.id AND deleted_at IS NULL
      AND (start_at<NEW.start_at OR end_at>NEW.end_at)) OR EXISTS (
    SELECT 1 FROM app.mock_interviews WHERE season_id=NEW.id AND deleted_at IS NULL
      AND (scheduled_at<NEW.start_at OR scheduled_at>NEW.end_at)) THEN
    RAISE EXCEPTION 'season dates cannot exclude existing weeks or mock interviews' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END
$$;
-- +goose StatementEnd
