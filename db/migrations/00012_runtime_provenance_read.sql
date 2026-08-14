-- +goose Up

-- Recommendation quality and public attempt projections distinguish imported
-- attempts from outcome-known native attempts. The API needs read-only access
-- to that narrow provenance relation; mutation rights remain migration-only.
GRANT SELECT ON migration.row_provenance TO rsp_app;

-- +goose Down

REVOKE SELECT ON migration.row_provenance FROM rsp_app;
