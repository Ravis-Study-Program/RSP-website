-- Problem, practice, mock interview, and audit domain schema.

-- +goose Up

CREATE TABLE app.problems (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  title text NOT NULL,
    CHECK (length(trim(title)) > 0),
  url text,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.leetcode_problems (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  problem_id uuid NOT NULL UNIQUE REFERENCES app.problems(id) ON DELETE RESTRICT,
  leetcode_number integer NOT NULL UNIQUE CHECK (leetcode_number > 0),
  difficulty app.problem_difficulty NOT NULL,
  is_premium boolean NOT NULL DEFAULT false,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.custom_problems (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  problem_id uuid NOT NULL UNIQUE REFERENCES app.problems(id) ON DELETE RESTRICT,
  difficulty_label text NOT NULL,
    CHECK (length(trim(difficulty_label)) > 0),
  question_html text NOT NULL,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.leetcode_problem_categories (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  name text NOT NULL,
    CHECK (length(trim(name)) > 0),
  normalized_name text NOT NULL,
    CHECK (length(trim(normalized_name)) > 0),
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX leetcode_problem_categories_normalized_unique
  ON app.leetcode_problem_categories (lower(normalized_name));

CREATE TABLE app.leetcode_problem_category_mappings (
  leetcode_problem_id uuid NOT NULL REFERENCES app.leetcode_problems(id) ON DELETE CASCADE,
  category_id uuid NOT NULL REFERENCES app.leetcode_problem_categories(id) ON DELETE CASCADE,
  PRIMARY KEY (leetcode_problem_id, category_id)
);

CREATE TABLE app.practice_goals (
  user_id uuid PRIMARY KEY REFERENCES app.users(id) ON DELETE RESTRICT,
  enabled boolean NOT NULL DEFAULT false,
  enabled_by_user_id uuid REFERENCES app.users(id) ON DELETE RESTRICT,
  enabled_at timestamptz,
  easy_minutes integer NOT NULL DEFAULT 10 CHECK (easy_minutes BETWEEN 1 AND 180),
  medium_minutes integer NOT NULL DEFAULT 20 CHECK (medium_minutes BETWEEN 1 AND 180),
  hard_minutes integer NOT NULL DEFAULT 45 CHECK (hard_minutes BETWEEN 1 AND 180),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT practice_goals_enablement CHECK (
    (NOT enabled AND enabled_by_user_id IS NULL AND enabled_at IS NULL)
    OR (enabled AND enabled_by_user_id IS NOT NULL AND enabled_at IS NOT NULL)
  )
);

CREATE TABLE app.problem_attempts (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  problem_id uuid NOT NULL REFERENCES app.problems(id) ON DELETE RESTRICT,
  enrollment_id uuid REFERENCES app.enrollments(id) ON DELETE RESTRICT,
  season_week_id uuid REFERENCES app.season_weeks(id) ON DELETE RESTRICT,
  attempted_at timestamptz NOT NULL,
  time_taken_minutes integer NOT NULL CHECK (time_taken_minutes > 0),
  outcome app.attempt_outcome NOT NULL DEFAULT 'unknown',
  confidence smallint CHECK (confidence BETWEEN 1 AND 5),
  notes_html text,
  notes_sanitization_changed boolean NOT NULL DEFAULT false,
  deleted_at timestamptz,
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
CREATE TRIGGER problem_attempts_validate_scope
  BEFORE INSERT OR UPDATE OF user_id, enrollment_id, season_week_id
  ON app.problem_attempts
  FOR EACH ROW EXECUTE FUNCTION app.validate_problem_attempt_scope();

CREATE TABLE app.mock_interviews (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  interviewer_user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  interviewee_user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  season_id uuid REFERENCES app.seasons(id) ON DELETE RESTRICT,
  season_week_id uuid REFERENCES app.season_weeks(id) ON DELETE RESTRICT,
  scheduled_at timestamptz NOT NULL,
  duration_minutes integer NOT NULL CHECK (duration_minutes > 0),
  interviewer_notes_html text,
  notes_sanitization_changed boolean NOT NULL DEFAULT false,
  deleted_at timestamptz,
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

CREATE TABLE app.mock_interview_rounds (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  mock_interview_id uuid NOT NULL REFERENCES app.mock_interviews(id) ON DELETE CASCADE,
  position integer NOT NULL CHECK (position > 0),
  kind app.mock_round_kind NOT NULL,
  review_status app.review_status NOT NULL DEFAULT 'pending',
  interviewee_comment_html text,
  review_sanitization_changed boolean NOT NULL DEFAULT false,
  reviewed_at timestamptz,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (mock_interview_id, position),
  CONSTRAINT mock_round_review_state CHECK (
    (review_status = 'pending' AND reviewed_at IS NULL)
    OR (review_status = 'reviewed' AND reviewed_at IS NOT NULL)
  )
);

CREATE TABLE app.behavioural_mock_interview_rounds (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  mock_interview_round_id uuid NOT NULL UNIQUE REFERENCES app.mock_interview_rounds(id) ON DELETE CASCADE,
  behavioural_score smallint NOT NULL CHECK (behavioural_score BETWEEN 0 AND 10),
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.leetcode_mock_interview_rounds (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  mock_interview_round_id uuid NOT NULL UNIQUE REFERENCES app.mock_interview_rounds(id) ON DELETE CASCADE,
  leetcode_problem_id uuid NOT NULL REFERENCES app.leetcode_problems(id) ON DELETE RESTRICT,
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
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  mock_interview_round_id uuid NOT NULL UNIQUE REFERENCES app.mock_interview_rounds(id) ON DELETE CASCADE,
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
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  mock_interview_id uuid NOT NULL REFERENCES app.mock_interviews(id) ON DELETE RESTRICT,
  version bigint NOT NULL CHECK (version > 0),
  actor_user_id uuid REFERENCES app.users(id) ON DELETE RESTRICT,
  reason text NOT NULL,
  snapshot jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (mock_interview_id, version),
  CONSTRAINT mock_interview_versions_reason_nonempty CHECK (length(trim(reason)) > 0),
  CONSTRAINT mock_interview_versions_snapshot_object CHECK (jsonb_typeof(snapshot) = 'object')
);

CREATE TABLE app.leetcode_sync_runs (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  trigger_kind text NOT NULL CHECK (trigger_kind IN ('schedule', 'catch_up', 'manual')),
  requested_by_user_id uuid REFERENCES app.users(id) ON DELETE RESTRICT,
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
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  actor_user_id uuid REFERENCES app.users(id) ON DELETE RESTRICT,
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

CREATE TRIGGER mock_interview_versions_append_only
  BEFORE UPDATE OR DELETE ON app.mock_interview_versions
  FOR EACH ROW EXECUTE FUNCTION app.reject_immutable_change();
CREATE TRIGGER audit_events_append_only
  BEFORE UPDATE OR DELETE ON app.audit_events
  FOR EACH ROW EXECUTE FUNCTION app.reject_immutable_change();
REVOKE UPDATE, DELETE ON app.mock_interview_versions FROM PUBLIC;
REVOKE UPDATE, DELETE ON app.audit_events FROM PUBLIC;

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
CREATE TRIGGER mock_interviews_touch_updated_at
  BEFORE UPDATE ON app.mock_interviews FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER mock_interview_rounds_touch_updated_at
  BEFORE UPDATE ON app.mock_interview_rounds FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS seasons_validate_child_dates ON app.seasons;
DROP TABLE IF EXISTS app.audit_events, app.leetcode_sync_runs, app.mock_interview_versions,
  app.custom_mock_interview_rounds, app.leetcode_mock_interview_rounds,
  app.behavioural_mock_interview_rounds, app.mock_interview_rounds, app.mock_interviews,
  app.problem_attempts, app.practice_goals, app.leetcode_problem_category_mappings,
  app.leetcode_problem_categories, app.custom_problems, app.leetcode_problems, app.problems CASCADE;
DROP FUNCTION IF EXISTS app.validate_mock_round_has_subtype();
DROP FUNCTION IF EXISTS app.validate_mock_round_subtype();
DROP FUNCTION IF EXISTS app.validate_season_child_dates();
DROP FUNCTION IF EXISTS app.validate_mock_interview_scope();
DROP FUNCTION IF EXISTS app.validate_problem_attempt_scope();
