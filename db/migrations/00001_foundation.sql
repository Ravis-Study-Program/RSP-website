-- +goose Up

CREATE SCHEMA app;
CREATE SCHEMA migration;

CREATE TYPE app.account_state AS ENUM (
  'active',
  'suspended',
  'deletion_pending',
  'deleted'
);
CREATE TYPE app.assignment_state AS ENUM ('pending_mfa', 'active', 'revoked');
CREATE TYPE app.global_role AS ENUM ('director', 'system_admin');
CREATE TYPE app.season_status AS ENUM ('open', 'closed');
CREATE TYPE app.season_role AS ENUM ('student', 'mentor', 'coordinator');
CREATE TYPE app.enrollment_state AS ENUM ('active', 'completed', 'kicked', 'withdrawn');
CREATE TYPE app.student_level AS ENUM (
  'not_applicable',
  'novice',
  'beginner',
  'intermediate',
  'advanced'
);
CREATE TYPE app.problem_difficulty AS ENUM ('easy', 'medium', 'hard');
CREATE TYPE app.attempt_outcome AS ENUM (
  'independently_solved',
  'solved_with_hints',
  'not_solved',
  'unknown'
);
CREATE TYPE app.recommendation_state AS ENUM (
  'active',
  'attempted',
  'dismissed',
  'superseded'
);
CREATE TYPE app.mock_round_kind AS ENUM ('behavioural', 'leetcode', 'custom');
CREATE TYPE app.review_status AS ENUM ('pending', 'reviewed');
CREATE TYPE app.migration_run_state AS ENUM (
  'planned',
  'applying',
  'applied',
  'verified',
  'rolled_back',
  'failed'
);
CREATE TYPE migration.anomaly_severity AS ENUM ('warning', 'blocking');
CREATE TYPE migration.resolution_action AS ENUM (
  'map_id',
  'use_value',
  'skip_source_row',
  'approve_transform'
);

-- +goose StatementBegin
CREATE FUNCTION app.touch_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION app.reject_immutable_change()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
  RAISE EXCEPTION '% is append-only', TG_TABLE_NAME
    USING ERRCODE = '55000';
END
$function$;
-- +goose StatementEnd

CREATE TABLE app.users (
  id text PRIMARY KEY,
  slug text NOT NULL,
  display_name text NOT NULL,
  email text,
  discord_id text,
  avatar_url text,
  account_state app.account_state NOT NULL DEFAULT 'active',
  timezone text NOT NULL DEFAULT 'Australia/Adelaide',
  is_test boolean NOT NULL DEFAULT false,
  leetcode_premium_opt_in boolean NOT NULL DEFAULT false,
  legacy_is_admin boolean,
  legacy_is_graduate boolean,
  suspended_at timestamptz,
  deletion_requested_at timestamptz,
  deletion_due_at timestamptz,
  pseudonymized_at timestamptz,
  deleted_at timestamptz,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT users_slug_nonempty CHECK (length(trim(slug)) > 0),
  CONSTRAINT users_display_name_nonempty CHECK (length(trim(display_name)) > 0),
  CONSTRAINT users_deletion_window CHECK (
    (deletion_requested_at IS NULL AND deletion_due_at IS NULL)
    OR
    (deletion_requested_at IS NOT NULL AND deletion_due_at >= deletion_requested_at)
  ),
  CONSTRAINT users_state_timestamps CHECK (
    (account_state <> 'suspended' OR suspended_at IS NOT NULL)
    AND (account_state <> 'deletion_pending' OR deletion_due_at IS NOT NULL)
    AND (account_state <> 'deleted' OR pseudonymized_at IS NOT NULL)
  )
);
CREATE UNIQUE INDEX users_slug_unique ON app.users (lower(slug));
CREATE UNIQUE INDEX users_email_unique
  ON app.users (lower(email))
  WHERE email IS NOT NULL;

CREATE TABLE app.user_auth_links (
  auth_subject text NOT NULL,
  user_id text NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  provider text NOT NULL,
  provider_account_id text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  linked_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz,
  PRIMARY KEY (auth_subject),
  UNIQUE (provider, provider_account_id),
  CONSTRAINT user_auth_links_provider_nonempty CHECK (length(trim(provider)) > 0),
  CONSTRAINT user_auth_links_revocation CHECK (
    (active AND revoked_at IS NULL) OR (NOT active AND revoked_at IS NOT NULL)
  )
);
CREATE UNIQUE INDEX user_auth_links_one_active_subject
  ON app.user_auth_links (user_id, auth_subject)
  WHERE active;

