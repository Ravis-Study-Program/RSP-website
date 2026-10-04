-- +goose Up
-- +goose StatementBegin
-- validate_mock_interview admits enrollments in state 'active' or 'completed',
-- because a season that has closed still has valid historical records against
-- it. validate_mentorship admitted only 'active', so every mentorship in a
-- closed season was rejected. Align the two.
CREATE OR REPLACE FUNCTION app.validate_mentorship()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE
  mentor_row app.enrollments%ROWTYPE;
  student_row app.enrollments%ROWTYPE;
BEGIN
  SELECT * INTO STRICT mentor_row FROM app.enrollments WHERE id = NEW.mentor_enrollment_id;
  SELECT * INTO STRICT student_row FROM app.enrollments WHERE id = NEW.student_enrollment_id;
  IF mentor_row.season_id <> NEW.season_id OR student_row.season_id <> NEW.season_id THEN
    RAISE EXCEPTION 'mentorship enrollments must belong to the selected season'
      USING ERRCODE = 'check_violation';
  END IF;
  IF mentor_row.role NOT IN ('mentor', 'coordinator') OR student_row.role <> 'student' THEN
    RAISE EXCEPTION 'mentorship requires a mentor/coordinator and a student'
      USING ERRCODE = 'check_violation';
  END IF;
  IF mentor_row.state NOT IN ('active', 'completed')
     OR student_row.state NOT IN ('active', 'completed')
     OR mentor_row.deleted_at IS NOT NULL OR student_row.deleted_at IS NOT NULL THEN
    RAISE EXCEPTION 'mentorship requires active or completed, non-deleted enrollments'
      USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app.validate_mentorship()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE
  mentor_row app.enrollments%ROWTYPE;
  student_row app.enrollments%ROWTYPE;
BEGIN
  SELECT * INTO STRICT mentor_row FROM app.enrollments WHERE id = NEW.mentor_enrollment_id;
  SELECT * INTO STRICT student_row FROM app.enrollments WHERE id = NEW.student_enrollment_id;
  IF mentor_row.season_id <> NEW.season_id OR student_row.season_id <> NEW.season_id THEN
    RAISE EXCEPTION 'mentorship enrollments must belong to the selected season'
      USING ERRCODE = 'check_violation';
  END IF;
  IF mentor_row.role NOT IN ('mentor', 'coordinator') OR student_row.role <> 'student' THEN
    RAISE EXCEPTION 'mentorship requires a mentor/coordinator and a student'
      USING ERRCODE = 'check_violation';
  END IF;
  IF mentor_row.state <> 'active' OR student_row.state <> 'active'
     OR mentor_row.deleted_at IS NOT NULL OR student_row.deleted_at IS NOT NULL THEN
    RAISE EXCEPTION 'mentorship requires active, non-deleted enrollments'
      USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd