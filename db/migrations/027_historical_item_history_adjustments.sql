-- Phase Q source-backed item history and stock-adjustment projections.
--
-- These tables retain the reviewed SQL Server source identity and payload.
-- They are deliberately separate from current master/stock projections: an
-- imported historical snapshot must remain reportable even when the current
-- item or godown was not imported into the target tenant.

CREATE TABLE IF NOT EXISTS historical_item_changes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL,
    report_kind text NOT NULL CHECK (report_kind IN (
        'item-price-difference',
        'item-basic-data',
        'item-sale-price',
        'item-first-observed',
        'item-name'
    )),
    source_legacy_id text NOT NULL,
    item_id uuid,
    item_legacy_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    old_name text NOT NULL DEFAULT '',
    new_name text NOT NULL DEFAULT '',
    old_sale_price numeric(19, 4),
    new_sale_price numeric(19, 4),
    stock numeric(19, 4),
    user_legacy_id text NOT NULL DEFAULT '',
    change_reason text NOT NULL DEFAULT '',
    source_table text NOT NULL,
    source_table_row text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, report_kind, source_legacy_id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, item_id) REFERENCES master_items(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS historical_stock_adjustment_lines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL,
    legacy_id text NOT NULL,
    adjustment_legacy_id text NOT NULL,
    item_id uuid,
    item_legacy_id text NOT NULL,
    godown_id uuid,
    godown_legacy_id text NOT NULL,
    batch text NOT NULL DEFAULT '',
    expiry_date date,
    occurred_at timestamptz NOT NULL,
    loose_quantity numeric(19, 8) NOT NULL DEFAULT 0,
    current_stock numeric(19, 8),
    priority integer NOT NULL DEFAULT 0,
    price numeric(19, 4) NOT NULL DEFAULT 0,
    average_price numeric(19, 4) NOT NULL DEFAULT 0,
    new_average_price numeric(19, 4),
    sale_price numeric(19, 4) NOT NULL DEFAULT 0,
    purchase_price numeric(19, 4) NOT NULL DEFAULT 0,
    recent_purchase_price numeric(19, 4) NOT NULL DEFAULT 0,
    pack_units integer NOT NULL DEFAULT 0,
    user_legacy_id text NOT NULL DEFAULT '',
    posted boolean NOT NULL DEFAULT false,
    remarks text NOT NULL DEFAULT '',
    source_table text NOT NULL,
    source_table_row text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, legacy_id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, item_id) REFERENCES master_items(tenant_id, id),
    FOREIGN KEY (tenant_id, godown_id) REFERENCES master_godowns(tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_historical_item_changes_scope
    ON historical_item_changes (tenant_id, branch_id, report_kind, occurred_at DESC, item_legacy_id);
CREATE INDEX IF NOT EXISTS idx_historical_item_changes_item
    ON historical_item_changes (tenant_id, branch_id, item_legacy_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_historical_adjustment_lines_scope
    ON historical_stock_adjustment_lines (tenant_id, branch_id, occurred_at DESC, item_legacy_id);
CREATE INDEX IF NOT EXISTS idx_historical_adjustment_lines_adjustment
    ON historical_stock_adjustment_lines (tenant_id, branch_id, adjustment_legacy_id);

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'historical_item_changes', 'historical_stock_adjustment_lines'
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
                         OR branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid
                     )
                 )
             ) WITH CHECK (
                 current_setting(''app.authenticating'', true) = ''true''
                 OR (
                     tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid
                     AND (
                         NULLIF(current_setting(''app.allow_tenant_scope'', true), '''') = ''true''
                         OR branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid
                     )
                 )
             )',
            table_name || '_branch_tenant_hardening', table_name
        );
    END LOOP;
END $$;