CREATE TABLE app.global_role_assignments (
  id text PRIMARY KEY,
  user_id text NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  role app.global_role NOT NULL,
  state app.assignment_state NOT NULL DEFAULT 'pending_mfa',
  granted_by_user_id text REFERENCES app.users(id) ON DELETE RESTRICT,
  granted_at timestamptz NOT NULL DEFAULT now(),
  activated_at timestamptz,
  revoked_at timestamptz,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  UNIQUE (user_id, role),
  CONSTRAINT global_role_assignment_state CHECK (
    (state = 'pending_mfa' AND activated_at IS NULL AND revoked_at IS NULL)
    OR (state = 'active' AND activated_at IS NOT NULL AND revoked_at IS NULL)
    OR (state = 'revoked' AND revoked_at IS NOT NULL)
  )
);

CREATE TABLE app.account_deletion_requests (
  id text PRIMARY KEY,
  user_id text NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  recovery_token_hash text NOT NULL UNIQUE,
  requested_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  cancelled_at timestamptz,
  completed_at timestamptz,
  CONSTRAINT account_deletion_request_window CHECK (expires_at > requested_at),
  CONSTRAINT account_deletion_request_terminal CHECK (
    cancelled_at IS NULL OR completed_at IS NULL
  )
);
CREATE UNIQUE INDEX account_deletion_requests_one_open
  ON app.account_deletion_requests (user_id)
  WHERE cancelled_at IS NULL AND completed_at IS NULL;

CREATE TABLE app.seasons (
  id text PRIMARY KEY,
  slug text NOT NULL,
  name text NOT NULL,
  status app.season_status NOT NULL DEFAULT 'open',
  start_at timestamptz NOT NULL,
  end_at timestamptz NOT NULL,
  location text NOT NULL DEFAULT '',
  image_url text NOT NULL DEFAULT '',
  resources_url text NOT NULL DEFAULT '',
  legacy_data_backfilled boolean,
  closed_at timestamptz,
  deleted_at timestamptz,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT seasons_slug_nonempty CHECK (length(trim(slug)) > 0),
  CONSTRAINT seasons_name_nonempty CHECK (length(trim(name)) > 0),
  CONSTRAINT seasons_date_order CHECK (end_at >= start_at),
  CONSTRAINT seasons_status_closed_at CHECK (
    (status = 'open' AND closed_at IS NULL)
    OR (status = 'closed' AND closed_at IS NOT NULL)
  )
);
CREATE UNIQUE INDEX seasons_slug_unique ON app.seasons (lower(slug));

CREATE TABLE app.season_close_events (
  id text PRIMARY KEY,
  season_id text NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  closed_by_user_id text REFERENCES app.users(id) ON DELETE RESTRICT,
  close_reason text NOT NULL,
  closed_at timestamptz NOT NULL,
  reopened_by_user_id text REFERENCES app.users(id) ON DELETE RESTRICT,
  reopen_reason text,
  reopened_at timestamptz,
  migration_run_id text,
  CONSTRAINT season_close_events_reason_nonempty CHECK (length(trim(close_reason)) > 0),
  CONSTRAINT season_close_events_reopen_complete CHECK (
    (reopened_at IS NULL AND reopened_by_user_id IS NULL AND reopen_reason IS NULL)
    OR
    (reopened_at >= closed_at AND reopened_by_user_id IS NOT NULL AND length(trim(reopen_reason)) > 0)
  )
);
CREATE UNIQUE INDEX season_close_events_one_open
  ON app.season_close_events (season_id)
  WHERE reopened_at IS NULL;

CREATE TABLE app.season_weeks (
  id text PRIMARY KEY,
  season_id text NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  week_number integer NOT NULL CHECK (week_number > 0),
  start_at timestamptz NOT NULL,
  end_at timestamptz NOT NULL,
  deleted_at timestamptz,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (season_id, week_number),
  CONSTRAINT season_weeks_date_order CHECK (end_at >= start_at)
);

-- +goose StatementBegin
CREATE FUNCTION app.validate_season_week_bounds()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE
  season_start timestamptz;
  season_end timestamptz;
