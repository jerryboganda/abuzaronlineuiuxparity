-- Branch RLS hardening.
--
-- The earlier tenant and branch policies are intentionally left in place for
-- compatibility. They are permissive policies, so PostgreSQL combines them
-- with OR. A restrictive policy is therefore required to make the tenant and
-- branch predicates apply together.
--
-- Authentication/bootstrap queries retain their existing app.authenticating
-- escape hatch. Normal API transactions set it to false before application
-- data access. Tenant-admin scope remains tenant-wide, but never
-- cross-tenant.

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        -- 001/002 operational tables
        'counters', 'tenant_sequences', 'shifts', 'audit_events',
        'sync_events', 'sync_cursors', 'conflict_records',
        'sales_documents', 'inventory_movements',
        -- 011 business documents and command receipts
        'business_documents', 'business_document_lines',
        'business_document_revisions', 'command_receipts',
        -- 012 stock/batch projections
        'stock_batches', 'stock_ledger', 'stock_allocations',
        'stock_balances', 'stock_balance_rebuilds',
        -- 013 finance and party-ledger projections (014 extends these)
        'gl_journals', 'gl_lines', 'party_ledger_entries',
        'party_ledger_balances', 'voucher_entries'
    ] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', table_name);
        EXECUTE format(
            'DROP POLICY IF EXISTS %I ON %I',
            table_name || '_branch_tenant_hardening', table_name
        );
        EXECUTE format(
            'CREATE POLICY %I ON %I AS RESTRICTIVE FOR ALL
             USING (
                 current_setting(''app.authenticating'', true) = ''true''
                 OR (
                     tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid
                     AND (
                         NULLIF(current_setting(''app.allow_tenant_scope'', true), '''') = ''true''
                         OR %s
                     )
                 )
             )
             WITH CHECK (
                 current_setting(''app.authenticating'', true) = ''true''
                 OR (
                     tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid
                     AND (
                         NULLIF(current_setting(''app.allow_tenant_scope'', true), '''') = ''true''
                         OR %s
                     )
                 )
             )',
            table_name || '_branch_tenant_hardening',
            table_name,
            CASE
                WHEN table_name = 'audit_events'
                    THEN 'branch_id IS NULL OR branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid'
                ELSE 'branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid'
            END,
            CASE
                WHEN table_name = 'audit_events'
                    THEN 'branch_id IS NULL OR branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid'
                ELSE 'branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid'
            END
        );
    END LOOP;
END $$;
