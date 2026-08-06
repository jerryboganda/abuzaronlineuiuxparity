-- Run as the protected schema owner after migrations:
--   psql "$ABUZAR_ADMIN_DATABASE_URL" -v app_role="$ABUZAR_APP_ROLE" -f grant-app-role.sql
-- RLS remains the tenant boundary; this grant deliberately gives the API no
-- database DDL or role-management privileges.
\if :{?app_role}
\else
\echo 'app_role is required (pass -v app_role=...)'
\quit
\endif

-- psql expands variables in SQL statements (not inside dollar-quoted blocks),
-- so generate identifier-quoted statements with format() and execute them.
SELECT format($sql$
DO $$ BEGIN
    IF btrim(%L) = '' OR NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = %L) THEN
        RAISE EXCEPTION 'application role does not exist: %%', %L;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM pg_roles
        WHERE rolname = %L
          AND (rolsuper OR rolbypassrls OR rolcreatedb OR rolcreaterole)
    ) THEN
        RAISE EXCEPTION 'application role has a privileged attribute: %%', %L;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        JOIN pg_roles r ON r.oid = c.relowner
        WHERE n.nspname = 'public'
          AND c.relkind IN ('r', 'p')
          AND r.rolname = %L
    ) THEN
        RAISE EXCEPTION 'application role owns a protected table: %%', %L;
    END IF;
    IF has_schema_privilege(%L, 'public', 'CREATE') THEN
        RAISE EXCEPTION 'application role has CREATE on public schema: %%', %L;
    END IF;
END $$;
$sql$, :'app_role', :'app_role', :'app_role', :'app_role', :'app_role', :'app_role', :'app_role', :'app_role', :'app_role') \gexec
SELECT format('GRANT USAGE ON SCHEMA public TO %I', :'app_role') \gexec
SELECT format('REVOKE ALL ON ALL TABLES IN SCHEMA public FROM %I', :'app_role') \gexec
SELECT format('REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM %I', :'app_role') \gexec
SELECT format('GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO %I', :'app_role') \gexec
SELECT format('GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO %I', :'app_role') \gexec
-- sync_events is the immutable event envelope. The application role must not
-- delete it, even while the owner-side 017 delete guard is being rolled out.
SELECT format('REVOKE DELETE ON TABLE public.sync_events FROM %I', :'app_role') \gexec
SELECT format('ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %I', :'app_role') \gexec
SELECT format('ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO %I', :'app_role') \gexec
