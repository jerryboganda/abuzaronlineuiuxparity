-- Disposable Phase W fixture. Run only against a newly-created benchmark DB.
--
-- psql variables:
--   scale_stock (default 25000), scale_gl (default 10000), scale_items
--   (default 3000), scale_batches (default 5000), batch_size (default 50000)
--
-- The full-volume invocation uses scale_stock=3231846, scale_gl=1040590,
-- scale_items=30050, and scale_batches=30050. It is intentionally opt-in:
-- several million rows plus indexes can require substantial disk and time.
--
-- ROOT CAUSE OF THE 2026-08-07 CRASH (see tmp/phase-w-blocker-20260807-014759.json
-- and the "Disposable volume fixture" section of README.md):
--
-- The previous version of this script generated every table's full row count
-- with a single set-based `INSERT ... SELECT ... FROM generate_series(1, N)`
-- and never committed until one final COMMIT at the very end. For the
-- full-volume run that meant ~3.2M stock-ledger rows, ~3.2M matching
-- sync_events rows, and ~1.04M GL journals/lines/party-ledger rows (about
-- 7.5M rows total) were all generated and held open inside ONE Postgres
-- transaction:
--   * All of that transaction's WAL stays unflushed-to-durable-commit and the
--     backend has to hold onto every buffer/lock touched by the transaction
--     until the single COMMIT at the very end -- there is no intermediate
--     checkpoint-friendly boundary the server can rely on to make progress
--     durable and release resources.
--   * gl_journals/gl_lines carry `DEFERRABLE INITIALLY DEFERRED` constraint
--     triggers (assert_balanced_gl_journal, see db/migrations/013_finance_ledgers.sql)
--     that validate journal balance. Because the whole GL section ran inside
--     one transaction, every one of the ~3.1M deferred trigger firings
--     (1,040,590 journals + 2,081,180 lines) fired in a single burst at the
--     final COMMIT instead of being spread out.
--   * The read-path indexes added by db/migrations/021_scale_read_indexes.sql
--     were maintained row-by-row for all ~7.5M inserted rows instead of being
--     built once in bulk after loading.
--   * There was no way to resume: any failure meant the entire multi-million
--     row transaction rolled back and the next attempt started from zero.
-- On the shared local Postgres instance this combination of a huge
-- long-running single transaction, a large deferred-trigger burst, and
-- continuous index maintenance was enough to crash the backend and force
-- cluster-wide crash recovery (see README.md).
--
-- FIX APPLIED HERE:
--   1. Large tables (sync_events/stock_ledger, and the business_documents /
--      business_document_lines / sync_events / gl_journals / gl_lines /
--      party_ledger_entries bundle) are now loaded through PL/pgSQL
--      PROCEDUREs that loop over `batch_size`-row windows and COMMIT after
--      each window (procedures may control transactions when CALLed as a
--      top-level statement -- see PostgreSQL 11+ CALL/COMMIT-in-procedure
--      support). This bounds WAL/lock/temp usage per window, spreads the
--      deferred GL trigger firings across many small commits instead of one
--      giant burst, and lets checkpoints happen between windows.
--   2. Each GL window inserts business_documents, business_document_lines,
--      sync_events, gl_journals, and BOTH gl_lines sides (debit + credit)
--      for that window before committing, so the deferred balance-check
--      trigger always sees a balanced journal at commit time.
--   3. Progress is recorded in a small `phase_w_seed_progress` bookkeeping
--      table, updated in the same transaction as each committed window. If a
--      run is interrupted, rerunning this script against the same
--      (retained) disposable database resumes from the last committed
--      window instead of restarting from zero.
--   4. The non-unique read-path indexes from migration 021 (they carry no
--      FK/PK/UNIQUE obligation) are dropped before the bulk load and
--      recreated with the exact same definitions afterward, so 7.5M rows of
--      index maintenance happen once in bulk instead of incrementally on
--      every insert.
--   5. RAISE NOTICE progress lines are emitted after every committed window
--      so a human running run-phase-w.ps1 sees rows-committed-so-far instead
--      of a silently hanging psql process.
--
-- This script still requires a genuinely disposable/isolated Postgres
-- instance for -FullVolume: batching removes the single-giant-transaction
-- failure mode, but 7.5M+ rows of WAL/disk/CPU work is still substantial and
-- should not run unattended against a shared cluster used by other work.

