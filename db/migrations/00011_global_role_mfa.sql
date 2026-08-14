-- +goose Up

-- Privileged assignments are never allowed to rely on caller-provided state.
-- The mirrored MFA state is maintained by the idempotent identity lifecycle
-- callback in the same database as these assignments.
-- +goose StatementBegin
CREATE FUNCTION app.require_global_role_mfa()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE configured boolean;
BEGIN
  IF NEW.state <> 'revoked' THEN
    SELECT u.mfa_configured INTO configured
    FROM app.users u
    WHERE u.id=NEW.user_id;

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

-- Reconcile any rows written before this invariant was installed.
UPDATE app.global_role_assignments g
SET state = CASE WHEN u.mfa_configured THEN 'active'::app.assignment_state ELSE 'pending_mfa'::app.assignment_state END,
    activated_at = CASE WHEN u.mfa_configured THEN COALESCE(g.activated_at,now()) ELSE NULL END,
    revoked_at = NULL
FROM app.users u
WHERE g.user_id=u.id AND g.state<>'revoked';

-- +goose Down

DROP TRIGGER global_role_assignments_require_mfa ON app.global_role_assignments;
DROP FUNCTION app.require_global_role_mfa();
