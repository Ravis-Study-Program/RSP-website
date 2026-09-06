-- +goose Up

-- Core account, season, enrollment, and mentorship schema.
CREATE SCHEMA app;

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

CREATE TYPE app.mock_round_kind AS ENUM ('behavioural', 'leetcode', 'custom');

CREATE TYPE app.review_status AS ENUM ('pending', 'reviewed');

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
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  account_state app.account_state NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE TABLE app.user_profiles (
  user_id uuid PRIMARY KEY REFERENCES app.users(id) ON DELETE RESTRICT,
  updated_at timestamptz NOT NULL DEFAULT now(),
  slug text NOT NULL,
    CHECK (length(trim(slug)) > 0 AND slug = lower(slug)),
  display_name text NOT NULL,
    CHECK (length(trim(display_name)) > 0),
  avatar_url text
);
CREATE UNIQUE INDEX user_profiles_slug_unique ON app.user_profiles (lower(slug));

CREATE TABLE app.user_preferences (
  user_id uuid PRIMARY KEY REFERENCES app.users(id) ON DELETE RESTRICT,
  updated_at timestamptz NOT NULL DEFAULT now(),
  timezone text NOT NULL DEFAULT 'Australia/Adelaide',
  timezone_configured boolean NOT NULL DEFAULT true
);

CREATE TABLE app.user_contacts (
  user_id uuid PRIMARY KEY REFERENCES app.users(id) ON DELETE RESTRICT,
  updated_at timestamptz NOT NULL DEFAULT now(),
  email text,
  discord_id text
);
CREATE UNIQUE INDEX user_contacts_email_unique
  ON app.user_contacts (lower(email))
  WHERE email IS NOT NULL;

CREATE TABLE app.user_security (
  user_id uuid PRIMARY KEY REFERENCES app.users(id) ON DELETE RESTRICT,
  updated_at timestamptz NOT NULL DEFAULT now(),
  security_version bigint NOT NULL DEFAULT 1 CHECK (security_version > 0),
  mfa_configured boolean NOT NULL DEFAULT false
);

CREATE TABLE app.identity_event_receipts (
  event_id text PRIMARY KEY,
  auth_subject text NOT NULL,
  event_type text NOT NULL,
  security_version bigint NOT NULL CHECK (security_version > 0),
  received_at timestamptz NOT NULL DEFAULT now(),
  payload_hash text NOT NULL
);

CREATE TABLE app.user_auth_links (
  auth_subject text NOT NULL,
  user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  provider text NOT NULL,
    CHECK (length(trim(provider)) > 0),
  provider_account_id text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  linked_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz,
  PRIMARY KEY (auth_subject),
  UNIQUE (provider, provider_account_id),
  CONSTRAINT user_auth_links_revocation CHECK (
    (active AND revoked_at IS NULL) OR (NOT active AND revoked_at IS NOT NULL)
  )
);
CREATE UNIQUE INDEX user_auth_links_one_active_subject
  ON app.user_auth_links (user_id, auth_subject)
  WHERE active;

CREATE TABLE app.global_role_assignments (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  role app.global_role NOT NULL,
  state app.assignment_state NOT NULL DEFAULT 'pending_mfa',
  granted_by_user_id uuid REFERENCES app.users(id) ON DELETE RESTRICT,
  granted_at timestamptz NOT NULL DEFAULT now(),
  activated_at timestamptz,
  revoked_at timestamptz,
  UNIQUE (user_id, role),
  CONSTRAINT global_role_assignment_state CHECK (
    (state = 'pending_mfa' AND activated_at IS NULL AND revoked_at IS NULL)
    OR (state = 'active' AND activated_at IS NOT NULL AND revoked_at IS NULL)
    OR (state = 'revoked' AND revoked_at IS NOT NULL)
  )
);

CREATE TABLE app.account_deletion_requests (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
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
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  slug text NOT NULL,
    CHECK (
      (length(slug) BETWEEN 1 AND 50)
      AND
      (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
    ),
  name text NOT NULL,
    CHECK (length(trim(name)) > 0),
  status app.season_status NOT NULL DEFAULT 'open',
  start_at timestamptz NOT NULL,
  end_at timestamptz NOT NULL,
  location text NOT NULL DEFAULT '',
  image_url text NOT NULL DEFAULT '',
  resources_url text NOT NULL DEFAULT '',
  closed_at timestamptz,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT seasons_date_order CHECK (end_at >= start_at),
  CONSTRAINT seasons_status_closed_at CHECK (
    (status = 'open' AND closed_at IS NULL)
    OR (status = 'closed' AND closed_at IS NOT NULL)
  )
);
CREATE UNIQUE INDEX seasons_slug_unique ON app.seasons (lower(slug));

CREATE TABLE app.season_close_events (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  season_id uuid NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  closed_by_user_id uuid REFERENCES app.users(id) ON DELETE RESTRICT,
  close_reason text NOT NULL,
    CHECK (length(trim(close_reason)) > 0),
  closed_at timestamptz NOT NULL,
  reopened_by_user_id uuid REFERENCES app.users(id) ON DELETE RESTRICT,
  reopen_reason text,
  reopened_at timestamptz,
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
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  season_id uuid NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  week_number integer NOT NULL CHECK (week_number > 0),
  start_at timestamptz NOT NULL,
  end_at timestamptz NOT NULL,
  resource_url text NOT NULL DEFAULT '',
  deleted_at timestamptz,
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
      USING ERRCODE = 'check_violation';
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
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  season_id uuid NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  role app.season_role NOT NULL,
  student_level app.student_level NOT NULL DEFAULT 'not_applicable',
  state app.enrollment_state NOT NULL DEFAULT 'active',
  assignment_state app.assignment_state NOT NULL DEFAULT 'active',
  activated_at timestamptz DEFAULT now(),
  completed_by_close_id uuid REFERENCES app.season_close_events(id) ON DELETE RESTRICT,
  close_assignment_state app.assignment_state,
  close_activated_at timestamptz,
  state_changed_at timestamptz,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, season_id),
  CONSTRAINT enrollments_student_level CHECK (
    (role = 'student' AND student_level <> 'not_applicable')
    OR (role <> 'student' AND student_level = 'not_applicable')
  ),
  CONSTRAINT enrollments_close_completion CHECK (
    completed_by_close_id IS NULL OR state = 'completed'
  ),
  CONSTRAINT enrollments_coordinator_mfa CHECK (
    (role='coordinator' AND state='active' AND assignment_state='pending_mfa' AND activated_at IS NULL)
    OR (state='active' AND assignment_state='active' AND activated_at IS NOT NULL)
    OR (state<>'active' AND assignment_state='revoked' AND activated_at IS NULL)
  ),
  CONSTRAINT enrollments_close_assignment_snapshot CHECK (
    (close_assignment_state IS NULL AND close_activated_at IS NULL)
    OR (completed_by_close_id IS NOT NULL AND state='completed' AND close_assignment_state IS NOT NULL)
  )
);