BEGIN
  SELECT start_at, end_at INTO STRICT season_start, season_end
  FROM app.seasons WHERE id = NEW.season_id;
  IF NEW.start_at < season_start OR NEW.end_at > season_end THEN
    RAISE EXCEPTION 'season week must fall within season dates'
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd
CREATE TRIGGER season_weeks_validate_bounds
  BEFORE INSERT OR UPDATE OF season_id, start_at, end_at
  ON app.season_weeks
  FOR EACH ROW EXECUTE FUNCTION app.validate_season_week_bounds();

CREATE TABLE app.enrollments (
  id text PRIMARY KEY,
  user_id text NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  season_id text NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  role app.season_role NOT NULL,
  student_level app.student_level NOT NULL DEFAULT 'not_applicable',
  state app.enrollment_state NOT NULL DEFAULT 'active',
  completed_by_close_id text REFERENCES app.season_close_events(id) ON DELETE RESTRICT,
  state_changed_at timestamptz,
  deleted_at timestamptz,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, season_id),
  CONSTRAINT enrollments_student_level CHECK (
    (role = 'student' AND student_level <> 'not_applicable')
    OR (role <> 'student' AND student_level = 'not_applicable')
  ),
  CONSTRAINT enrollments_close_completion CHECK (
    completed_by_close_id IS NULL OR state = 'completed'
  )
);

CREATE TABLE app.mentorships (
  id text PRIMARY KEY,
  season_id text NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  mentor_enrollment_id text NOT NULL REFERENCES app.enrollments(id) ON DELETE RESTRICT,
  student_enrollment_id text NOT NULL REFERENCES app.enrollments(id) ON DELETE RESTRICT,
  ended_at timestamptz,
  deleted_at timestamptz,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT mentorships_distinct_members CHECK (mentor_enrollment_id <> student_enrollment_id)
);
CREATE UNIQUE INDEX mentorships_one_active_mentor
  ON app.mentorships (student_enrollment_id)
  WHERE ended_at IS NULL AND deleted_at IS NULL;

-- +goose StatementBegin
CREATE FUNCTION app.validate_mentorship()
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
      USING ERRCODE = '23514';
  END IF;
  IF mentor_row.role NOT IN ('mentor', 'coordinator') OR student_row.role <> 'student' THEN
    RAISE EXCEPTION 'mentorship requires a mentor/coordinator and a student'
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd
CREATE TRIGGER mentorships_validate
  BEFORE INSERT OR UPDATE OF season_id, mentor_enrollment_id, student_enrollment_id
  ON app.mentorships
  FOR EACH ROW EXECUTE FUNCTION app.validate_mentorship();

-- +goose StatementBegin
CREATE FUNCTION app.validate_enrollment_close_event()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE
  close_season_id text;
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
CREATE TRIGGER enrollments_validate_close_event
  BEFORE INSERT OR UPDATE OF season_id, completed_by_close_id
  ON app.enrollments
  FOR EACH ROW EXECUTE FUNCTION app.validate_enrollment_close_event();

CREATE TABLE app.enrollment_removal_events (
  id text PRIMARY KEY,
  enrollment_id text NOT NULL REFERENCES app.enrollments(id) ON DELETE RESTRICT,
  season_id text NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  subject_user_id text NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  actor_user_id text REFERENCES app.users(id) ON DELETE RESTRICT,
  resulting_state app.enrollment_state NOT NULL,
  reason text NOT NULL,
  occurred_at timestamptz NOT NULL,
  deleted_at_legacy timestamptz,
  CONSTRAINT enrollment_removals_terminal CHECK (resulting_state IN ('kicked', 'withdrawn')),
  CONSTRAINT enrollment_removals_reason_nonempty CHECK (length(trim(reason)) > 0)
);

CREATE TABLE app.problems (
  id text PRIMARY KEY,
  title text NOT NULL,
  url text,
  deleted_at timestamptz,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT problems_title_nonempty CHECK (length(trim(title)) > 0)
);

