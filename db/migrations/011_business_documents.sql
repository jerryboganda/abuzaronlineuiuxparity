-- Phase G/H document foundation.
--
-- This migration deliberately models the document lifecycle and pricing
-- snapshot only. Posting does not create stock, batch, GL, or party-ledger
-- rows; those projections belong to later parity waves.

-- 010's canonical item table has a globally unique primary key but no
-- composite tenant key. Add the non-partial unique index needed by the
-- tenant-aware line foreign key without changing the Phase F table contract.
CREATE UNIQUE INDEX IF NOT EXISTS uq_master_items_tenant_id
    ON master_items (tenant_id, id);

CREATE TABLE IF NOT EXISTS business_documents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    counter_id uuid NOT NULL REFERENCES counters(id),
    operator_id uuid NOT NULL REFERENCES users(id),
    kind text NOT NULL CHECK (kind IN ('cash-sale', 'credit-sale')),
    document_number text NOT NULL,
    status text NOT NULL CHECK (status IN ('draft', 'posted', 'void')),
    occurred_at timestamptz NOT NULL,
    customer_id uuid,
    reference text NOT NULL DEFAULT '',
    remarks text NOT NULL DEFAULT '',
    price_level smallint NOT NULL DEFAULT 1 CHECK (price_level BETWEEN 1 AND 10),
    subtotal numeric(19, 4) NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    discount_amount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
    misc_amount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (misc_amount >= 0),
    tax_amount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (tax_amount >= 0),
    total_amount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    paid_amount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (paid_amount >= 0),
    balance_amount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (balance_amount >= 0),
    pricing_inputs jsonb NOT NULL DEFAULT '{}'::jsonb,
    pricing_result jsonb NOT NULL DEFAULT '{}'::jsonb,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, kind, document_number),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id, counter_id) REFERENCES counters(tenant_id, branch_id, id),
    FOREIGN KEY (tenant_id, operator_id) REFERENCES users(tenant_id, id)
);
ALTER TABLE business_documents
    ADD COLUMN IF NOT EXISTS void_reason text NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS business_document_lines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    document_id uuid NOT NULL,
    line_number integer NOT NULL CHECK (line_number > 0),
    item_id uuid NOT NULL,
    item_legacy_id text NOT NULL,
    item_code text NOT NULL,
    item_name text NOT NULL,
    quantity numeric(19, 4) NOT NULL CHECK (quantity > 0),
    unit_price numeric(19, 4) NOT NULL CHECK (unit_price >= 0),
    line_gross numeric(19, 4) NOT NULL CHECK (line_gross >= 0),
    supplier_discount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (supplier_discount >= 0),
    item_discount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (item_discount >= 0),
    customer_discount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (customer_discount >= 0),
    line_total numeric(19, 4) NOT NULL CHECK (line_total >= 0),
    pricing jsonb NOT NULL DEFAULT '{}'::jsonb,
    notes text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, document_id, line_number),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, document_id) REFERENCES business_documents(tenant_id, id),
    FOREIGN KEY (tenant_id, item_id) REFERENCES master_items(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS business_document_revisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    document_id uuid NOT NULL,
    revision_number bigint NOT NULL CHECK (revision_number > 0),
    action text NOT NULL CHECK (action IN ('save', 'post', 'save-and-post', 'void')),
    status text NOT NULL CHECK (status IN ('draft', 'posted', 'void')),
    snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, document_id, revision_number),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, document_id) REFERENCES business_documents(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS command_receipts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    operator_id uuid NOT NULL REFERENCES users(id),
    command_id uuid NOT NULL,
    idempotency_key text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('cash-sale', 'credit-sale')),
    action text NOT NULL CHECK (action IN ('save', 'post', 'save-and-post', 'void')),
    payload_hash text NOT NULL,
    document_id uuid,
    status text NOT NULL CHECK (status IN ('accepted')),
    response jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, command_id),
    UNIQUE (tenant_id, idempotency_key),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, operator_id) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, document_id) REFERENCES business_documents(tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_business_documents_scope
    ON business_documents (tenant_id, branch_id, kind, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_business_document_lines_document
    ON business_document_lines (tenant_id, document_id, line_number);
CREATE INDEX IF NOT EXISTS idx_command_receipts_scope
    ON command_receipts (tenant_id, branch_id, created_at DESC);

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'business_documents', 'business_document_lines',
        'business_document_revisions', 'command_receipts'
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

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'business_documents', 'business_document_lines',
        'business_document_revisions', 'command_receipts'
    ] LOOP
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', table_name || '_branch_scope', table_name);
        EXECUTE format(
            'CREATE POLICY %I ON %I USING (current_setting(''app.authenticating'', true) = ''true'' OR branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid OR NULLIF(current_setting(''app.allow_tenant_scope'', true), '''') = ''true'') WITH CHECK (current_setting(''app.authenticating'', true) = ''true'' OR branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid OR NULLIF(current_setting(''app.allow_tenant_scope'', true), '''') = ''true'')',
            table_name || '_branch_scope', table_name
        );
    END LOOP;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'business_documents_credit_customer_check'
    ) THEN
        ALTER TABLE business_documents
            ADD CONSTRAINT business_documents_credit_customer_check
            CHECK (kind <> 'credit-sale' OR customer_id IS NOT NULL);
    END IF;
END $$;
