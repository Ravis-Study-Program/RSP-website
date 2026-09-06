-- +goose Up
CREATE TABLE app.season_invitations (
  id uuid PRIMARY KEY,
  season_id uuid NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  name text NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 100),
  email text NOT NULL CHECK (email=lower(trim(email)) AND length(email)<=254),
  role app.season_role NOT NULL,
  token_hash text NOT NULL UNIQUE,
  delivery_id uuid NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  sent_at timestamptz,
  accepted_at timestamptz,
  accepted_by uuid REFERENCES app.users(id) ON DELETE RESTRICT,
  cancelled_at timestamptz,
  CHECK (accepted_at IS NULL OR cancelled_at IS NULL),
  CHECK ((accepted_at IS NULL) = (accepted_by IS NULL))
);
CREATE UNIQUE INDEX season_invitations_pending ON app.season_invitations(season_id,email)
  WHERE accepted_at IS NULL AND cancelled_at IS NULL;
GRANT SELECT,INSERT,UPDATE ON app.season_invitations TO rsp_app;

-- +goose Down
DROP TABLE app.season_invitations;
