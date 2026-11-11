-- +goose Up

-- A student can be invited together with their mentor; accepting the
-- invitation then creates the mentorship.
ALTER TABLE app.season_invitations
  ADD COLUMN mentor_user_id uuid REFERENCES app.users(id) ON DELETE RESTRICT,
  ADD CONSTRAINT season_invitations_mentor_for_students
    CHECK (mentor_user_id IS NULL OR role = 'student');

-- +goose Down
ALTER TABLE app.season_invitations DROP COLUMN mentor_user_id;
