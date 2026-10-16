-- +goose Up
-- +goose StatementBegin
-- mentorships_one_active_mentor already treats "ended_at IS NULL AND
-- deleted_at IS NULL" as the liveness test for a mentorship. validate_mentorship
-- did not, so it policed rows that are no longer live: a mentorship whose
-- student was kicked ended at that moment, but the trigger still demanded an
-- active or completed enrollment and rejected the import of 79 historical
-- mentorships. Validate only live rows, matching the index.
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
  IF NEW.ended_at IS NULL AND NEW.deleted_at IS NULL THEN
    IF mentor_row.state NOT IN ('active', 'completed')
       OR student_row.state NOT IN ('active', 'completed')
       OR mentor_row.deleted_at IS NOT NULL OR student_row.deleted_at IS NOT NULL THEN
      RAISE EXCEPTION 'mentorship requires active or completed, non-deleted enrollments'
        USING ERRCODE = 'check_violation';
    END IF;
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