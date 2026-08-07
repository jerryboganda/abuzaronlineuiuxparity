-- Disposable Phase W fixture. Run only against a newly-created benchmark DB.
--
-- psql variables:
--   scale_stock (default 25000), scale_gl (default 10000), scale_items
--   (default 3000), scale_batches (default 5000)
--
-- The full-volume invocation uses scale_stock=3231846, scale_gl=1040590,
-- scale_items=30050, and scale_batches=30050. It is intentionally opt-in:
-- several million rows plus indexes can require substantial disk and time.

\set ON_ERROR_STOP on
SET statement_timeout = 0;
SET lock_timeout = 0;
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

BEGIN;
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

-- Each stock row has a source event because the canonical movement report
-- joins the immutable event envelope to exclude draft/void compatibility data.
\echo Phase W fixture: stock source events and ledger
INSERT INTO sync_events (
    event_id, tenant_id, branch_id, aggregate, aggregate_id,
    idempotency_key, payload, occurred_at
)
SELECT
    md5(:'tenant_id' || ':stock-event:' || n)::uuid,
    :'tenant_id'::uuid,
    :'branch_id'::uuid,
    'phase_w_stock',
    md5(:'tenant_id' || ':stock-event-aggregate:' || n)::uuid,
    'phase-w-stock-' || n,
    jsonb_build_object('status', 'posted', 'benchmark', true),
    now() - ((n % 365) || ' days')::interval
FROM generate_series(1, :scale_stock::integer) AS rows(n)
ON CONFLICT DO NOTHING;

INSERT INTO stock_ledger (
    id, tenant_id, branch_id, batch_id, source_event_id, source_line_key,
    direction, adjustment_sign, quantity, unit_cost, occurred_at
)
SELECT
    md5(:'tenant_id' || ':stock-ledger:' || n)::uuid,
    :'tenant_id'::uuid,
    :'branch_id'::uuid,
    md5(:'tenant_id' || ':batch:' || (((n - 1) % :scale_batches::integer) + 1))::uuid,
    md5(:'tenant_id' || ':stock-event:' || n)::uuid,
    'phase-w-line-' || n,
    CASE WHEN n % 5 = 0 THEN 'out' ELSE 'in' END,
    1,
    (1 + (n % 7))::numeric(19,4),
    (10 + (n % 100))::numeric(19,4),
    now() - ((n % 365) || ' days')::interval
FROM generate_series(1, :scale_stock::integer) AS rows(n)
ON CONFLICT DO NOTHING;

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

-- Canonical posted sales provide report rows and the source documents required
-- by the disposable GL and party-ledger volume. They are not imported business
-- data and are never written to the real local application database.
\echo Phase W fixture: canonical documents and lines
INSERT INTO business_documents (
    id, tenant_id, branch_id, counter_id, operator_id, kind, document_number,
    status, occurred_at, customer_id, subtotal, total_amount, paid_amount,
    balance_amount
)
SELECT
    md5(:'tenant_id' || ':document:' || n)::uuid,
    :'tenant_id'::uuid,
    :'branch_id'::uuid,
    '90000000-0000-0000-0000-000000000111'::uuid,
    '90000000-0000-0000-0000-000000000121'::uuid,
    'credit-sale',
    'PHASE-W-' || lpad(n::text, 10, '0'),
    'posted',
    now() - ((n % 365) || ' days')::interval,
    :'party_id'::uuid,
    100.00,
    100.00,
    0,
    100.00
FROM generate_series(1, :scale_gl::integer) AS docs(n)
ON CONFLICT DO NOTHING;

INSERT INTO business_document_lines (
    id, tenant_id, branch_id, document_id, line_number, item_id,
    item_legacy_id, item_code, item_name, quantity, unit_price,
    line_gross, line_total
)
SELECT
    md5(:'tenant_id' || ':document-line:' || n)::uuid,
    :'tenant_id'::uuid,
    :'branch_id'::uuid,
    md5(:'tenant_id' || ':document:' || n)::uuid,
    1,
    md5(:'tenant_id' || ':item:' || (((n - 1) % :scale_items::integer) + 1))::uuid,
    'ITEM-' || lpad((((n - 1) % :scale_items::integer) + 1)::text, 6, '0'),
    'ITEM-' || lpad((((n - 1) % :scale_items::integer) + 1)::text, 6, '0'),
    'Phase W Item ' || (((n - 1) % :scale_items::integer) + 1),
    1,
    100,
    100,
    100
