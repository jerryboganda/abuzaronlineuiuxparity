-- Branch-sensitive policies are layered on top of tenant scope for operational tables.
-- A tenant-wide operator may set app.branch_id to an empty value and use API-approved tenant views.

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'counters', 'tenant_sequences', 'shifts', 'audit_events', 'sync_events',
        'sync_cursors', 'conflict_records', 'sales_documents', 'inventory_movements'
    ] LOOP
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', table_name || '_branch_scope', table_name);
        EXECUTE format(
            'CREATE POLICY %I ON %I USING (current_setting(''app.authenticating'', true) = ''true'' OR branch_id IS NULL OR branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid OR NULLIF(current_setting(''app.allow_tenant_scope'', true), '''') = ''true'') WITH CHECK (current_setting(''app.authenticating'', true) = ''true'' OR branch_id IS NULL OR branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid OR NULLIF(current_setting(''app.allow_tenant_scope'', true), '''') = ''true'')',
            table_name || '_branch_scope', table_name
        );
    END LOOP;
END $$;
