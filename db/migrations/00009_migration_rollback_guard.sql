-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app.reject_immutable_change()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
  IF TG_OP='DELETE'
     AND current_user='rsp_migration'
     AND current_setting('rsp.migration_rollback',true)='on'
     AND TG_TABLE_NAME IN ('enrollment_removal_events','mock_interview_versions') THEN
    RETURN OLD;
  END IF;
  RAISE EXCEPTION '% is append-only', TG_TABLE_NAME
    USING ERRCODE = '55000';
END
$function$;
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app.reject_immutable_change()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
  RAISE EXCEPTION '% is append-only', TG_TABLE_NAME
    USING ERRCODE = '55000';
END
$function$;
-- +goose StatementEnd
