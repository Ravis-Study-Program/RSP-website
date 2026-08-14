-- +goose Up

ALTER TABLE app.users ADD COLUMN mfa_configured boolean NOT NULL DEFAULT false;

ALTER TABLE app.identity_event_receipts ADD COLUMN payload_hash text NOT NULL DEFAULT '';
ALTER TABLE app.identity_event_receipts ALTER COLUMN payload_hash DROP DEFAULT;

ALTER TABLE app.enrollments
  ADD COLUMN close_assignment_state app.assignment_state,
  ADD COLUMN close_activated_at timestamptz,
  ADD CONSTRAINT enrollments_close_assignment_snapshot CHECK (
    (close_assignment_state IS NULL AND close_activated_at IS NULL)
    OR (completed_by_close_id IS NOT NULL AND state='completed' AND close_assignment_state IS NOT NULL)
  );

DROP TRIGGER enrollments_require_coordinator_mfa ON app.enrollments;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app.require_coordinator_mfa()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE configured boolean;
BEGIN
  IF NEW.role='coordinator' AND NEW.state='active' THEN
    SELECT u.mfa_configured INTO configured FROM app.users u WHERE u.id=NEW.user_id;
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

-- +goose Down

DROP TRIGGER enrollments_require_coordinator_mfa ON app.enrollments;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app.require_coordinator_mfa()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
  IF NEW.role='coordinator' AND NEW.state='active' AND TG_OP='INSERT' THEN
    NEW.assignment_state := 'pending_mfa';
    NEW.activated_at := NULL;
  END IF;
  RETURN NEW;
END
$function$;
-- +goose StatementEnd

CREATE TRIGGER enrollments_require_coordinator_mfa
  BEFORE INSERT ON app.enrollments
  FOR EACH ROW EXECUTE FUNCTION app.require_coordinator_mfa();

ALTER TABLE app.enrollments DROP CONSTRAINT enrollments_close_assignment_snapshot;
ALTER TABLE app.enrollments DROP COLUMN close_activated_at;
ALTER TABLE app.enrollments DROP COLUMN close_assignment_state;
ALTER TABLE app.users DROP COLUMN mfa_configured;
ALTER TABLE app.identity_event_receipts DROP COLUMN payload_hash;
