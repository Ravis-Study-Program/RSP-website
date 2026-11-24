-- +goose Up
ALTER TABLE app.job_referrals ADD COLUMN matching_enabled boolean NOT NULL DEFAULT false;

CREATE TABLE app.job_referral_profiles (
  user_id uuid PRIMARY KEY REFERENCES app.users(id) ON DELETE CASCADE,
  reason text NOT NULL CHECK (length(trim(reason)) BETWEEN 1 AND 4000),
  resume_url text NOT NULL CHECK (length(resume_url) BETWEEN 1 AND 2048)
);

CREATE TABLE app.job_referral_requests (
  user_id uuid NOT NULL REFERENCES app.job_referral_profiles(user_id) ON DELETE CASCADE,
  position_id uuid NOT NULL REFERENCES app.job_positions(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, position_id)
);
CREATE INDEX job_referral_requests_position ON app.job_referral_requests (position_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON app.job_referral_profiles, app.job_referral_requests TO rsp_app;

-- +goose Down
DROP TABLE app.job_referral_requests, app.job_referral_profiles;
ALTER TABLE app.job_referrals DROP COLUMN matching_enabled;