\set ON_ERROR_STOP on
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET synchronous_commit = off;
SET maintenance_work_mem = '1GB';
SET work_mem = '64MB';
\if :{?scale_stock}
\else
  \set scale_stock 25000
\endif
\if :{?scale_gl}
\else
  \set scale_gl 10000
\endif
\if :{?scale_items}
\else
  \set scale_items 3000
\endif
\if :{?scale_batches}
\else
  \set scale_batches 5000
\endif
\if :{?batch_size}
\else
  \set batch_size 50000
\endif
\if :{?tenant_id}
\else
  \set tenant_id '90000000-0000-0000-0000-000000000001'
\endif
\if :{?branch_id}
\else
  \set branch_id '90000000-0000-0000-0000-000000000101'
\endif
\if :{?godown_id}
\else
  \set godown_id '90000000-0000-0000-0000-000000000201'
\endif
\if :{?party_id}
\else
  \set party_id '90000000-0000-0000-0000-000000000301'
\endif

-- Resumability bookkeeping. Disposable-DB-only helper table; not part of the
-- application schema, holds no tenant data, never read by the application.
CREATE TABLE IF NOT EXISTS phase_w_seed_progress (
    stage text PRIMARY KEY,
    last_n integer NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now()
);

\echo Phase W fixture: tenant and master data

-- A fixed fixture identity makes the harness repeatable while the database
-- itself remains disposable. No application or legacy credentials are used.
INSERT INTO tenants (id, legal_name, code)
VALUES (:'tenant_id'::uuid, 'Phase W disposable benchmark', 'phase-w')
ON CONFLICT DO NOTHING;

INSERT INTO branches (id, tenant_id, code, name)
VALUES (:'branch_id'::uuid, :'tenant_id'::uuid, 'MAIN', 'Phase W Main')
ON CONFLICT DO NOTHING;

INSERT INTO counters (id, tenant_id, branch_id, code, name)
VALUES (
    '90000000-0000-0000-0000-000000000111'::uuid,
    :'tenant_id'::uuid, :'branch_id'::uuid, 'POS-1', 'Phase W POS'
)
ON CONFLICT DO NOTHING;

INSERT INTO users (id, tenant_id, username, display_name, password_hash)
VALUES (
    '90000000-0000-0000-0000-000000000121'::uuid,
    :'tenant_id'::uuid, 'phase-w-fixture', 'Phase W Fixture',
    'phase-w-fixture-hash'
)
ON CONFLICT DO NOTHING;

INSERT INTO master_godowns (id, tenant_id, legacy_id, code, name)
VALUES (:'godown_id'::uuid, :'tenant_id'::uuid, 'GODOWN1', 'GODOWN1', 'Phase W Godown')
ON CONFLICT DO NOTHING;

INSERT INTO master_parties (id, tenant_id, party_type, legacy_id, code, name)
VALUES (:'party_id'::uuid, :'tenant_id'::uuid, 'customer', 'CUSTOMER1', 'CUSTOMER1', 'Phase W Customer')
ON CONFLICT DO NOTHING;

-- master_items and stock_batches are bounded by scale_items/scale_batches
-- (<=30,050 even at full volume), three orders of magnitude smaller than the
-- ledger/GL tables. A single set-based statement for each is not the crash
-- risk and is left unbatched for simplicity.
INSERT INTO master_items (id, tenant_id, legacy_id, code, name, payload)
SELECT
    md5(:'tenant_id' || ':item:' || n)::uuid,
    :'tenant_id'::uuid,
    'ITEM-' || lpad(n::text, 6, '0'),
    'ITEM-' || lpad(n::text, 6, '0'),
    'Phase W Item ' || n,
    jsonb_build_object('benchmark', true, 'ordinal', n)
