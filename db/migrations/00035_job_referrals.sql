-- +goose Up
CREATE TABLE app.job_referrals (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  company_id uuid NOT NULL REFERENCES app.job_companies(id),
  notes text NOT NULL DEFAULT '' CHECK (length(notes) <= 10000),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, company_id)
);
CREATE INDEX job_referrals_company ON app.job_referrals(company_id);
GRANT SELECT, INSERT, UPDATE, DELETE ON app.job_referrals TO rsp_app;

-- +goose Down
DROP TABLE app.job_referrals;
