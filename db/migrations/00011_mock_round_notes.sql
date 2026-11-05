-- +goose Up

-- Interviewer notes for each mock interview round. Custom rounds already keep
-- theirs in custom_mock_interview_rounds.content_html, but behavioural and
-- LeetCode rounds had nowhere to store them, so notes written for those rounds
-- were silently discarded. Legacy data had no such notes, so nothing is backfilled.
ALTER TABLE app.behavioural_mock_interview_rounds ADD COLUMN content_html text;
ALTER TABLE app.leetcode_mock_interview_rounds ADD COLUMN content_html text;

-- +goose Down
ALTER TABLE app.leetcode_mock_interview_rounds DROP COLUMN content_html;
ALTER TABLE app.behavioural_mock_interview_rounds DROP COLUMN content_html;
