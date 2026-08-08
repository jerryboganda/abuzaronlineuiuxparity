-- One-time cleanup of orphaned test-fixture tenants — 2026-08-08 (v2)
--
-- v1 (hand-derived table order) failed 3 times, each on a newly-discovered
-- missing dependency (business_document_revisions, then sync_events'
-- finalized-event guard, then stock_batches -> branches). The dependency
-- graph is too deep (106+ FK relationships across 74 tenant-scoped tables)
-- to hand-order reliably. This version instead deletes from ALL 74
-- tenant-scoped tables (the complete set confirmed via pg_constraint WHERE
-- confrelid='tenants') across multiple passes, catching and ignoring
-- per-table errors within each pass via a PL/pgSQL EXCEPTION block: a
-- table blocked on pass N by a dependency not yet cleared succeeds once
-- that dependency is cleared on a later pass. This converges regardless of
-- the exact topological order. 6 passes is comfortably more than the
-- deepest chain observed (4 levels: tenants -> branches -> stock_batches,
-- for example).
--
-- Same 5 scoped, human-approved (2026-08-08) trigger bypasses as v1 for the
-- append-only/guarded tables that only block DELETE specifically
-- (confirmed via pg_get_triggerdef — the other 6 triggers found on tables
-- in this path are INSERT/UPDATE-only or a DEFERRED constraint trigger and
-- never fire on DELETE, so are left untouched; see v1's header comment,
-- preserved in git history, for the full trigger audit).
--
-- Only targets tenants whose code matches the exact test-fixture naming
-- convention (document-test-*, access-test-*) — confirmed via prior query
-- that neither 'demo' nor 'legacy-reference-sandbox' match this filter.

\set ON_ERROR_STOP on
BEGIN;

CREATE TEMPORARY TABLE orphan_test_tenants AS
SELECT id FROM tenants WHERE code LIKE 'document-test-%' OR code LIKE 'access-test-%';

CREATE TEMPORARY TABLE cleanup_report (step text PRIMARY KEY, affected bigint NOT NULL);
INSERT INTO cleanup_report VALUES ('orphans_identified', (SELECT count(*) FROM orphan_test_tenants));

ALTER TABLE sync_events DISABLE TRIGGER sync_events_final_business_document_delete_017;
ALTER TABLE stock_ledger DISABLE TRIGGER stock_ledger_immutable;
ALTER TABLE gl_journals DISABLE TRIGGER gl_journals_immutable;
ALTER TABLE gl_lines DISABLE TRIGGER gl_lines_immutable;
ALTER TABLE party_ledger_entries DISABLE TRIGGER party_ledger_entries_immutable;

DO $$
DECLARE
    tbl text;
    pass int;
BEGIN
    FOR pass IN 1..6 LOOP
        FOR tbl IN SELECT unnest(ARRAY[
            'audit_events','branches','business_document_lines','business_document_reversals',
            'business_document_revisions','business_document_void_reversals','business_documents',
            'command_receipts','conflict_records','counters','finance_account_categories',
            'finance_accounts','gl_journals','gl_lines','group_allowed_scopes','group_rights',
            'historical_deleted_sale_items','historical_gl_entries','historical_item_changes',
            'historical_party_ledger_adjustments','historical_party_payment_allocations',
            'historical_party_return_allocations','historical_stock_adjustment_lines',
            'historical_stock_snapshots','historical_withholding_tax_entries','inventory_movements',
            'item_supplier_links','item_suppliers','item_tax_assignments','legacy_groups',
            'legacy_id_mappings','master_aliases','master_categories','master_godowns',
            'master_item_associations','master_item_authors','master_item_groups',
            'master_item_images','master_item_models','master_item_notes',
            'master_item_registration_requests','master_items','master_manufacturers',
            'master_parties','master_records','migration_ambiguous_records','migration_exceptions',
            'migration_reconciliation','party_ledger_balances','party_ledger_entries',
            'party_tax_assignments','price_policy_tiers','role_permissions','roles',
            'sales_documents','sessions','shifts','stock_allocations','stock_balance_rebuilds',
            'stock_balances','stock_batches','stock_ledger','sync_cursors','sync_events',
            'tax_rates','tenant_preferences','tenant_sequences','user_branch_assignments',
            'user_counter_assignments','user_memberships','users','voucher_categories',
            'voucher_entries'
        ]) LOOP
            BEGIN
                EXECUTE format('DELETE FROM %I WHERE tenant_id IN (SELECT id FROM orphan_test_tenants)', tbl);
            EXCEPTION WHEN OTHERS THEN
                -- Still blocked on this pass; a later pass will retry once
                -- whatever currently references these rows is cleared.
                NULL;
            END;
        END LOOP;
    END LOOP;
END $$;

ALTER TABLE sync_events ENABLE TRIGGER sync_events_final_business_document_delete_017;
ALTER TABLE stock_ledger ENABLE TRIGGER stock_ledger_immutable;
ALTER TABLE gl_journals ENABLE TRIGGER gl_journals_immutable;
ALTER TABLE gl_lines ENABLE TRIGGER gl_lines_immutable;
ALTER TABLE party_ledger_entries ENABLE TRIGGER party_ledger_entries_immutable;

INSERT INTO cleanup_report
SELECT 'triggers_reenabled', count(*) FROM pg_trigger
 WHERE tgname IN ('sync_events_final_business_document_delete_017','stock_ledger_immutable',
                   'gl_journals_immutable','gl_lines_immutable','party_ledger_entries_immutable')
   AND tgenabled = 'O';

-- If any tenant still has undeleted children after 6 passes, this final
-- DELETE will fail loudly (ON_ERROR_STOP) rather than silently leaving a
-- partially-cleaned tenant — that's a real remaining gap to investigate,
-- not something to paper over.
DELETE FROM tenants WHERE id IN (SELECT id FROM orphan_test_tenants);

INSERT INTO cleanup_report
SELECT 'remaining_orphans', count(*) FROM tenants
 WHERE code LIKE 'document-test-%' OR code LIKE 'access-test-%';

SELECT step, affected FROM cleanup_report ORDER BY step;

COMMIT;
