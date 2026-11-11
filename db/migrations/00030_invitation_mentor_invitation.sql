-- +goose Up
-- A student invited together with a mentor who is invited in the same import
-- names that mentor's invitation: the mentorship is made once both accept.
ALTER TABLE app.season_invitations
  ADD COLUMN mentor_invitation_id uuid REFERENCES app.season_invitations(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE app.season_invitations DROP COLUMN mentor_invitation_id;
