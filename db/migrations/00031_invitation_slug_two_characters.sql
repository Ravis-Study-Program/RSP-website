-- +goose Up
-- Usernames may be two characters ("mm").
ALTER TABLE app.season_invitations
  DROP CONSTRAINT season_invitations_profile_slug_format,
  ADD CONSTRAINT season_invitations_profile_slug_format
    CHECK (profile_slug IS NULL OR (length(profile_slug) BETWEEN 2 AND 50 AND profile_slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'));

-- +goose Down
ALTER TABLE app.season_invitations
  DROP CONSTRAINT season_invitations_profile_slug_format,
  ADD CONSTRAINT season_invitations_profile_slug_format
    CHECK (profile_slug IS NULL OR (length(profile_slug) BETWEEN 3 AND 50 AND profile_slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'));
