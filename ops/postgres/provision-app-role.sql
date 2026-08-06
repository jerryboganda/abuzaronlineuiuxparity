-- Idempotently create a login role for the API, then delegate table DML to
-- grant-app-role.sql. Run this only with ABUZAR_ADMIN_DATABASE_URL.
-- ABUZAR_APP_ROLE_PASSWORD is optional for local trust-authenticated clusters;
-- when supplied it is read from the process environment and never printed.
\set ON_ERROR_STOP on
\if :{?app_role}
\else
\echo 'app_role is required (pass -v app_role=...)'
\quit
\endif

SELECT format('CREATE ROLE %I LOGIN NOINHERIT NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE', :'app_role')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'app_role') \gexec

SELECT format('ALTER ROLE %I WITH LOGIN NOINHERIT NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION', :'app_role') \gexec

\getenv app_role_password ABUZAR_APP_ROLE_PASSWORD
\if :{?app_role_password}
SELECT format('ALTER ROLE %I PASSWORD %L', :'app_role', :'app_role_password')
WHERE length(:'app_role_password') > 0 \gexec
\unset app_role_password
\endif

-- The grant script revokes stale table/sequence privileges before applying
-- the narrow DML/sequence set and also rejects privileged role attributes.
\ir grant-app-role.sql
