-- +goose Up

-- A mentor can set a mark back to "Not marked", which removes it.
GRANT DELETE ON app.monday_update_marks, app.monday_week_marks TO rsp_app;

-- +goose Down
REVOKE DELETE ON app.monday_update_marks, app.monday_week_marks FROM rsp_app;
