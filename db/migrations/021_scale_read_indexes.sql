-- Phase W read-path indexes.
-- Migration number 021 avoids the pre-existing historical 020 wave.
--
-- These indexes are intentionally limited to the canonical, tenant/branch
-- scoped stock, report, GL, and party-ledger reads.  They do not replace RLS
-- or authorize a broader scope.  The disposable Phase W harness records
-- EXPLAIN (ANALYZE, BUFFERS) evidence after applying this migration.
--
-- Keep this migration ordinary (not CONCURRENTLY): the repository migration
-- runner applies one file at a time and must remain safe to rerun.  Production
-- operators should apply the same definitions with their normal online-index
-- change procedure if a large table requires CREATE INDEX CONCURRENTLY.

CREATE INDEX IF NOT EXISTS idx_stock_batches_availability_020
    ON stock_batches (
        tenant_id, branch_id, item_legacy_id, godown_id,
        locked, expiry_date, received_at, id
    )
    INCLUDE (batch_number, unit_cost);

CREATE INDEX IF NOT EXISTS idx_stock_balances_item_godown_020
    ON stock_balances (
        tenant_id, branch_id, item_legacy_id, godown_id, batch_id
    )
    INCLUDE (on_hand, updated_at);

CREATE INDEX IF NOT EXISTS idx_stock_ledger_scope_time_020
    ON stock_ledger (tenant_id, branch_id, occurred_at DESC, id)
    INCLUDE (batch_id, source_event_id, direction, adjustment_sign, quantity, unit_cost);

CREATE INDEX IF NOT EXISTS idx_sync_events_scope_aggregate_time_020
    ON sync_events (tenant_id, branch_id, aggregate, occurred_at DESC, event_id);

CREATE INDEX IF NOT EXISTS idx_business_documents_report_020
    ON business_documents (
        tenant_id, branch_id, kind, status, occurred_at DESC, id
    )
    INCLUDE (document_number, customer_id, supplier_id, total_amount);

CREATE INDEX IF NOT EXISTS idx_business_document_lines_report_020
    ON business_document_lines (tenant_id, branch_id, document_id, line_number)
    INCLUDE (item_id, item_name, quantity, line_total);

CREATE INDEX IF NOT EXISTS idx_gl_journals_read_020
    ON gl_journals (tenant_id, branch_id, posted_at DESC, id)
    INCLUDE (source_document_id, source_event_id, kind, total_debit, total_credit);

CREATE INDEX IF NOT EXISTS idx_party_ledger_read_020
    ON party_ledger_entries (tenant_id, branch_id, party_id, occurred_at, id)
    INCLUDE (source_document_id, debit_amount, credit_amount, balance_after);

CREATE INDEX IF NOT EXISTS idx_gl_lines_account_read_020
    ON gl_lines (tenant_id, branch_id, account_id, created_at, id)
    INCLUDE (journal_id, party_id, debit_amount, credit_amount);
