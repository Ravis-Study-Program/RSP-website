-- +goose Up

-- A short description of what a custom mock interview round covered, shown as
-- the round's title instead of a generic "Custom round".
ALTER TABLE app.custom_mock_interview_rounds ADD COLUMN description text;

-- +goose Down
ALTER TABLE app.custom_mock_interview_rounds DROP COLUMN description;