FROM generate_series(1, :scale_items::integer) AS items(n)
ON CONFLICT DO NOTHING;

-- Batch identity is deterministic. The first batch is the line-add proxy's
-- lookup target and all batches belong to the explicit benchmark godown.
\echo Phase W fixture: stock batches
INSERT INTO stock_batches (
    id, tenant_id, branch_id, item_id, item_legacy_id, godown_id,
    batch_number, expiry_date, unit_cost, received_at
)
SELECT
    md5(:'tenant_id' || ':batch:' || n)::uuid,
    :'tenant_id'::uuid,
    :'branch_id'::uuid,
    md5(:'tenant_id' || ':item:' || (((n - 1) % :scale_items::integer) + 1))::uuid,
    'ITEM-' || lpad((((n - 1) % :scale_items::integer) + 1)::text, 6, '0'),
    :'godown_id'::uuid,
    'BATCH-' || lpad(n::text, 8, '0'),
    CURRENT_DATE + ((n % 720) + 30),
    (10 + (n % 100))::numeric(19,4),
    now() - ((n % 365) || ' days')::interval
FROM generate_series(1, :scale_batches::integer) AS batches(n)
ON CONFLICT DO NOTHING;

-- Drop the migration-021 read-path indexes for the duration of the bulk
-- load. None of these back a PK/UNIQUE/FK obligation (verified against
-- db/migrations/021_scale_read_indexes.sql), so dropping them is safe and
-- avoids ~7.5M rows of incremental index maintenance. They are recreated
-- with identical definitions after the load, before ANALYZE.
\echo Phase W fixture: dropping read-path indexes for bulk load
DROP INDEX IF EXISTS idx_stock_ledger_scope_time_020;
DROP INDEX IF EXISTS idx_sync_events_scope_aggregate_time_020;
DROP INDEX IF EXISTS idx_business_documents_report_020;
DROP INDEX IF EXISTS idx_business_document_lines_report_020;
DROP INDEX IF EXISTS idx_gl_journals_read_020;
DROP INDEX IF EXISTS idx_party_ledger_read_020;
DROP INDEX IF EXISTS idx_gl_lines_account_read_020;

-- Batched, resumable stock-ledger load. Each window inserts the matching
-- sync_events rows and then the stock_ledger rows for that window in the
-- SAME transaction (source_event_id FK is satisfied by self-visibility of
-- the just-inserted, not-yet-committed sync_events rows) before COMMITting
-- and recording progress.
CREATE OR REPLACE PROCEDURE phase_w_seed_stock(
    p_tenant_id uuid, p_branch_id uuid, p_total integer, p_batches integer,
    p_batch_size integer
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_last integer;
    v_start integer;
    v_end integer;
BEGIN
    SELECT last_n INTO v_last FROM phase_w_seed_progress WHERE stage = 'stock';
    IF NOT FOUND THEN
        v_last := 0;
    END IF;
    v_start := v_last + 1;
    WHILE v_start <= p_total LOOP
        v_end := LEAST(v_start + p_batch_size - 1, p_total);

        INSERT INTO sync_events (
            event_id, tenant_id, branch_id, aggregate, aggregate_id,
            idempotency_key, payload, occurred_at
        )
        SELECT
            md5(p_tenant_id || ':stock-event:' || n)::uuid,
            p_tenant_id,
            p_branch_id,
            'phase_w_stock',
            md5(p_tenant_id || ':stock-event-aggregate:' || n)::uuid,
            'phase-w-stock-' || n,
            jsonb_build_object('status', 'posted', 'benchmark', true),
            now() - ((n % 365) || ' days')::interval
        FROM generate_series(v_start, v_end) AS rows(n)
        ON CONFLICT DO NOTHING;

        INSERT INTO stock_ledger (
            id, tenant_id, branch_id, batch_id, source_event_id, source_line_key,
            direction, adjustment_sign, quantity, unit_cost, occurred_at
        )
        SELECT
            md5(p_tenant_id || ':stock-ledger:' || n)::uuid,
            p_tenant_id,
            p_branch_id,
            md5(p_tenant_id || ':batch:' || (((n - 1) % p_batches) + 1))::uuid,
            md5(p_tenant_id || ':stock-event:' || n)::uuid,
            'phase-w-line-' || n,
            CASE WHEN n % 5 = 0 THEN 'out' ELSE 'in' END,
            1,
            (1 + (n % 7))::numeric(19,4),
            (10 + (n % 100))::numeric(19,4),
            now() - ((n % 365) || ' days')::interval
        FROM generate_series(v_start, v_end) AS rows(n)
        ON CONFLICT DO NOTHING;

        INSERT INTO phase_w_seed_progress (stage, last_n)
        VALUES ('stock', v_end)
        ON CONFLICT (stage) DO UPDATE
        SET last_n = EXCLUDED.last_n, updated_at = now();

        RAISE NOTICE 'Phase W stock seed: committed % / % rows (sync_events + stock_ledger)', v_end, p_total;
        COMMIT;

        v_start := v_end + 1;
    END LOOP;
END;
$$;

\echo Phase W fixture: stock source events and ledger (batched)
CALL phase_w_seed_stock(:'tenant_id'::uuid, :'branch_id'::uuid, :scale_stock::integer, :scale_batches::integer, :batch_size::integer);

-- stock_balances is a rebuildable cache keyed by (tenant, branch, batch); its
-- output cardinality is bounded by scale_batches (<=30,050), not scale_stock,
-- so a single aggregate statement over the now-fully-committed stock_ledger
-- is safe (small hash-aggregate, bounded groups) even though it scans every
-- stock_ledger row.
\echo Phase W fixture: stock balances
INSERT INTO stock_balances (
    tenant_id, branch_id, batch_id, item_id, item_legacy_id, godown_id, on_hand
)
SELECT
    b.tenant_id, b.branch_id, b.id, b.item_id, b.item_legacy_id, b.godown_id,
    COALESCE(SUM(CASE WHEN l.direction = 'out' THEN -l.quantity ELSE l.quantity END), 0)
FROM stock_batches b
LEFT JOIN stock_ledger l
  ON l.tenant_id = b.tenant_id AND l.branch_id = b.branch_id AND l.batch_id = b.id
WHERE b.tenant_id = :'tenant_id'::uuid AND b.branch_id = :'branch_id'::uuid
GROUP BY b.tenant_id, b.branch_id, b.id, b.item_id, b.item_legacy_id, b.godown_id
HAVING COALESCE(SUM(CASE WHEN l.direction = 'out' THEN -l.quantity ELSE l.quantity END), 0) >= 0
ON CONFLICT (tenant_id, branch_id, batch_id) DO UPDATE
SET on_hand = EXCLUDED.on_hand, updated_at = now();

-- Batched, resumable canonical-documents + GL + party-ledger load. Each
-- window inserts business_documents, business_document_lines, sync_events,
-- gl_journals, and BOTH gl_lines sides (debit + credit) for that window
-- before COMMITting, so gl_journals/gl_lines' DEFERRABLE INITIALLY DEFERRED
-- balance-check trigger (assert_balanced_gl_journal) always observes a
-- balanced journal at commit time -- never a debit-only or credit-only
-- partial state.
CREATE OR REPLACE PROCEDURE phase_w_seed_gl(
    p_tenant_id uuid, p_branch_id uuid, p_counter_id uuid, p_operator_id uuid,
    p_party_id uuid, p_items integer, p_total integer, p_batch_size integer
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_last integer;
    v_start integer;
    v_end integer;
BEGIN
    SELECT last_n INTO v_last FROM phase_w_seed_progress WHERE stage = 'gl';
    IF NOT FOUND THEN
        v_last := 0;
    END IF;
    v_start := v_last + 1;
    WHILE v_start <= p_total LOOP
        v_end := LEAST(v_start + p_batch_size - 1, p_total);

        INSERT INTO business_documents (
            id, tenant_id, branch_id, counter_id, operator_id, kind, document_number,
            status, occurred_at, customer_id, subtotal, total_amount, paid_amount,
            balance_amount
        )
        SELECT
            md5(p_tenant_id || ':document:' || n)::uuid,
            p_tenant_id,
            p_branch_id,
            p_counter_id,
            p_operator_id,
            'credit-sale',
            'PHASE-W-' || lpad(n::text, 10, '0'),
            'posted',
            now() - ((n % 365) || ' days')::interval,
            p_party_id,
            100.00,
            100.00,
            0,
            100.00
        FROM generate_series(v_start, v_end) AS docs(n)
        ON CONFLICT DO NOTHING;

        INSERT INTO business_document_lines (
            id, tenant_id, branch_id, document_id, line_number, item_id,
            item_legacy_id, item_code, item_name, quantity, unit_price,
            line_gross, line_total
        )
        SELECT
            md5(p_tenant_id || ':document-line:' || n)::uuid,
            p_tenant_id,
            p_branch_id,
            md5(p_tenant_id || ':document:' || n)::uuid,
            1,
            md5(p_tenant_id || ':item:' || (((n - 1) % p_items) + 1))::uuid,
            'ITEM-' || lpad((((n - 1) % p_items) + 1)::text, 6, '0'),
            'ITEM-' || lpad((((n - 1) % p_items) + 1)::text, 6, '0'),
            'Phase W Item ' || (((n - 1) % p_items) + 1),
            1,
            100,
            100,
            100
        FROM generate_series(v_start, v_end) AS docs(n)
        ON CONFLICT DO NOTHING;

        INSERT INTO sync_events (
            event_id, tenant_id, branch_id, aggregate, aggregate_id,
            idempotency_key, payload, occurred_at
        )
        SELECT
            md5(p_tenant_id || ':document-event:' || n)::uuid,
            p_tenant_id,
            p_branch_id,
            'phase_w_sale',
            md5(p_tenant_id || ':document:' || n)::uuid,
            'phase-w-document-' || n,
            jsonb_build_object('status', 'posted', 'benchmark', true),
            now() - ((n % 365) || ' days')::interval
        FROM generate_series(v_start, v_end) AS docs(n)
        ON CONFLICT DO NOTHING;

        INSERT INTO gl_journals (
            id, tenant_id, branch_id, source_event_id, source_document_id, kind,
            posted_at, total_debit, total_credit
        )
        SELECT
            md5(p_tenant_id || ':journal:' || n)::uuid,
            p_tenant_id,
            p_branch_id,
            md5(p_tenant_id || ':document-event:' || n)::uuid,
            md5(p_tenant_id || ':document:' || n)::uuid,
            'credit-sale',
            now() - ((n % 365) || ' days')::interval,
            100,
            100
        FROM generate_series(v_start, v_end) AS docs(n)
        ON CONFLICT DO NOTHING;

        INSERT INTO gl_lines (
            id, tenant_id, branch_id, journal_id, line_number, account_id,
            party_id, debit_amount, credit_amount, memo
        )
        SELECT
            md5(p_tenant_id || ':gl-line-cash:' || n)::uuid,
            p_tenant_id, p_branch_id,
            md5(p_tenant_id || ':journal:' || n)::uuid, 1, cash.id,
            p_party_id, 100, 0, 'Phase W debit'
        FROM generate_series(v_start, v_end) AS docs(n)
        CROSS JOIN LATERAL (
            SELECT id FROM finance_accounts
            WHERE tenant_id = p_tenant_id AND system_key = 'accounts_receivable'
        ) cash
        ON CONFLICT DO NOTHING;

        INSERT INTO gl_lines (
            id, tenant_id, branch_id, journal_id, line_number, account_id,
            debit_amount, credit_amount, memo
        )
        SELECT
            md5(p_tenant_id || ':gl-line-revenue:' || n)::uuid,
            p_tenant_id, p_branch_id,
            md5(p_tenant_id || ':journal:' || n)::uuid, 2, revenue.id,
            0, 100, 'Phase W credit'
        FROM generate_series(v_start, v_end) AS docs(n)
        CROSS JOIN LATERAL (
            SELECT id FROM finance_accounts
            WHERE tenant_id = p_tenant_id AND system_key = 'sales_revenue'
        ) revenue
        ON CONFLICT DO NOTHING;

        INSERT INTO party_ledger_entries (
            id, tenant_id, branch_id, party_id, counterparty_kind, source_event_id,
            source_document_id, entry_kind, debit_amount, credit_amount,
            balance_after, occurred_at, description
        )
        SELECT
            md5(p_tenant_id || ':party-ledger:' || n)::uuid,
            p_tenant_id, p_branch_id, p_party_id, 'customer',
            md5(p_tenant_id || ':document-event:' || n)::uuid,
            md5(p_tenant_id || ':document:' || n)::uuid, 'sale', 100, 0,
            (n * 100)::numeric(19,4),
            now() - ((n % 365) || ' days')::interval,
            'Phase W customer ledger'
        FROM generate_series(v_start, v_end) AS docs(n)
        ON CONFLICT DO NOTHING;

        INSERT INTO phase_w_seed_progress (stage, last_n)
        VALUES ('gl', v_end)
        ON CONFLICT (stage) DO UPDATE
        SET last_n = EXCLUDED.last_n, updated_at = now();

        RAISE NOTICE 'Phase W GL seed: committed % / % rows (documents + lines + events + journals + gl_lines + party ledger)', v_end, p_total;
        COMMIT;

        v_start := v_end + 1;
    END LOOP;
END;
$$;

\echo Phase W fixture: canonical documents, GL journals/lines, and party ledger (batched)
CALL phase_w_seed_gl(
    :'tenant_id'::uuid, :'branch_id'::uuid,
    '90000000-0000-0000-0000-000000000111'::uuid,
    '90000000-0000-0000-0000-000000000121'::uuid,
    :'party_id'::uuid, :scale_items::integer, :scale_gl::integer, :batch_size::integer
);

INSERT INTO party_ledger_balances (
    tenant_id, branch_id, party_id, debit_total, credit_total, balance
)
VALUES (:'tenant_id'::uuid, :'branch_id'::uuid, :'party_id'::uuid,
        (:scale_gl::numeric * 100), 0, (:scale_gl::numeric * 100))
ON CONFLICT (tenant_id, branch_id, party_id) DO UPDATE
SET debit_total = EXCLUDED.debit_total,
    credit_total = EXCLUDED.credit_total,
    balance = EXCLUDED.balance,
    updated_at = now();

-- Recreate the read-path indexes dropped before the bulk load, with the
-- exact same definitions as db/migrations/021_scale_read_indexes.sql, then
-- ANALYZE everything the performance probes read.
\echo Phase W fixture: rebuilding read-path indexes
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

ANALYZE stock_batches;
ANALYZE stock_ledger;
ANALYZE stock_balances;
ANALYZE business_documents;
ANALYZE business_document_lines;
ANALYZE sync_events;
ANALYZE gl_journals;
ANALYZE gl_lines;
ANALYZE party_ledger_entries;

\echo Phase W fixture: seed complete
