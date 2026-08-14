-- +goose Up

ALTER TABLE app.enrollments
  ADD COLUMN assignment_state app.assignment_state NOT NULL DEFAULT 'active',
  ADD COLUMN activated_at timestamptz DEFAULT now();

UPDATE app.enrollments
SET assignment_state='pending_mfa'
WHERE role='coordinator' AND state='active';

UPDATE app.enrollments
SET activated_at=created_at
WHERE assignment_state='active';

ALTER TABLE app.enrollments
  ADD CONSTRAINT enrollments_coordinator_mfa CHECK (
    (role='coordinator' AND state='active' AND assignment_state='pending_mfa' AND activated_at IS NULL)
    OR (state='active' AND assignment_state='active' AND activated_at IS NOT NULL)
    OR (state<>'active' AND assignment_state='revoked' AND activated_at IS NULL)
  );

-- +goose StatementBegin
CREATE FUNCTION app.require_coordinator_mfa()
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

-- +goose Down

DROP TRIGGER enrollments_require_coordinator_mfa ON app.enrollments;
DROP FUNCTION app.require_coordinator_mfa();
ALTER TABLE app.enrollments DROP CONSTRAINT enrollments_coordinator_mfa;
ALTER TABLE app.enrollments DROP COLUMN activated_at;
ALTER TABLE app.enrollments DROP COLUMN assignment_state;
