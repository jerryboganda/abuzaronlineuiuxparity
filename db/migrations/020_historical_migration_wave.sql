-- Phase E historical document, stock, finance, and rate import targets.
--
-- These columns keep source identity separate from generated target identity.
-- Historical rows are imported only by reviewed maps bound to
-- AbuzarLegacyReference; no source-side writes or canonical-database imports
-- are permitted by the migration workbench.

ALTER TABLE business_documents
    ALTER COLUMN operator_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS legacy_source_table text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS legacy_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS legacy_import_key text,
    ADD COLUMN IF NOT EXISTS legacy_payload jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE business_document_lines
    ADD COLUMN IF NOT EXISTS legacy_source_table text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS legacy_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS legacy_import_key text,
    ADD COLUMN IF NOT EXISTS legacy_payload jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE sync_events
    ADD COLUMN IF NOT EXISTS legacy_source_table text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS legacy_id text NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS uq_sync_events_legacy_identity
    ON sync_events (tenant_id, branch_id, legacy_source_table, legacy_id)
    WHERE legacy_source_table <> '' AND legacy_id <> '';

ALTER TABLE party_ledger_entries
    ADD COLUMN IF NOT EXISTS legacy_payload jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE business_documents DROP CONSTRAINT IF EXISTS business_documents_kind_check;
ALTER TABLE business_documents ADD CONSTRAINT business_documents_kind_check CHECK (kind IN (
    'cash-sale', 'credit-sale', 'cash-sale-return', 'credit-sale-return',
    'open-sale-return', 'quotation', 'refused-sale', 'sale-order',
    'pack-purchase', 'loose-purchase', 'opening-purchase',
    'purchase-return', 'purchase-order', 'purchase-quotation'
)) NOT VALID;

ALTER TABLE command_receipts DROP CONSTRAINT IF EXISTS command_receipts_kind_check;
ALTER TABLE command_receipts ADD CONSTRAINT command_receipts_kind_check CHECK (kind IN (
    'cash-sale', 'credit-sale', 'cash-sale-return', 'credit-sale-return',
    'open-sale-return', 'quotation', 'refused-sale', 'sale-order',
    'pack-purchase', 'loose-purchase', 'opening-purchase',
    'purchase-return', 'purchase-order', 'purchase-quotation'
)) NOT VALID;

DROP INDEX IF EXISTS uq_business_documents_legacy_identity;
CREATE UNIQUE INDEX IF NOT EXISTS uq_business_documents_legacy_identity
    ON business_documents (tenant_id, branch_id, legacy_import_key);
DROP INDEX IF EXISTS uq_business_document_lines_legacy_identity;
CREATE UNIQUE INDEX IF NOT EXISTS uq_business_document_lines_legacy_identity
    ON business_document_lines (tenant_id, branch_id, legacy_import_key);
CREATE INDEX IF NOT EXISTS idx_business_documents_legacy_source
    ON business_documents (tenant_id, branch_id, legacy_source_table, legacy_id);
CREATE INDEX IF NOT EXISTS idx_business_document_lines_legacy_source
    ON business_document_lines (tenant_id, branch_id, legacy_source_table, legacy_id);

ALTER TABLE stock_batches
    ADD COLUMN IF NOT EXISTS legacy_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS legacy_import_key text,
    ADD COLUMN IF NOT EXISTS source_table text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_legacy_id text NOT NULL DEFAULT '';
DROP INDEX IF EXISTS uq_stock_batches_legacy_identity;
CREATE UNIQUE INDEX IF NOT EXISTS uq_stock_batches_legacy_identity
    ON stock_batches (tenant_id, branch_id, legacy_import_key);

CREATE TABLE IF NOT EXISTS historical_stock_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL,
    legacy_id text NOT NULL,
    item_id uuid NOT NULL,
    item_legacy_id text NOT NULL,
    godown_id uuid NOT NULL,
    as_of date NOT NULL,
    quantity numeric(19, 4) NOT NULL,
    purchase_price numeric(19, 4) NOT NULL DEFAULT 0,
    sale_price numeric(19, 4) NOT NULL DEFAULT 0,
    average_price numeric(19, 4) NOT NULL DEFAULT 0,
    recent_purchase_price numeric(19, 4) NOT NULL DEFAULT 0,
    pack_units integer NOT NULL DEFAULT 0,
    source_table text NOT NULL,
    source_legacy_id text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, legacy_id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, item_id) REFERENCES master_items(tenant_id, id),
    FOREIGN KEY (tenant_id, godown_id) REFERENCES master_godowns(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS historical_gl_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL,
    legacy_id text NOT NULL,
    document_code text NOT NULL,
    document_type text NOT NULL,
    account_code text NOT NULL,
    alternate_account_code text NOT NULL DEFAULT '',
    debit_amount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (debit_amount >= 0),
    credit_amount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (credit_amount >= 0),
    occurred_at timestamptz NOT NULL,
    user_legacy_id text NOT NULL DEFAULT '',
    invoice_code text NOT NULL DEFAULT '',
    remarks text NOT NULL DEFAULT '',
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, legacy_id)
);

CREATE TABLE IF NOT EXISTS price_policy_tiers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    legacy_id text NOT NULL,
    legacy_policy_id text NOT NULL,
    quantity_limit integer NOT NULL DEFAULT 0,
    price numeric(19, 4) NOT NULL DEFAULT 0,
    expiry_date date,
    flat_discount numeric(19, 4) NOT NULL DEFAULT 0,
    discount_percent numeric(9, 4) NOT NULL DEFAULT 0,
    source_table text NOT NULL,
    source_legacy_id text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, legacy_id)
);

CREATE TABLE IF NOT EXISTS migration_ambiguous_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source_schema text NOT NULL,
    source_table text NOT NULL,
    legacy_id text NOT NULL,
    reason_code text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved', 'ignored')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, source_schema, source_table, legacy_id, reason_code)
);

CREATE INDEX IF NOT EXISTS idx_historical_stock_snapshots_scope
    ON historical_stock_snapshots (tenant_id, branch_id, item_legacy_id, godown_id, as_of);
CREATE INDEX IF NOT EXISTS idx_historical_gl_entries_scope
    ON historical_gl_entries (tenant_id, branch_id, occurred_at, document_code);
CREATE INDEX IF NOT EXISTS idx_price_policy_tiers_policy
    ON price_policy_tiers (tenant_id, legacy_policy_id, quantity_limit);
CREATE INDEX IF NOT EXISTS idx_migration_ambiguous_records_source
    ON migration_ambiguous_records (tenant_id, source_table, reason_code, status);

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'historical_stock_snapshots', 'historical_gl_entries',
        'price_policy_tiers', 'migration_ambiguous_records'
    ] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', table_name);
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I',
            table_name || '_tenant_scope', table_name);
        EXECUTE format(
            'CREATE POLICY %I ON %I USING (
                tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid
                OR current_setting(''app.authenticating'', true) = ''true''
            ) WITH CHECK (
                tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid
                OR current_setting(''app.authenticating'', true) = ''true''
            )',
            table_name || '_tenant_scope', table_name
        );
    END LOOP;
END $$;

-- The isolated historical importer needs a real counter because
-- business_documents intentionally retains the operational counter FK.
INSERT INTO counters (id, tenant_id, branch_id, code, name, active)
VALUES (
    '11111111-1111-4111-8111-111111111111',
    'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee',
    'ffffffff-ffff-ffff-ffff-ffffffffffff',
    'LEGACY-IMPORT',
    'Legacy historical import',
    true
)
ON CONFLICT (tenant_id, branch_id, code) DO NOTHING;
