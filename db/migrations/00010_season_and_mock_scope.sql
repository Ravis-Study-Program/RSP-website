-- +goose Up

ALTER TABLE app.seasons
  ADD CONSTRAINT seasons_slug_format CHECK (
    length(slug) BETWEEN 1 AND 50
    AND slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'
  );

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app.validate_mock_interview_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE
  week_season_id uuid;
  season_start timestamptz;
  season_end timestamptz;
BEGIN
  IF NEW.season_id IS NULL THEN
    IF NEW.season_week_id IS NOT NULL THEN
      RAISE EXCEPTION 'mock interview week requires a selected season'
        USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
  END IF;

  SELECT start_at, end_at INTO STRICT season_start, season_end
  FROM app.seasons WHERE id = NEW.season_id;
  IF NEW.scheduled_at < season_start OR NEW.scheduled_at > season_end THEN
    RAISE EXCEPTION 'mock interview date must fall within season dates'
      USING ERRCODE = '23514';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM app.enrollments
    WHERE user_id = NEW.interviewer_user_id AND season_id = NEW.season_id
      AND state IN ('active','completed') AND deleted_at IS NULL
  ) OR NOT EXISTS (
    SELECT 1 FROM app.enrollments
    WHERE user_id = NEW.interviewee_user_id AND season_id = NEW.season_id
      AND state IN ('active','completed') AND deleted_at IS NULL
  ) THEN
    RAISE EXCEPTION 'mock interview participants must belong to the selected season'
      USING ERRCODE = '23514';
  END IF;

  IF NEW.season_week_id IS NOT NULL THEN
    SELECT season_id INTO STRICT week_season_id
    FROM app.season_weeks WHERE id = NEW.season_week_id;
    IF NEW.season_id <> week_season_id THEN
      RAISE EXCEPTION 'mock interview week must belong to the selected season'
        USING ERRCODE = '23514';
    END IF;
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd

DROP TRIGGER mock_interviews_validate_scope ON app.mock_interviews;
CREATE TRIGGER mock_interviews_validate_scope
  BEFORE INSERT OR UPDATE OF season_id, season_week_id, scheduled_at, interviewer_user_id, interviewee_user_id
  ON app.mock_interviews
  FOR EACH ROW EXECUTE FUNCTION app.validate_mock_interview_scope();

-- +goose StatementBegin
CREATE FUNCTION app.validate_season_child_dates()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
  IF EXISTS (
    SELECT 1 FROM app.season_weeks
    WHERE season_id = NEW.id AND deleted_at IS NULL
      AND (start_at < NEW.start_at OR end_at > NEW.end_at)
  ) OR EXISTS (
    SELECT 1 FROM app.mock_interviews
    WHERE season_id = NEW.id AND deleted_at IS NULL
      AND (scheduled_at < NEW.start_at OR scheduled_at > NEW.end_at)
  ) THEN
    RAISE EXCEPTION 'season dates cannot exclude existing weeks or mock interviews'
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd

CREATE TRIGGER seasons_validate_child_dates
  BEFORE UPDATE OF start_at, end_at ON app.seasons
  FOR EACH ROW EXECUTE FUNCTION app.validate_season_child_dates();

-- +goose Down

DROP TRIGGER seasons_validate_child_dates ON app.seasons;
DROP FUNCTION app.validate_season_child_dates();

DROP TRIGGER mock_interviews_validate_scope ON app.mock_interviews;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app.validate_mock_interview_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE
  week_season_id uuid;
BEGIN
  IF NEW.season_week_id IS NULL THEN
    RETURN NEW;
  END IF;
  SELECT season_id INTO STRICT week_season_id
  FROM app.season_weeks WHERE id = NEW.season_week_id;
  IF NEW.season_id IS NULL OR NEW.season_id <> week_season_id THEN
    RAISE EXCEPTION 'mock interview week must belong to the selected season'
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd

CREATE TRIGGER mock_interviews_validate_scope
  BEFORE INSERT OR UPDATE OF season_id, season_week_id
  ON app.mock_interviews
  FOR EACH ROW EXECUTE FUNCTION app.validate_mock_interview_scope();

ALTER TABLE app.seasons DROP CONSTRAINT seasons_slug_format;
