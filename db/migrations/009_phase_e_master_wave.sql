-- Phase E wave 1/2 target storage.
-- Legacy identifiers remain text because the source uses mixed integer and
-- composite keys. The generated UUID is an internal target identity.

ALTER TABLE master_records ADD COLUMN IF NOT EXISTS legacy_id text;

ALTER TABLE master_records DROP CONSTRAINT IF EXISTS master_records_kind_check;
ALTER TABLE master_records
    ADD CONSTRAINT master_records_kind_check CHECK (kind IN (
        'customer', 'supplier', 'item', 'user', 'category', 'manufacturer',
        'template', 'area', 'godown', 'godown_group', 'item_group',
        'customer_group', 'supplier_category', 'manufacturer_category',
        'item_category', 'customer_category', 'price_policy', 'company_header',
        'config_setting', 'preference'
    ));

CREATE UNIQUE INDEX IF NOT EXISTS idx_master_records_legacy_key
    ON master_records (tenant_id, kind, legacy_id);

CREATE TABLE IF NOT EXISTS item_supplier_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    legacy_item_id text NOT NULL,
    legacy_supplier_id text NOT NULL,
    priority integer,
    rate numeric(19, 4),
    discount_percent numeric(9, 4),
    sale_quantity numeric(19, 4),
    bonus_quantity numeric(19, 4),
    scheme_days integer,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, legacy_item_id, legacy_supplier_id)
);

CREATE INDEX IF NOT EXISTS idx_item_supplier_links_item
    ON item_supplier_links (tenant_id, legacy_item_id);

ALTER TABLE item_supplier_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE item_supplier_links FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS item_supplier_links_tenant_scope ON item_supplier_links;
CREATE POLICY item_supplier_links_tenant_scope ON item_supplier_links
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true')
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true');