FROM generate_series(1, :scale_gl::integer) AS docs(n)
ON CONFLICT DO NOTHING;

INSERT INTO sync_events (
    event_id, tenant_id, branch_id, aggregate, aggregate_id,
    idempotency_key, payload, occurred_at
)
SELECT
    md5(:'tenant_id' || ':document-event:' || n)::uuid,
    :'tenant_id'::uuid,
    :'branch_id'::uuid,
    'phase_w_sale',
    md5(:'tenant_id' || ':document:' || n)::uuid,
    'phase-w-document-' || n,
    jsonb_build_object('status', 'posted', 'benchmark', true),
    now() - ((n % 365) || ' days')::interval
FROM generate_series(1, :scale_gl::integer) AS docs(n)
ON CONFLICT DO NOTHING;

\echo Phase W fixture: GL journals, lines, and party ledger
INSERT INTO gl_journals (
    id, tenant_id, branch_id, source_event_id, source_document_id, kind,
    posted_at, total_debit, total_credit
)
SELECT
    md5(:'tenant_id' || ':journal:' || n)::uuid,
    :'tenant_id'::uuid,
    :'branch_id'::uuid,
    md5(:'tenant_id' || ':document-event:' || n)::uuid,
    md5(:'tenant_id' || ':document:' || n)::uuid,
    'credit-sale',
    now() - ((n % 365) || ' days')::interval,
    100,
    100
FROM generate_series(1, :scale_gl::integer) AS docs(n)
ON CONFLICT DO NOTHING;

INSERT INTO gl_lines (
    id, tenant_id, branch_id, journal_id, line_number, account_id,
    party_id, debit_amount, credit_amount, memo
)
SELECT
    md5(:'tenant_id' || ':gl-line-cash:' || n)::uuid,
    :'tenant_id'::uuid, :'branch_id'::uuid,
    md5(:'tenant_id' || ':journal:' || n)::uuid, 1, cash.id,
    :'party_id'::uuid, 100, 0, 'Phase W debit'
FROM generate_series(1, :scale_gl::integer) AS docs(n)
CROSS JOIN LATERAL (
    SELECT id FROM finance_accounts
    WHERE tenant_id = :'tenant_id'::uuid AND system_key = 'accounts_receivable'
) cash
ON CONFLICT DO NOTHING;

INSERT INTO gl_lines (
    id, tenant_id, branch_id, journal_id, line_number, account_id,
    debit_amount, credit_amount, memo
)
SELECT
    md5(:'tenant_id' || ':gl-line-revenue:' || n)::uuid,
    :'tenant_id'::uuid, :'branch_id'::uuid,
    md5(:'tenant_id' || ':journal:' || n)::uuid, 2, revenue.id,
    0, 100, 'Phase W credit'
FROM generate_series(1, :scale_gl::integer) AS docs(n)
CROSS JOIN LATERAL (
    SELECT id FROM finance_accounts
    WHERE tenant_id = :'tenant_id'::uuid AND system_key = 'sales_revenue'
) revenue
ON CONFLICT DO NOTHING;

INSERT INTO party_ledger_entries (
    id, tenant_id, branch_id, party_id, counterparty_kind, source_event_id,
    source_document_id, entry_kind, debit_amount, credit_amount,
    balance_after, occurred_at, description
)
SELECT
    md5(:'tenant_id' || ':party-ledger:' || n)::uuid,
    :'tenant_id'::uuid, :'branch_id'::uuid, :'party_id'::uuid, 'customer',
    md5(:'tenant_id' || ':document-event:' || n)::uuid,
    md5(:'tenant_id' || ':document:' || n)::uuid, 'sale', 100, 0,
    (n * 100)::numeric(19,4),
    now() - ((n % 365) || ' days')::interval,
    'Phase W customer ledger'
FROM generate_series(1, :scale_gl::integer) AS docs(n)
ON CONFLICT DO NOTHING;

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

COMMIT;

ANALYZE stock_batches;
ANALYZE stock_ledger;
ANALYZE stock_balances;
ANALYZE business_documents;
ANALYZE business_document_lines;
ANALYZE sync_events;
ANALYZE gl_journals;
ANALYZE gl_lines;
ANALYZE party_ledger_entries;
