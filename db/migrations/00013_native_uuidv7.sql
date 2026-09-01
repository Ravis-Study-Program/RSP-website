-- +goose Up

-- The initial schema used text IDs. Convert an existing installation without
-- losing relationships, while fresh installations get UUID columns directly
-- from 00001. Legacy values that are not UUIDs receive a stable UUIDv7 from
-- this migration; source IDs in migration provenance remain text.
-- +goose StatementBegin
DO $function$
DECLARE
  foreign_key record;
  column_spec record;
  legacy_ids boolean;
BEGIN
  SELECT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'app' AND table_name = 'users'
      AND column_name = 'id' AND udt_name = 'text'
  ) INTO legacy_ids;

  IF NOT legacy_ids THEN
    RETURN;
  END IF;

  CREATE TEMP TABLE native_uuid_map (
    table_name text NOT NULL,
    old_id text NOT NULL,
    new_id uuid NOT NULL DEFAULT uuidv7(),
    PRIMARY KEY (table_name, old_id)
  ) ON COMMIT DROP;

  INSERT INTO native_uuid_map (table_name, old_id)
  SELECT 'app.users', id FROM app.users
  UNION ALL SELECT 'app.global_role_assignments', id FROM app.global_role_assignments
  UNION ALL SELECT 'app.account_deletion_requests', id FROM app.account_deletion_requests
  UNION ALL SELECT 'app.seasons', id FROM app.seasons
  UNION ALL SELECT 'app.season_close_events', id FROM app.season_close_events
  UNION ALL SELECT 'app.season_weeks', id FROM app.season_weeks
  UNION ALL SELECT 'app.enrollments', id FROM app.enrollments
  UNION ALL SELECT 'app.mentorships', id FROM app.mentorships
  UNION ALL SELECT 'app.enrollment_removal_events', id FROM app.enrollment_removal_events
  UNION ALL SELECT 'app.problems', id FROM app.problems
  UNION ALL SELECT 'app.leetcode_problems', id FROM app.leetcode_problems
  UNION ALL SELECT 'app.custom_problems', id FROM app.custom_problems
  UNION ALL SELECT 'app.leetcode_problem_categories', id FROM app.leetcode_problem_categories
  UNION ALL SELECT 'app.problem_attempts', id FROM app.problem_attempts
  UNION ALL SELECT 'app.mock_interviews', id FROM app.mock_interviews
  UNION ALL SELECT 'app.mock_interview_rounds', id FROM app.mock_interview_rounds
  UNION ALL SELECT 'app.behavioural_mock_interview_rounds', id FROM app.behavioural_mock_interview_rounds
  UNION ALL SELECT 'app.leetcode_mock_interview_rounds', id FROM app.leetcode_mock_interview_rounds
  UNION ALL SELECT 'app.custom_mock_interview_rounds', id FROM app.custom_mock_interview_rounds
  UNION ALL SELECT 'app.mock_interview_versions', id FROM app.mock_interview_versions
  UNION ALL SELECT 'app.leetcode_sync_runs', id FROM app.leetcode_sync_runs
  UNION ALL SELECT 'app.audit_events', id FROM app.audit_events
  UNION ALL SELECT 'migration.runs', id FROM migration.runs;

  CREATE TEMP TABLE native_uuid_foreign_keys ON COMMIT DROP AS
  SELECT n.nspname AS schema_name, c.relname AS table_name,
         con.conname AS constraint_name, pg_get_constraintdef(con.oid) AS definition
  FROM pg_constraint con
  JOIN pg_class c ON c.oid = con.conrelid
  JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE con.contype = 'f'
    AND n.nspname IN ('app', 'migration');

  FOR foreign_key IN SELECT * FROM native_uuid_foreign_keys LOOP
    EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT %I', foreign_key.schema_name, foreign_key.table_name, foreign_key.constraint_name);
  END LOOP;

  FOR column_spec IN
    SELECT table_schema, table_name
    FROM information_schema.columns
    WHERE table_schema IN ('app', 'migration')
      AND column_name = 'id' AND udt_name = 'text'
  LOOP
    EXECUTE format(
      'UPDATE %I.%I t SET id = m.new_id::text FROM native_uuid_map m WHERE m.table_name = %L AND m.old_id = t.id',
      column_spec.table_schema, column_spec.table_name,
      column_spec.table_schema || '.' || column_spec.table_name
    );
  END LOOP;

  UPDATE app.user_auth_links t SET user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.user_id;
  UPDATE app.global_role_assignments t SET user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.user_id;
  UPDATE app.global_role_assignments t SET granted_by_user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.granted_by_user_id;
  UPDATE app.account_deletion_requests t SET user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.user_id;
  UPDATE app.season_close_events t SET season_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.seasons' AND m.old_id = t.season_id;
  UPDATE app.season_close_events t SET closed_by_user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.closed_by_user_id;
  UPDATE app.season_close_events t SET reopened_by_user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.reopened_by_user_id;
  UPDATE app.season_close_events t SET migration_run_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'migration.runs' AND m.old_id = t.migration_run_id;
  UPDATE app.season_weeks t SET season_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.seasons' AND m.old_id = t.season_id;
  UPDATE app.enrollments t SET user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.user_id;
  UPDATE app.enrollments t SET season_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.seasons' AND m.old_id = t.season_id;
  UPDATE app.enrollments t SET completed_by_close_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.season_close_events' AND m.old_id = t.completed_by_close_id;
  UPDATE app.mentorships t SET season_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.seasons' AND m.old_id = t.season_id;
  UPDATE app.mentorships t SET mentor_enrollment_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.enrollments' AND m.old_id = t.mentor_enrollment_id;
  UPDATE app.mentorships t SET student_enrollment_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.enrollments' AND m.old_id = t.student_enrollment_id;
  UPDATE app.enrollment_removal_events t SET enrollment_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.enrollments' AND m.old_id = t.enrollment_id;
  UPDATE app.enrollment_removal_events t SET season_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.seasons' AND m.old_id = t.season_id;
  UPDATE app.enrollment_removal_events t SET subject_user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.subject_user_id;
  UPDATE app.enrollment_removal_events t SET actor_user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.actor_user_id;
  UPDATE app.leetcode_problems t SET problem_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.problems' AND m.old_id = t.problem_id;
  UPDATE app.custom_problems t SET problem_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.problems' AND m.old_id = t.problem_id;
  UPDATE app.leetcode_problem_category_mappings t SET leetcode_problem_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.leetcode_problems' AND m.old_id = t.leetcode_problem_id;
  UPDATE app.leetcode_problem_category_mappings t SET category_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.leetcode_problem_categories' AND m.old_id = t.category_id;
  UPDATE app.practice_goals t SET user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.user_id;
  UPDATE app.practice_goals t SET enabled_by_user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.enabled_by_user_id;
  UPDATE app.problem_attempts t SET user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.user_id;
  UPDATE app.problem_attempts t SET problem_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.problems' AND m.old_id = t.problem_id;
  UPDATE app.problem_attempts t SET enrollment_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.enrollments' AND m.old_id = t.enrollment_id;
  UPDATE app.problem_attempts t SET season_week_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.season_weeks' AND m.old_id = t.season_week_id;
  UPDATE app.mock_interviews t SET interviewer_user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.interviewer_user_id;
  UPDATE app.mock_interviews t SET interviewee_user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.interviewee_user_id;
  UPDATE app.mock_interviews t SET season_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.seasons' AND m.old_id = t.season_id;
  UPDATE app.mock_interviews t SET season_week_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.season_weeks' AND m.old_id = t.season_week_id;
  UPDATE app.mock_interview_rounds t SET mock_interview_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.mock_interviews' AND m.old_id = t.mock_interview_id;
  UPDATE app.behavioural_mock_interview_rounds t SET mock_interview_round_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.mock_interview_rounds' AND m.old_id = t.mock_interview_round_id;
  UPDATE app.leetcode_mock_interview_rounds t SET mock_interview_round_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.mock_interview_rounds' AND m.old_id = t.mock_interview_round_id;
  UPDATE app.leetcode_mock_interview_rounds t SET leetcode_problem_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.leetcode_problems' AND m.old_id = t.leetcode_problem_id;
  UPDATE app.custom_mock_interview_rounds t SET mock_interview_round_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.mock_interview_rounds' AND m.old_id = t.mock_interview_round_id;
  UPDATE app.mock_interview_versions t SET mock_interview_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.mock_interviews' AND m.old_id = t.mock_interview_id;
  UPDATE app.mock_interview_versions t SET actor_user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.actor_user_id;
  UPDATE app.leetcode_sync_runs t SET requested_by_user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.requested_by_user_id;
  UPDATE app.audit_events t SET actor_user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.actor_user_id;
  UPDATE migration.source_tables t SET run_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'migration.runs' AND m.old_id = t.run_id;
  UPDATE migration.row_provenance t SET run_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'migration.runs' AND m.old_id = t.run_id;
  UPDATE migration.anomalies t SET run_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'migration.runs' AND m.old_id = t.run_id;
  UPDATE migration.resolutions t SET run_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'migration.runs' AND m.old_id = t.run_id;
  UPDATE migration.auto_fixes t SET run_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'migration.runs' AND m.old_id = t.run_id;
  UPDATE migration.auth0_identity_imports t SET run_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'migration.runs' AND m.old_id = t.run_id;
  UPDATE migration.auth0_identity_imports t SET app_user_id = m.new_id::text
    FROM native_uuid_map m WHERE m.table_name = 'app.users' AND m.old_id = t.app_user_id;

  UPDATE migration.row_provenance p
  SET target_id = m.new_id::text
  FROM native_uuid_map m
  WHERE m.table_name = 'app.' || p.target_table AND m.old_id = p.target_id;
  UPDATE migration.row_provenance p
  SET target_id = mp.new_id::text || '|' || mc.new_id::text
  FROM native_uuid_map mp, native_uuid_map mc
  WHERE p.target_table = 'leetcode_problem_category_mappings'
    AND mp.table_name = 'app.leetcode_problems' AND mp.old_id = split_part(p.target_id, '|', 1)
    AND mc.table_name = 'app.leetcode_problem_categories' AND mc.old_id = split_part(p.target_id, '|', 2);

  FOR column_spec IN
    SELECT * FROM (VALUES
      ('app','users','id'), ('app','user_auth_links','user_id'),
      ('app','global_role_assignments','id'), ('app','global_role_assignments','user_id'), ('app','global_role_assignments','granted_by_user_id'),
      ('app','account_deletion_requests','id'), ('app','account_deletion_requests','user_id'),
      ('app','seasons','id'), ('app','season_close_events','id'), ('app','season_close_events','season_id'), ('app','season_close_events','closed_by_user_id'), ('app','season_close_events','reopened_by_user_id'), ('app','season_close_events','migration_run_id'),
      ('app','season_weeks','id'), ('app','season_weeks','season_id'),
      ('app','enrollments','id'), ('app','enrollments','user_id'), ('app','enrollments','season_id'), ('app','enrollments','completed_by_close_id'),
      ('app','mentorships','id'), ('app','mentorships','season_id'), ('app','mentorships','mentor_enrollment_id'), ('app','mentorships','student_enrollment_id'),
      ('app','enrollment_removal_events','id'), ('app','enrollment_removal_events','enrollment_id'), ('app','enrollment_removal_events','season_id'), ('app','enrollment_removal_events','subject_user_id'), ('app','enrollment_removal_events','actor_user_id'),
      ('app','problems','id'), ('app','leetcode_problems','id'), ('app','leetcode_problems','problem_id'), ('app','custom_problems','id'), ('app','custom_problems','problem_id'), ('app','leetcode_problem_categories','id'), ('app','leetcode_problem_category_mappings','leetcode_problem_id'), ('app','leetcode_problem_category_mappings','category_id'),
      ('app','practice_goals','user_id'), ('app','practice_goals','enabled_by_user_id'), ('app','problem_attempts','id'), ('app','problem_attempts','user_id'), ('app','problem_attempts','problem_id'), ('app','problem_attempts','enrollment_id'), ('app','problem_attempts','season_week_id'),
      ('app','mock_interviews','id'), ('app','mock_interviews','interviewer_user_id'), ('app','mock_interviews','interviewee_user_id'), ('app','mock_interviews','season_id'), ('app','mock_interviews','season_week_id'), ('app','mock_interview_rounds','id'), ('app','mock_interview_rounds','mock_interview_id'), ('app','behavioural_mock_interview_rounds','id'), ('app','behavioural_mock_interview_rounds','mock_interview_round_id'), ('app','leetcode_mock_interview_rounds','id'), ('app','leetcode_mock_interview_rounds','mock_interview_round_id'), ('app','leetcode_mock_interview_rounds','leetcode_problem_id'), ('app','custom_mock_interview_rounds','id'), ('app','custom_mock_interview_rounds','mock_interview_round_id'), ('app','mock_interview_versions','id'), ('app','mock_interview_versions','mock_interview_id'), ('app','mock_interview_versions','actor_user_id'), ('app','leetcode_sync_runs','id'), ('app','leetcode_sync_runs','requested_by_user_id'), ('app','audit_events','id'), ('app','audit_events','actor_user_id'),
      ('migration','runs','id'), ('migration','source_tables','run_id'), ('migration','row_provenance','run_id'), ('migration','anomalies','run_id'), ('migration','resolutions','run_id'), ('migration','auto_fixes','run_id'), ('migration','auth0_identity_imports','run_id'), ('migration','auth0_identity_imports','app_user_id')
    ) AS columns(schema_name, table_name, column_name)
  LOOP
    EXECUTE format('ALTER TABLE %I.%I ALTER COLUMN %I TYPE uuid USING %I::uuid', column_spec.schema_name, column_spec.table_name, column_spec.column_name, column_spec.column_name);
  END LOOP;

  FOR column_spec IN
    SELECT table_schema, table_name
    FROM information_schema.columns
    WHERE table_schema = 'app' AND column_name = 'id'
  LOOP
    EXECUTE format('ALTER TABLE %I.%I ALTER COLUMN id SET DEFAULT uuidv7()', column_spec.table_schema, column_spec.table_name);
  END LOOP;
  EXECUTE 'ALTER TABLE migration.runs ALTER COLUMN id SET DEFAULT uuidv7()';

  FOR foreign_key IN SELECT * FROM native_uuid_foreign_keys LOOP
    EXECUTE format('ALTER TABLE %I.%I ADD CONSTRAINT %I %s', foreign_key.schema_name, foreign_key.table_name, foreign_key.constraint_name, foreign_key.definition);
  END LOOP;
