-- Source-backed DeletedSaleItem report projection.
--
-- DeletedSaleItem is a historical audit stream, not a current sale line. Keep
-- it separate from business_document_lines so deleted rows remain reportable
-- even after the live document is edited or removed.

CREATE TABLE IF NOT EXISTS historical_deleted_sale_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL,
    legacy_id text NOT NULL,
    item_id uuid,
    item_legacy_id text NOT NULL,
    godown_legacy_id text NOT NULL,
    pack_units integer NOT NULL DEFAULT 0,
    quantity numeric(19, 4) NOT NULL DEFAULT 0,
    bonus_quantity numeric(19, 4) NOT NULL DEFAULT 0,
    sale_price numeric(19, 4) NOT NULL DEFAULT 0,
    discount_percent numeric(19, 4) NOT NULL DEFAULT 0,
    item_flat_discount numeric(19, 4) NOT NULL DEFAULT 0,
    unit_sales_tax numeric(19, 4) NOT NULL DEFAULT 0,
    gst_percent numeric(19, 4) NOT NULL DEFAULT 0,
    occurred_at timestamptz NOT NULL,
    machine_name text NOT NULL DEFAULT '',
    user_legacy_id text NOT NULL DEFAULT '',
    sale_invoice_code text NOT NULL DEFAULT '',
    source_table text NOT NULL,
    source_table_row text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, legacy_id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, item_id) REFERENCES master_items(tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_historical_deleted_sale_items_scope
    ON historical_deleted_sale_items (tenant_id, branch_id, occurred_at DESC, sale_invoice_code);
CREATE INDEX IF NOT EXISTS idx_historical_deleted_sale_items_item
    ON historical_deleted_sale_items (tenant_id, branch_id, item_legacy_id, occurred_at DESC);

ALTER TABLE historical_deleted_sale_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE historical_deleted_sale_items FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS historical_deleted_sale_items_tenant_scope ON historical_deleted_sale_items;
CREATE POLICY historical_deleted_sale_items_tenant_scope
    ON historical_deleted_sale_items
    USING (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    );

DROP POLICY IF EXISTS historical_deleted_sale_items_branch_tenant_hardening ON historical_deleted_sale_items;
CREATE POLICY historical_deleted_sale_items_branch_tenant_hardening
    ON historical_deleted_sale_items AS RESTRICTIVE FOR ALL
    USING (
        current_setting('app.authenticating', true) = 'true'
        OR (
            tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
            AND (
                NULLIF(current_setting('app.allow_tenant_scope', true), '') = 'true'
                OR branch_id = NULLIF(current_setting('app.branch_id', true), '')::uuid
            )
        )
    )
    WITH CHECK (
        current_setting('app.authenticating', true) = 'true'
        OR (
            tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
            AND (
                NULLIF(current_setting('app.allow_tenant_scope', true), '') = 'true'
                OR branch_id = NULLIF(current_setting('app.branch_id', true), '')::uuid
            )
        )
    );
