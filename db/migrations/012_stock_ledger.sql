-- Phase J stock/batch ledger.
--
-- Stock is append-only at the movement boundary. stock_balances is only a
-- rebuildable cache; it is never the authority for history or valuation.
-- Every business key includes tenant and branch scope so a batch or movement
-- cannot be reused across an operational boundary.

ALTER TABLE business_documents ADD COLUMN IF NOT EXISTS godown_id uuid;
CREATE UNIQUE INDEX IF NOT EXISTS uq_master_godowns_tenant_id
    ON master_godowns (tenant_id, id);
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'business_documents_godown_scope_fkey'
    ) THEN
        ALTER TABLE business_documents
            ADD CONSTRAINT business_documents_godown_scope_fkey
            FOREIGN KEY (tenant_id, godown_id)
            REFERENCES master_godowns(tenant_id, id);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'business_document_lines_tenant_id_key'
    ) THEN
        ALTER TABLE business_document_lines
            ADD CONSTRAINT business_document_lines_tenant_id_key UNIQUE (tenant_id, id);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS stock_batches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    item_id uuid NOT NULL,
    item_legacy_id text NOT NULL,
    godown_id uuid NOT NULL,
    batch_number text NOT NULL,
    expiry_date date,
    locked boolean NOT NULL DEFAULT false,
    unit_cost numeric(19, 4) NOT NULL DEFAULT 0 CHECK (unit_cost >= 0),
    received_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, item_id) REFERENCES master_items(tenant_id, id),
    FOREIGN KEY (tenant_id, godown_id) REFERENCES master_godowns(tenant_id, id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_stock_batches_identity
    ON stock_batches (
        tenant_id, branch_id, item_id, godown_id, batch_number,
        COALESCE(expiry_date, DATE '9999-12-31')
    );
CREATE INDEX IF NOT EXISTS idx_stock_batches_lookup
    ON stock_batches (tenant_id, branch_id, item_legacy_id, godown_id, locked, expiry_date);

CREATE TABLE IF NOT EXISTS stock_ledger (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    batch_id uuid NOT NULL,
    source_event_id uuid NOT NULL,
    source_document_id uuid,
    source_document_line_id uuid,
    source_line_key text NOT NULL,
    direction text NOT NULL CHECK (direction IN ('in', 'out', 'adjustment')),
    adjustment_sign smallint NOT NULL CHECK (adjustment_sign IN (-1, 1)),
    quantity numeric(19, 4) NOT NULL CHECK (quantity > 0),
    unit_cost numeric(19, 4) NOT NULL DEFAULT 0 CHECK (unit_cost >= 0),
    occurred_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, id),
    UNIQUE (tenant_id, source_event_id, source_line_key, batch_id, direction),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id, batch_id) REFERENCES stock_batches(tenant_id, branch_id, id),
    FOREIGN KEY (tenant_id, source_event_id) REFERENCES sync_events(tenant_id, event_id),
    FOREIGN KEY (tenant_id, source_document_id) REFERENCES business_documents(tenant_id, id),
    FOREIGN KEY (tenant_id, source_document_line_id) REFERENCES business_document_lines(tenant_id, id)
);
ALTER TABLE stock_ledger ALTER COLUMN adjustment_sign DROP DEFAULT;
CREATE INDEX IF NOT EXISTS idx_stock_ledger_batch_time
    ON stock_ledger (tenant_id, branch_id, batch_id, occurred_at, id);
CREATE INDEX IF NOT EXISTS idx_stock_ledger_source_document
    ON stock_ledger (tenant_id, source_document_id, source_document_line_id);

CREATE TABLE IF NOT EXISTS stock_allocations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    document_id uuid NOT NULL,
    document_line_id uuid NOT NULL,
    batch_id uuid NOT NULL,
    movement_id uuid NOT NULL,
    quantity numeric(19, 4) NOT NULL CHECK (quantity > 0),
    unit_cost numeric(19, 4) NOT NULL DEFAULT 0 CHECK (unit_cost >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, id),
    UNIQUE (tenant_id, document_line_id, batch_id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, document_id) REFERENCES business_documents(tenant_id, id),
    FOREIGN KEY (tenant_id, document_line_id) REFERENCES business_document_lines(tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id, batch_id) REFERENCES stock_batches(tenant_id, branch_id, id),
    FOREIGN KEY (tenant_id, branch_id, movement_id) REFERENCES stock_ledger(tenant_id, branch_id, id)
);
CREATE INDEX IF NOT EXISTS idx_stock_allocations_document
    ON stock_allocations (tenant_id, document_id, document_line_id);

CREATE TABLE IF NOT EXISTS stock_balances (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    batch_id uuid NOT NULL,
    item_id uuid NOT NULL,
    item_legacy_id text NOT NULL,
    godown_id uuid NOT NULL,
    on_hand numeric(19, 4) NOT NULL DEFAULT 0 CHECK (on_hand >= 0),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, branch_id, batch_id),
    UNIQUE (tenant_id, branch_id, item_id, godown_id, batch_id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id, batch_id) REFERENCES stock_batches(tenant_id, branch_id, id),
    FOREIGN KEY (tenant_id, item_id) REFERENCES master_items(tenant_id, id),
    FOREIGN KEY (tenant_id, godown_id) REFERENCES master_godowns(tenant_id, id)
);
CREATE INDEX IF NOT EXISTS idx_stock_balances_available
    ON stock_balances (tenant_id, branch_id, item_legacy_id, godown_id, on_hand);

CREATE TABLE IF NOT EXISTS stock_balance_rebuilds (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    status text NOT NULL CHECK (status IN ('running', 'completed', 'failed')),
    allocation_policy text NOT NULL,
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    batch_count bigint NOT NULL DEFAULT 0,
    balance_count bigint NOT NULL DEFAULT 0,
    error_text text NOT NULL DEFAULT '',
    UNIQUE (tenant_id, branch_id, id)
);
CREATE INDEX IF NOT EXISTS idx_stock_balance_rebuilds_scope
    ON stock_balance_rebuilds (tenant_id, branch_id, started_at DESC);

CREATE OR REPLACE FUNCTION reject_stock_ledger_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'stock_ledger is immutable';
END;
$$;

DROP TRIGGER IF EXISTS stock_ledger_immutable ON stock_ledger;
CREATE TRIGGER stock_ledger_immutable
BEFORE UPDATE OR DELETE ON stock_ledger
FOR EACH ROW EXECUTE FUNCTION reject_stock_ledger_mutation();

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'stock_batches', 'stock_ledger', 'stock_allocations',
        'stock_balances', 'stock_balance_rebuilds'
    ] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', table_name);
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', table_name || '_tenant_scope', table_name);
        EXECUTE format(
            'CREATE POLICY %I ON %I USING (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid OR current_setting(''app.authenticating'', true) = ''true'') WITH CHECK (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid OR current_setting(''app.authenticating'', true) = ''true'')',
            table_name || '_tenant_scope', table_name
        );
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', table_name || '_branch_scope', table_name);
        EXECUTE format(
            'CREATE POLICY %I ON %I USING (current_setting(''app.authenticating'', true) = ''true'' OR branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid OR NULLIF(current_setting(''app.allow_tenant_scope'', true), '''') = ''true'') WITH CHECK (current_setting(''app.authenticating'', true) = ''true'' OR branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid OR NULLIF(current_setting(''app.allow_tenant_scope'', true), '''') = ''true'')',
            table_name || '_branch_scope', table_name
        );
    END LOOP;
END $$;