END
$function$;
-- +goose StatementEnd

-- Recreate trigger functions whose local variables previously used text. A
-- function body is not rewritten when PostgreSQL changes a column type.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app.validate_enrollment_close_event()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE
  close_season_id uuid;
BEGIN
  IF NEW.completed_by_close_id IS NULL THEN
    RETURN NEW;
  END IF;
  SELECT season_id INTO STRICT close_season_id
  FROM app.season_close_events
  WHERE id = NEW.completed_by_close_id;
  IF close_season_id <> NEW.season_id THEN
    RAISE EXCEPTION 'enrollment close event belongs to another season'
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app.validate_problem_attempt_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE
  enrollment_user_id uuid;
  enrollment_season_id uuid;
  week_season_id uuid;
BEGIN
  IF NEW.enrollment_id IS NOT NULL THEN
    SELECT user_id, season_id INTO STRICT enrollment_user_id, enrollment_season_id
    FROM app.enrollments WHERE id = NEW.enrollment_id;
    IF enrollment_user_id <> NEW.user_id THEN
      RAISE EXCEPTION 'attempt enrollment belongs to another user'
        USING ERRCODE = '23514';
    END IF;
  END IF;
  IF NEW.season_week_id IS NOT NULL THEN
    SELECT season_id INTO STRICT week_season_id
    FROM app.season_weeks WHERE id = NEW.season_week_id;
  END IF;
  IF enrollment_season_id IS NOT NULL AND week_season_id IS NOT NULL
     AND enrollment_season_id <> week_season_id THEN
    RAISE EXCEPTION 'attempt enrollment and week belong to different seasons'
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd

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

-- +goose Down

-- UUID conversion is intentionally irreversible. Restore from a backup to
-- return to the old text-ID schema.
