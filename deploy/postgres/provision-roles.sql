\set ON_ERROR_STOP on

SELECT format('CREATE ROLE rsp_migration NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'rsp_migration') \gexec
SELECT format('CREATE ROLE rsp_app NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'rsp_app') \gexec
SELECT format('CREATE ROLE rsp_auth NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'rsp_auth') \gexec

SELECT format('ALTER ROLE rsp_migration LOGIN PASSWORD %L', :'migration_password') \gexec
SELECT format('ALTER ROLE rsp_app LOGIN PASSWORD %L', :'app_password') \gexec
SELECT format('ALTER ROLE rsp_auth LOGIN PASSWORD %L', :'auth_password') \gexec

SELECT format('GRANT CONNECT ON DATABASE %I TO rsp_migration, rsp_app, rsp_auth', current_database()) \gexec
SELECT format('GRANT CREATE ON DATABASE %I TO rsp_migration', current_database()) \gexec
SELECT format('REVOKE CREATE ON DATABASE %I FROM rsp_app, rsp_auth', current_database()) \gexec
GRANT CREATE ON SCHEMA public TO rsp_migration;

SELECT format('CREATE SCHEMA IF NOT EXISTS auth AUTHORIZATION rsp_auth') \gexec
ALTER SCHEMA auth OWNER TO rsp_auth;
