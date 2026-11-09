-- +goose Up

-- An invitation can name the username (profile slug) the person should get.
-- It is applied when they accept with an account created for the invitation.
ALTER TABLE app.season_invitations
  ADD COLUMN profile_slug text,
  ADD CONSTRAINT season_invitations_profile_slug_format
    CHECK (profile_slug IS NULL OR (length(profile_slug) BETWEEN 3 AND 50 AND profile_slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'));

-- +goose Down
ALTER TABLE app.season_invitations DROP COLUMN profile_slug;
