-- +goose Up
-- Jobs is deliberately separate from program activity and profile tables.
CREATE TABLE app.job_companies (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  name text NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 120),
  category text NOT NULL CHECK (category IN ('faang','big_tech','quant','other'))
);
CREATE UNIQUE INDEX job_companies_name_unique ON app.job_companies (lower(trim(name)));

CREATE TABLE app.job_positions (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  company_id uuid NOT NULL REFERENCES app.job_companies(id),
  title text NOT NULL CHECK (length(trim(title)) BETWEEN 1 AND 300),
  job_type text NOT NULL CHECK (job_type IN ('intern','grad','experienced')),
  eligibility text NOT NULL DEFAULT '' CHECK (length(eligibility) <= 4000),
  application_url text NOT NULL CHECK (length(application_url) BETWEEN 1 AND 2048),
  normalized_url text NOT NULL,
  locations text[] NOT NULL CHECK (cardinality(locations) BETWEEN 1 AND 20),
  notes text NOT NULL DEFAULT '' CHECK (length(notes) <= 10000),
  status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','closed')),
  merged_into_id uuid REFERENCES app.job_positions(id),
  created_by_user_id uuid REFERENCES app.users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (merged_into_id IS NULL OR merged_into_id <> id)
);
CREATE UNIQUE INDEX job_positions_url_unique ON app.job_positions (normalized_url) WHERE merged_into_id IS NULL;
CREATE INDEX job_positions_company ON app.job_positions(company_id);

CREATE TABLE app.job_applications (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id uuid NOT NULL REFERENCES app.users(id),
  position_id uuid NOT NULL REFERENCES app.job_positions(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, position_id)
);
CREATE INDEX job_applications_position ON app.job_applications(position_id);

CREATE TABLE app.job_application_stages (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  application_id uuid NOT NULL REFERENCES app.job_applications(id) ON DELETE CASCADE,
  title text NOT NULL CHECK (length(trim(title)) BETWEEN 1 AND 120),
  happened_on date NOT NULL,
  notes text NOT NULL DEFAULT '' CHECK (length(notes) <= 10000),
  initial boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (NOT initial OR title = 'Applied')
);
CREATE UNIQUE INDEX job_stages_one_initial ON app.job_application_stages(application_id) WHERE initial;
CREATE INDEX job_stages_application ON app.job_application_stages(application_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON app.job_companies, app.job_positions,
  app.job_applications, app.job_application_stages TO rsp_app;

-- +goose Down
DROP TABLE app.job_application_stages;
DROP TABLE app.job_applications;
DROP TABLE app.job_positions;
DROP TABLE app.job_companies;
