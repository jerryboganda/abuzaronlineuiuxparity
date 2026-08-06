-- Disposable CI/local fixture. Run as the schema owner, never as the API
-- role. The application-role probe must not be able to create this data.
\set ON_ERROR_STOP on
SELECT set_config('app.authenticating', 'true', false);

INSERT INTO tenants (id, code, legal_name)
VALUES
    ('10000000-0000-0000-0000-000000000001', 'rls-probe-a', 'RLS Probe A'),
    ('10000000-0000-0000-0000-000000000002', 'rls-probe-b', 'RLS Probe B')
ON CONFLICT (id) DO NOTHING;

INSERT INTO branches (id, tenant_id, code, name)
VALUES
    ('10000000-0000-0000-0000-000000000101', '10000000-0000-0000-0000-000000000001', 'a-main', 'A Main'),
    ('10000000-0000-0000-0000-000000000102', '10000000-0000-0000-0000-000000000001', 'a-second', 'A Second'),
    ('10000000-0000-0000-0000-000000000201', '10000000-0000-0000-0000-000000000002', 'b-main', 'B Main')
ON CONFLICT (id) DO NOTHING;

INSERT INTO counters (id, tenant_id, branch_id, code, name)
VALUES
    ('10000000-0000-0000-0000-000000000301', '10000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000101', 'a-counter', 'A Counter'),
    ('10000000-0000-0000-0000-000000000302', '10000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000102', 'a-counter-2', 'A Counter 2'),
    ('10000000-0000-0000-0000-000000000401', '10000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000201', 'b-counter', 'B Counter')
ON CONFLICT (id) DO NOTHING;

INSERT INTO users (id, tenant_id, username, display_name, password_hash)
VALUES
    ('10000000-0000-0000-0000-000000000501', '10000000-0000-0000-0000-000000000001', 'rls-a', 'RLS A', 'fixture'),
    ('10000000-0000-0000-0000-000000000502', '10000000-0000-0000-0000-000000000001', 'rls-a-2', 'RLS A 2', 'fixture'),
    ('10000000-0000-0000-0000-000000000601', '10000000-0000-0000-0000-000000000002', 'rls-b', 'RLS B', 'fixture')
ON CONFLICT (id) DO NOTHING;

INSERT INTO master_items (id, tenant_id, legacy_id, code, name)
VALUES
    ('10000000-0000-0000-0000-000000000701', '10000000-0000-0000-0000-000000000001', 'rls-item-a', 'rls-item-a', 'RLS Item A'),
    ('10000000-0000-0000-0000-000000000702', '10000000-0000-0000-0000-000000000002', 'rls-item-b', 'rls-item-b', 'RLS Item B')
ON CONFLICT (id) DO NOTHING;

INSERT INTO master_godowns (id, tenant_id, legacy_id, code, name)
VALUES
    ('10000000-0000-0000-0000-000000000801', '10000000-0000-0000-0000-000000000001', 'rls-godown-a', 'rls-godown-a', 'RLS Godown A'),
    ('10000000-0000-0000-0000-000000000802', '10000000-0000-0000-0000-000000000002', 'rls-godown-b', 'rls-godown-b', 'RLS Godown B')
ON CONFLICT (id) DO NOTHING;

INSERT INTO master_parties (id, tenant_id, party_type, legacy_id, code, name)
VALUES
    ('10000000-0000-0000-0000-000000000910', '10000000-0000-0000-0000-000000000001', 'supplier', 'rls-supplier-a', 'rls-supplier-a', 'RLS Supplier A')
ON CONFLICT (id) DO NOTHING;

INSERT INTO business_documents (
    id, tenant_id, branch_id, counter_id, operator_id, kind, document_number,
    status, occurred_at, supplier_id
)
VALUES
    ('10000000-0000-0000-0000-000000000901', '10000000-0000-0000-0000-000000000001',
     '10000000-0000-0000-0000-000000000101', '10000000-0000-0000-0000-000000000301',
     '10000000-0000-0000-0000-000000000501', 'cash-sale', 'RLS-A-1', 'draft',
     '2026-08-06T00:00:00Z', NULL),
    ('10000000-0000-0000-0000-000000000902', '10000000-0000-0000-0000-000000000001',
     '10000000-0000-0000-0000-000000000102', '10000000-0000-0000-0000-000000000302',
     '10000000-0000-0000-0000-000000000502', 'cash-sale', 'RLS-A-2', 'draft',
     '2026-08-06T00:00:00Z', NULL),
    ('10000000-0000-0000-0000-000000000903', '10000000-0000-0000-0000-000000000002',
     '10000000-0000-0000-0000-000000000201', '10000000-0000-0000-0000-000000000401',
     '10000000-0000-0000-0000-000000000601', 'cash-sale', 'RLS-B-1', 'draft',
     '2026-08-06T00:00:00Z', NULL),
    ('10000000-0000-0000-0000-000000000904', '10000000-0000-0000-0000-000000000001',
     '10000000-0000-0000-0000-000000000102', '10000000-0000-0000-0000-000000000302',
     '10000000-0000-0000-0000-000000000502', 'pack-purchase', 'RLS-A-P1', 'draft',
     '2026-08-06T00:00:00Z', '10000000-0000-0000-0000-000000000910')
ON CONFLICT (id) DO NOTHING;

-- Branch-B rows in tenant A exercise the branch predicate independently of
-- the existing tenant predicate. They are intentionally owner-seeded.
INSERT INTO business_document_lines (
    id, tenant_id, branch_id, document_id, line_number, item_id,
    item_legacy_id, item_code, item_name, quantity, unit_price,
    line_gross, line_total
)
VALUES (
    '10000000-0000-0000-0000-000000000921',
    '10000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000102',
    '10000000-0000-0000-0000-000000000902',
    1, '10000000-0000-0000-0000-000000000701',
    'rls-item-a', 'rls-item-a', 'RLS Item A', 1, 10, 10, 10
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO stock_batches (
    id, tenant_id, branch_id, item_id, item_legacy_id, godown_id,
    batch_number, unit_cost
)
VALUES (
    '10000000-0000-0000-0000-00000000b001',
    '10000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000102',
    '10000000-0000-0000-0000-000000000701',
    'rls-item-a', '10000000-0000-0000-0000-000000000801',
    'RLS-B-BATCH', 10
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO voucher_entries (
    id, tenant_id, branch_id, category_code, status, amount, description
)
VALUES (
    '10000000-0000-0000-0000-00000000c001',
    '10000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000102',
    'journal', 'draft', 1, 'RLS branch-B fixture'
)
ON CONFLICT (id) DO NOTHING;

-- A finalized event gives the app-role probe a real row against which to
-- verify the explicit DELETE revoke. This is owner-only fixture setup.
INSERT INTO sync_events (
    event_id, tenant_id, branch_id, counter_id, operator_id, aggregate,
    aggregate_id, idempotency_key, schema_version, payload, occurred_at
)
VALUES (
    '10000000-0000-0000-0000-00000000a001',
    '10000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000101',
    '10000000-0000-0000-0000-000000000301',
    '10000000-0000-0000-0000-000000000501',
    'business_document',
    '10000000-0000-0000-0000-000000000901',
    'rls-final-event-a', 1,
    jsonb_build_object(
        'state', 'final',
        'eventId', '10000000-0000-0000-0000-00000000a001',
        'document', jsonb_build_object('id', '10000000-0000-0000-0000-000000000901')
    ),
    '2026-08-06T00:00:00Z'
)
ON CONFLICT (event_id) DO NOTHING;