CREATE TABLE app.mentorships (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  season_id uuid NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  mentor_enrollment_id uuid NOT NULL REFERENCES app.enrollments(id) ON DELETE RESTRICT,
  student_enrollment_id uuid NOT NULL REFERENCES app.enrollments(id) ON DELETE RESTRICT,
  ended_at timestamptz,
  deleted_at timestamptz,
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
      USING ERRCODE = 'check_violation';
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
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  enrollment_id uuid NOT NULL REFERENCES app.enrollments(id) ON DELETE RESTRICT,
  season_id uuid NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  subject_user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  actor_user_id uuid REFERENCES app.users(id) ON DELETE RESTRICT,
  resulting_state app.enrollment_state NOT NULL,
  reason text NOT NULL,
  occurred_at timestamptz NOT NULL,
  CONSTRAINT enrollment_removals_terminal CHECK (resulting_state IN ('kicked', 'withdrawn')),
  CONSTRAINT enrollment_removals_reason_nonempty CHECK (length(trim(reason)) > 0)
);

-- Privileged assignments derive their state from the mirrored MFA record.
-- The trigger prevents callers from bypassing that invariant.
-- +goose StatementBegin
CREATE FUNCTION app.require_coordinator_mfa()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE configured boolean;
BEGIN
  IF NEW.role='coordinator' AND NEW.state='active' THEN
    SELECT s.mfa_configured INTO configured
    FROM app.user_security s
    WHERE s.user_id=NEW.user_id;
    IF COALESCE(configured,false) THEN
      NEW.assignment_state := 'active';
      NEW.activated_at := COALESCE(NEW.activated_at,now());
    ELSE
      NEW.assignment_state := 'pending_mfa';
      NEW.activated_at := NULL;
    END IF;
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd

CREATE TRIGGER enrollments_require_coordinator_mfa
  BEFORE INSERT OR UPDATE OF role,state,user_id ON app.enrollments
  FOR EACH ROW EXECUTE FUNCTION app.require_coordinator_mfa();

-- +goose StatementBegin
CREATE FUNCTION app.require_global_role_mfa()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE configured boolean;
BEGIN
  IF NEW.state <> 'revoked' THEN
    SELECT s.mfa_configured INTO configured
    FROM app.user_security s
    WHERE s.user_id=NEW.user_id;
    IF COALESCE(configured,false) THEN
      NEW.state := 'active';
      NEW.activated_at := COALESCE(NEW.activated_at,now());
      NEW.revoked_at := NULL;
    ELSE
      NEW.state := 'pending_mfa';
      NEW.activated_at := NULL;
      NEW.revoked_at := NULL;
    END IF;
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd

CREATE TRIGGER global_role_assignments_require_mfa
  BEFORE INSERT OR UPDATE OF user_id,role,state ON app.global_role_assignments
  FOR EACH ROW EXECUTE FUNCTION app.require_global_role_mfa();

CREATE TRIGGER enrollment_removal_events_append_only
  BEFORE UPDATE OR DELETE ON app.enrollment_removal_events
  FOR EACH ROW EXECUTE FUNCTION app.reject_immutable_change();
REVOKE UPDATE, DELETE ON app.enrollment_removal_events FROM PUBLIC;

CREATE TRIGGER users_touch_updated_at
  BEFORE UPDATE ON app.users FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER user_profiles_touch_updated_at
  BEFORE UPDATE ON app.user_profiles FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER user_preferences_touch_updated_at
  BEFORE UPDATE ON app.user_preferences FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER user_contacts_touch_updated_at
  BEFORE UPDATE ON app.user_contacts FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER user_security_touch_updated_at
  BEFORE UPDATE ON app.user_security FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER seasons_touch_updated_at
  BEFORE UPDATE ON app.seasons FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER season_weeks_touch_updated_at
  BEFORE UPDATE ON app.season_weeks FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER enrollments_touch_updated_at
  BEFORE UPDATE ON app.enrollments FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();
CREATE TRIGGER mentorships_touch_updated_at
  BEFORE UPDATE ON app.mentorships FOR EACH ROW EXECUTE FUNCTION app.touch_updated_at();

-- +goose Down
DROP SCHEMA app CASCADE;