CREATE TABLE app.leetcode_problems (
  id text PRIMARY KEY,
  problem_id text NOT NULL UNIQUE REFERENCES app.problems(id) ON DELETE RESTRICT,
  leetcode_number integer NOT NULL UNIQUE CHECK (leetcode_number > 0),
  difficulty app.problem_difficulty NOT NULL,
  is_premium boolean NOT NULL DEFAULT false,
  deleted_at timestamptz,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.custom_problems (
  id text PRIMARY KEY,
  problem_id text NOT NULL UNIQUE REFERENCES app.problems(id) ON DELETE RESTRICT,
  difficulty_label text NOT NULL,
  question_html text NOT NULL,
  deleted_at timestamptz,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT custom_problems_difficulty_nonempty CHECK (length(trim(difficulty_label)) > 0)
);

CREATE TABLE app.leetcode_problem_categories (
  id text PRIMARY KEY,
  name text NOT NULL,
  normalized_name text NOT NULL,
  deleted_at timestamptz,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT leetcode_problem_categories_name_nonempty CHECK (length(trim(name)) > 0),
  CONSTRAINT leetcode_problem_categories_normalized_nonempty CHECK (length(trim(normalized_name)) > 0)
);
CREATE UNIQUE INDEX leetcode_problem_categories_normalized_unique
  ON app.leetcode_problem_categories (lower(normalized_name));

CREATE TABLE app.leetcode_problem_category_mappings (
  leetcode_problem_id text NOT NULL REFERENCES app.leetcode_problems(id) ON DELETE CASCADE,
  category_id text NOT NULL REFERENCES app.leetcode_problem_categories(id) ON DELETE CASCADE,
  PRIMARY KEY (leetcode_problem_id, category_id)
);

CREATE TABLE app.practice_goals (
  user_id text PRIMARY KEY REFERENCES app.users(id) ON DELETE RESTRICT,
  enabled boolean NOT NULL DEFAULT false,
  enabled_by_user_id text REFERENCES app.users(id) ON DELETE RESTRICT,
  enabled_at timestamptz,
  easy_minutes integer NOT NULL DEFAULT 20 CHECK (easy_minutes > 0),
  medium_minutes integer NOT NULL DEFAULT 35 CHECK (medium_minutes > 0),
  hard_minutes integer NOT NULL DEFAULT 50 CHECK (hard_minutes > 0),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT practice_goals_enablement CHECK (
    (NOT enabled AND enabled_by_user_id IS NULL AND enabled_at IS NULL)
    OR (enabled AND enabled_by_user_id IS NOT NULL AND enabled_at IS NOT NULL)
  )
);

CREATE TABLE app.problem_attempts (
  id text PRIMARY KEY,
  user_id text NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  problem_id text NOT NULL REFERENCES app.problems(id) ON DELETE RESTRICT,
  enrollment_id text REFERENCES app.enrollments(id) ON DELETE RESTRICT,
  season_week_id text REFERENCES app.season_weeks(id) ON DELETE RESTRICT,
  attempted_at timestamptz NOT NULL,
  time_taken_minutes integer NOT NULL CHECK (time_taken_minutes > 0),
  outcome app.attempt_outcome NOT NULL DEFAULT 'unknown',
  confidence smallint CHECK (confidence BETWEEN 1 AND 5),
  notes_html text,
  notes_sanitization_changed boolean NOT NULL DEFAULT false,
  deleted_at timestamptz,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT problem_attempts_confidence_known CHECK (
    outcome <> 'unknown' OR confidence IS NULL
  )
);
CREATE INDEX problem_attempts_user_recent
  ON app.problem_attempts (user_id, attempted_at DESC, id DESC);

-- +goose StatementBegin
CREATE FUNCTION app.validate_problem_attempt_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE
  enrollment_user_id text;
  enrollment_season_id text;
  week_season_id text;
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
CREATE TRIGGER problem_attempts_validate_scope
  BEFORE INSERT OR UPDATE OF user_id, enrollment_id, season_week_id
  ON app.problem_attempts
  FOR EACH ROW EXECUTE FUNCTION app.validate_problem_attempt_scope();

CREATE TABLE app.recommendations (
  id text PRIMARY KEY,
  user_id text NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  leetcode_problem_id text NOT NULL REFERENCES app.leetcode_problems(id) ON DELETE RESTRICT,
  category_id text REFERENCES app.leetcode_problem_categories(id) ON DELETE RESTRICT,
  difficulty app.problem_difficulty NOT NULL,
  rationale text NOT NULL,
  rule_version text NOT NULL,
  state app.recommendation_state NOT NULL DEFAULT 'active',
  generated_at timestamptz NOT NULL,
  fulfilled_by_attempt_id text REFERENCES app.problem_attempts(id) ON DELETE RESTRICT,
  state_changed_at timestamptz,
  deleted_at timestamptz,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT recommendations_rationale_nonempty CHECK (length(trim(rationale)) > 0),
  CONSTRAINT recommendations_rule_version_nonempty CHECK (length(trim(rule_version)) > 0),
  CONSTRAINT recommendations_attempt_state CHECK (
    (state = 'attempted' AND fulfilled_by_attempt_id IS NOT NULL)
    OR (state <> 'attempted' AND fulfilled_by_attempt_id IS NULL)
  )
);
CREATE UNIQUE INDEX recommendations_one_active
  ON app.recommendations (user_id)
  WHERE state = 'active' AND deleted_at IS NULL;

-- +goose StatementBegin
CREATE FUNCTION app.validate_recommendation_attempt()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE
  attempt_user_id text;
  attempt_problem_id text;
  recommended_problem_id text;
BEGIN
  IF NEW.fulfilled_by_attempt_id IS NULL THEN
    RETURN NEW;
  END IF;
  SELECT user_id, problem_id INTO STRICT attempt_user_id, attempt_problem_id
  FROM app.problem_attempts WHERE id = NEW.fulfilled_by_attempt_id;
  SELECT problem_id INTO STRICT recommended_problem_id
  FROM app.leetcode_problems WHERE id = NEW.leetcode_problem_id;
  IF attempt_user_id <> NEW.user_id OR attempt_problem_id <> recommended_problem_id THEN
    RAISE EXCEPTION 'recommendation attempt must belong to the same user and problem'
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd
CREATE TRIGGER recommendations_validate_attempt
  BEFORE INSERT OR UPDATE OF user_id, leetcode_problem_id, fulfilled_by_attempt_id
  ON app.recommendations
  FOR EACH ROW EXECUTE FUNCTION app.validate_recommendation_attempt();

CREATE TABLE app.recommendation_dismissals (
  id text PRIMARY KEY,
  recommendation_id text NOT NULL UNIQUE REFERENCES app.recommendations(id) ON DELETE RESTRICT,
  user_id text NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  leetcode_problem_id text NOT NULL REFERENCES app.leetcode_problems(id) ON DELETE RESTRICT,
  reason text,
  dismissed_at timestamptz NOT NULL,
  excluded_until timestamptz NOT NULL,
  CONSTRAINT recommendation_dismissals_window CHECK (excluded_until > dismissed_at)
);

CREATE TABLE app.mock_interviews (
  id text PRIMARY KEY,
  interviewer_user_id text NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  interviewee_user_id text NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  season_id text REFERENCES app.seasons(id) ON DELETE RESTRICT,
  season_week_id text REFERENCES app.season_weeks(id) ON DELETE RESTRICT,
  scheduled_at timestamptz NOT NULL,
  duration_minutes integer NOT NULL CHECK (duration_minutes > 0),
  legacy_is_pass boolean,
  interviewer_notes_html text,
  notes_sanitization_changed boolean NOT NULL DEFAULT false,
  deleted_at timestamptz,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT mock_interviews_distinct_people CHECK (interviewer_user_id <> interviewee_user_id)
);
CREATE INDEX mock_interviews_interviewer_recent
  ON app.mock_interviews (interviewer_user_id, scheduled_at DESC, id DESC);
CREATE INDEX mock_interviews_interviewee_recent
  ON app.mock_interviews (interviewee_user_id, scheduled_at DESC, id DESC);

-- +goose StatementBegin
CREATE FUNCTION app.validate_mock_interview_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE
  week_season_id text;
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

CREATE TABLE app.mock_interview_rounds (
  id text PRIMARY KEY,
  mock_interview_id text NOT NULL REFERENCES app.mock_interviews(id) ON DELETE CASCADE,
  position integer NOT NULL CHECK (position > 0),
  kind app.mock_round_kind NOT NULL,
  review_status app.review_status NOT NULL DEFAULT 'pending',
  interviewee_comment_html text,
  review_sanitization_changed boolean NOT NULL DEFAULT false,
  reviewed_at timestamptz,
  deleted_at timestamptz,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (mock_interview_id, position),
  CONSTRAINT mock_round_review_state CHECK (
    (review_status = 'pending' AND reviewed_at IS NULL)
    OR (review_status = 'reviewed' AND reviewed_at IS NOT NULL)
  )
);

CREATE TABLE app.behavioural_mock_interview_rounds (
  id text PRIMARY KEY,
  mock_interview_round_id text NOT NULL UNIQUE REFERENCES app.mock_interview_rounds(id) ON DELETE CASCADE,
  behavioural_score smallint NOT NULL CHECK (behavioural_score BETWEEN 0 AND 10),
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.leetcode_mock_interview_rounds (
  id text PRIMARY KEY,
  mock_interview_round_id text NOT NULL UNIQUE REFERENCES app.mock_interview_rounds(id) ON DELETE CASCADE,
  leetcode_problem_id text NOT NULL REFERENCES app.leetcode_problems(id) ON DELETE RESTRICT,
  clarify_question_score smallint NOT NULL CHECK (clarify_question_score BETWEEN 0 AND 10),
  algorithm_design_score smallint NOT NULL CHECK (algorithm_design_score BETWEEN 0 AND 10),
  complexity_analysis_score smallint NOT NULL CHECK (complexity_analysis_score BETWEEN 0 AND 10),
  coding_score smallint NOT NULL CHECK (coding_score BETWEEN 0 AND 10),
  testing_score smallint NOT NULL CHECK (testing_score BETWEEN 0 AND 10),
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.custom_mock_interview_rounds (
  id text PRIMARY KEY,
  mock_interview_round_id text NOT NULL UNIQUE REFERENCES app.mock_interview_rounds(id) ON DELETE CASCADE,
  content_html text,
  url text,
  score smallint NOT NULL CHECK (score BETWEEN 0 AND 10),
  content_sanitization_changed boolean NOT NULL DEFAULT false,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose StatementBegin
CREATE FUNCTION app.validate_mock_round_subtype()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE
  expected_kind app.mock_round_kind;
  actual_kind app.mock_round_kind;
BEGIN
  expected_kind := TG_ARGV[0]::app.mock_round_kind;
  SELECT kind INTO STRICT actual_kind
  FROM app.mock_interview_rounds
  WHERE id = NEW.mock_interview_round_id;
  IF actual_kind <> expected_kind THEN
    RAISE EXCEPTION 'round subtype % does not match parent kind %', expected_kind, actual_kind
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd
CREATE TRIGGER behavioural_round_kind
  BEFORE INSERT OR UPDATE OF mock_interview_round_id
  ON app.behavioural_mock_interview_rounds
  FOR EACH ROW EXECUTE FUNCTION app.validate_mock_round_subtype('behavioural');
CREATE TRIGGER leetcode_round_kind
  BEFORE INSERT OR UPDATE OF mock_interview_round_id
  ON app.leetcode_mock_interview_rounds
  FOR EACH ROW EXECUTE FUNCTION app.validate_mock_round_subtype('leetcode');
CREATE TRIGGER custom_round_kind
  BEFORE INSERT OR UPDATE OF mock_interview_round_id
  ON app.custom_mock_interview_rounds
  FOR EACH ROW EXECUTE FUNCTION app.validate_mock_round_subtype('custom');

-- +goose StatementBegin
CREATE FUNCTION app.validate_mock_round_has_subtype()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE
  subtype_count integer;
BEGIN
  SELECT
    (SELECT count(*) FROM app.behavioural_mock_interview_rounds WHERE mock_interview_round_id = NEW.id)
    + (SELECT count(*) FROM app.leetcode_mock_interview_rounds WHERE mock_interview_round_id = NEW.id)
    + (SELECT count(*) FROM app.custom_mock_interview_rounds WHERE mock_interview_round_id = NEW.id)
  INTO subtype_count;
  IF subtype_count <> 1 THEN
    RAISE EXCEPTION 'mock interview round must have exactly one subtype row'
      USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER mock_rounds_require_one_subtype
  AFTER INSERT OR UPDATE OF kind ON app.mock_interview_rounds
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION app.validate_mock_round_has_subtype();

CREATE TABLE app.mock_interview_versions (
  id text PRIMARY KEY,
  mock_interview_id text NOT NULL REFERENCES app.mock_interviews(id) ON DELETE RESTRICT,
  version bigint NOT NULL CHECK (version > 0),
  actor_user_id text REFERENCES app.users(id) ON DELETE RESTRICT,
  reason text NOT NULL,
  snapshot jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (mock_interview_id, version),
  CONSTRAINT mock_interview_versions_reason_nonempty CHECK (length(trim(reason)) > 0),
  CONSTRAINT mock_interview_versions_snapshot_object CHECK (jsonb_typeof(snapshot) = 'object')
);

CREATE TABLE app.leetcode_sync_runs (
  id text PRIMARY KEY,
  trigger_kind text NOT NULL CHECK (trigger_kind IN ('schedule', 'catch_up', 'manual')),
  requested_by_user_id text REFERENCES app.users(id) ON DELETE RESTRICT,
  started_at timestamptz NOT NULL,
  finished_at timestamptz,
  succeeded boolean,
  fetched_count integer CHECK (fetched_count >= 0),
  changed_count integer CHECK (changed_count >= 0),
  error_summary text,
  CONSTRAINT leetcode_sync_runs_terminal CHECK (
    (finished_at IS NULL AND succeeded IS NULL)
    OR (finished_at >= started_at AND succeeded IS NOT NULL)
  )
);

CREATE TABLE app.audit_events (
  id text PRIMARY KEY,
  actor_user_id text REFERENCES app.users(id) ON DELETE RESTRICT,
  action text NOT NULL,
  subject_type text NOT NULL,
  subject_id text NOT NULL,
  request_id text,
  ip_hash text,
  data jsonb NOT NULL DEFAULT '{}'::jsonb,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT audit_events_action_nonempty CHECK (length(trim(action)) > 0),
  CONSTRAINT audit_events_subject_nonempty CHECK (
    length(trim(subject_type)) > 0 AND length(trim(subject_id)) > 0
  ),
  CONSTRAINT audit_events_data_object CHECK (jsonb_typeof(data) = 'object')
);
CREATE INDEX audit_events_subject_recent
  ON app.audit_events (subject_type, subject_id, occurred_at DESC, id DESC);

CREATE TRIGGER enrollment_removal_events_append_only
  BEFORE UPDATE OR DELETE ON app.enrollment_removal_events
  FOR EACH ROW EXECUTE FUNCTION app.reject_immutable_change();
CREATE TRIGGER mock_interview_versions_append_only
  BEFORE UPDATE OR DELETE ON app.mock_interview_versions
  FOR EACH ROW EXECUTE FUNCTION app.reject_immutable_change();
CREATE TRIGGER audit_events_append_only
  BEFORE UPDATE OR DELETE ON app.audit_events
  FOR EACH ROW EXECUTE FUNCTION app.reject_immutable_change();
REVOKE UPDATE, DELETE ON app.enrollment_removal_events FROM PUBLIC;
REVOKE UPDATE, DELETE ON app.mock_interview_versions FROM PUBLIC;
REVOKE UPDATE, DELETE ON app.audit_events FROM PUBLIC;

CREATE TRIGGER users_touch_updated_at
  BEFORE UPDATE ON app.users FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER seasons_touch_updated_at
  BEFORE UPDATE ON app.seasons FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER season_weeks_touch_updated_at
  BEFORE UPDATE ON app.season_weeks FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER enrollments_touch_updated_at
  BEFORE UPDATE ON app.enrollments FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER mentorships_touch_updated_at
  BEFORE UPDATE ON app.mentorships FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER problems_touch_updated_at
  BEFORE UPDATE ON app.problems FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER leetcode_problems_touch_updated_at
  BEFORE UPDATE ON app.leetcode_problems FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER custom_problems_touch_updated_at
  BEFORE UPDATE ON app.custom_problems FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER leetcode_problem_categories_touch_updated_at
  BEFORE UPDATE ON app.leetcode_problem_categories FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER practice_goals_touch_updated_at
  BEFORE UPDATE ON app.practice_goals FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER problem_attempts_touch_updated_at
  BEFORE UPDATE ON app.problem_attempts FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER recommendations_touch_updated_at
  BEFORE UPDATE ON app.recommendations FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER mock_interviews_touch_updated_at
  BEFORE UPDATE ON app.mock_interviews FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER mock_interview_rounds_touch_updated_at
  BEFORE UPDATE ON app.mock_interview_rounds FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();

CREATE TABLE migration.runs (
  id text PRIMARY KEY,
  manifest_checksum text NOT NULL UNIQUE,
  source_schema_fingerprint text NOT NULL,
  source_snapshot_at timestamptz NOT NULL,
  state app.migration_run_state NOT NULL DEFAULT 'planned',
  started_at timestamptz,
  finished_at timestamptz,
  rolled_back_at timestamptz,
  error_summary text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE migration.source_tables (
  run_id text NOT NULL REFERENCES migration.runs(id) ON DELETE RESTRICT,
  source_table text NOT NULL,
  source_count bigint NOT NULL CHECK (source_count >= 0),
  imported_count bigint NOT NULL CHECK (imported_count >= 0),
  source_ids_checksum text NOT NULL,
  transformed_checksum text NOT NULL,
  PRIMARY KEY (run_id, source_table)
);

CREATE TABLE migration.row_provenance (
  run_id text NOT NULL REFERENCES migration.runs(id) ON DELETE RESTRICT,
  source_table text NOT NULL,
  source_id text NOT NULL,
  target_table text NOT NULL,
  target_id text NOT NULL,
  source_checksum text NOT NULL,
  transformed_checksum text NOT NULL,
  transformed_data jsonb NOT NULL,
  duplicate_group text,
  PRIMARY KEY (run_id, source_table, source_id, target_table, target_id)
);
CREATE INDEX row_provenance_target
  ON migration.row_provenance (target_table, target_id);

CREATE TABLE migration.anomalies (
  run_id text NOT NULL REFERENCES migration.runs(id) ON DELETE RESTRICT,
  code text NOT NULL,
  source_table text NOT NULL,
  source_id text,
  severity migration.anomaly_severity NOT NULL,
  detail text NOT NULL,
  context jsonb NOT NULL DEFAULT '{}'::jsonb,
  resolution_checksum text,
  PRIMARY KEY (run_id, code, source_table, source_id)
);

CREATE TABLE migration.resolutions (
  run_id text NOT NULL REFERENCES migration.runs(id) ON DELETE RESTRICT,
  anomaly_code text NOT NULL,
  source_table text NOT NULL,
  source_id text,
  action migration.resolution_action NOT NULL,
  value jsonb NOT NULL,
  resolution_file_checksum text NOT NULL,
  applied_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id, anomaly_code, source_table, source_id)
);

CREATE TABLE migration.auto_fixes (
  run_id text NOT NULL REFERENCES migration.runs(id) ON DELETE RESTRICT,
  source_table text NOT NULL,
  source_id text,
  code text NOT NULL,
  before_value jsonb,
  after_value jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id, source_table, source_id, code)
);

CREATE TABLE migration.auth0_identity_imports (
  run_id text NOT NULL REFERENCES migration.runs(id) ON DELETE RESTRICT,
  auth0_user_id text NOT NULL,
  app_user_id text REFERENCES app.users(id) ON DELETE RESTRICT,
  provider text,
  provider_account_id text,
  email_verified boolean,
  password_hash_compatible boolean,
  status text NOT NULL CHECK (status IN ('matched', 'requires_resolution', 'password_reset_required', 'imported')),
  detail text,
  PRIMARY KEY (run_id, auth0_user_id)
);

ALTER TABLE app.season_close_events
  ADD CONSTRAINT season_close_events_migration_run_fk
  FOREIGN KEY (migration_run_id) REFERENCES migration.runs(id)
  ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED;

CREATE TRIGGER migration_source_tables_append_only
  BEFORE UPDATE OR DELETE ON migration.source_tables
  FOR EACH ROW EXECUTE FUNCTION app.reject_immutable_change();
CREATE TRIGGER migration_row_provenance_append_only
  BEFORE UPDATE OR DELETE ON migration.row_provenance
  FOR EACH ROW EXECUTE FUNCTION app.reject_immutable_change();
CREATE TRIGGER migration_anomalies_append_only
  BEFORE UPDATE OR DELETE ON migration.anomalies
  FOR EACH ROW EXECUTE FUNCTION app.reject_immutable_change();
CREATE TRIGGER migration_resolutions_append_only
  BEFORE UPDATE OR DELETE ON migration.resolutions
  FOR EACH ROW EXECUTE FUNCTION app.reject_immutable_change();
CREATE TRIGGER migration_auto_fixes_append_only
  BEFORE UPDATE OR DELETE ON migration.auto_fixes
  FOR EACH ROW EXECUTE FUNCTION app.reject_immutable_change();
CREATE TRIGGER migration_auth0_identity_imports_append_only
  BEFORE UPDATE OR DELETE ON migration.auth0_identity_imports
  FOR EACH ROW EXECUTE FUNCTION app.reject_immutable_change();

-- +goose Down
DROP SCHEMA migration CASCADE;
DROP SCHEMA app CASCADE;
