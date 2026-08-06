-- Migration bookkeeping is tenant data too. Keep importer and reconciliation
-- records inside the same tenant boundary as the business tables.
DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'legacy_id_mappings', 'migration_exceptions', 'migration_reconciliation'
    ] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', table_name);
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', table_name || '_tenant_scope', table_name);
        EXECUTE format(
            'CREATE POLICY %I ON %I USING (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid OR current_setting(''app.authenticating'', true) = ''true'') WITH CHECK (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid OR current_setting(''app.authenticating'', true) = ''true'')',
            table_name || '_tenant_scope', table_name
        );
    END LOOP;
END $$;
