-- Phase F normalized master-data storage.
--
-- master_records remains the reviewed Phase E compatibility store. These
-- tables provide stable targets for API CRUD and future imports while keeping
-- every source identifier as text. Master data is tenant-wide; the API still
-- validates the authenticated branch context and RLS supplies the tenant
-- boundary.

CREATE TABLE IF NOT EXISTS master_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    legacy_id text NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, legacy_id),
    UNIQUE (tenant_id, code)
);

CREATE TABLE IF NOT EXISTS master_parties (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    party_type text NOT NULL CHECK (party_type IN ('customer', 'supplier')),
    legacy_id text NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, party_type, legacy_id),
    UNIQUE (tenant_id, party_type, code)
);

CREATE TABLE IF NOT EXISTS master_manufacturers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    legacy_id text NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, legacy_id),
    UNIQUE (tenant_id, code)
);

CREATE TABLE IF NOT EXISTS master_item_groups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    legacy_id text NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, legacy_id),
    UNIQUE (tenant_id, code)
);

CREATE TABLE IF NOT EXISTS master_categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    category_kind text NOT NULL,
    legacy_id text NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, category_kind, legacy_id),
    UNIQUE (tenant_id, category_kind, code)
);

CREATE TABLE IF NOT EXISTS master_godowns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    legacy_id text NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, legacy_id),
    UNIQUE (tenant_id, code)
);

CREATE TABLE IF NOT EXISTS master_aliases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    item_id uuid NOT NULL REFERENCES master_items(id) ON DELETE CASCADE,
    alias_kind text NOT NULL CHECK (alias_kind IN ('alias', 'alternate_alias', 'barcode', 'legacy_id')),
    alias_value text NOT NULL,
    normalized_value text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, alias_kind, normalized_value),
    UNIQUE (tenant_id, item_id, alias_kind, normalized_value)
);

CREATE TABLE IF NOT EXISTS item_suppliers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    item_id uuid REFERENCES master_items(id) ON DELETE CASCADE,
    supplier_id uuid REFERENCES master_parties(id) ON DELETE SET NULL,
    legacy_item_id text NOT NULL,
    legacy_supplier_id text NOT NULL,
    priority integer,
    rate numeric(19, 4),
    discount_percent numeric(9, 4),
    quantity numeric(19, 4),
    bonus numeric(19, 4),
    days integer,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, legacy_item_id, legacy_supplier_id)
);

CREATE INDEX IF NOT EXISTS idx_master_items_lookup
    ON master_items (tenant_id, active, name, code, legacy_id);
CREATE INDEX IF NOT EXISTS idx_master_parties_lookup
    ON master_parties (tenant_id, party_type, active, name, code, legacy_id);
CREATE INDEX IF NOT EXISTS idx_master_manufacturers_lookup
    ON master_manufacturers (tenant_id, active, name, code, legacy_id);
CREATE INDEX IF NOT EXISTS idx_master_item_groups_lookup
    ON master_item_groups (tenant_id, active, name, code, legacy_id);
CREATE INDEX IF NOT EXISTS idx_master_categories_lookup
    ON master_categories (tenant_id, category_kind, active, name, code, legacy_id);
CREATE INDEX IF NOT EXISTS idx_master_godowns_lookup
    ON master_godowns (tenant_id, active, name, code, legacy_id);
CREATE INDEX IF NOT EXISTS idx_master_aliases_lookup
    ON master_aliases (tenant_id, alias_kind, normalized_value);
CREATE INDEX IF NOT EXISTS idx_item_suppliers_item
    ON item_suppliers (tenant_id, item_id, legacy_item_id);

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'master_items', 'master_parties', 'master_manufacturers',
        'master_item_groups', 'master_categories', 'master_godowns',
        'master_aliases', 'item_suppliers'
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

-- Backfill the normalized targets from the reviewed Phase E store. No
-- inferred legacy fields are created; the complete source payload remains
-- available for fields that are not yet promoted to typed columns.
INSERT INTO master_items (tenant_id, legacy_id, code, name, payload, active, created_at, updated_at)
SELECT tenant_id, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
FROM master_records
WHERE kind = 'item'
ON CONFLICT (tenant_id, legacy_id) DO UPDATE
SET code = EXCLUDED.code, name = EXCLUDED.name, payload = EXCLUDED.payload,
    active = EXCLUDED.active, updated_at = EXCLUDED.updated_at;

INSERT INTO master_parties (tenant_id, party_type, legacy_id, code, name, payload, active, created_at, updated_at)
SELECT tenant_id, kind, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
FROM master_records
WHERE kind IN ('customer', 'supplier')
ON CONFLICT (tenant_id, party_type, legacy_id) DO UPDATE
SET code = EXCLUDED.code, name = EXCLUDED.name, payload = EXCLUDED.payload,
    active = EXCLUDED.active, updated_at = EXCLUDED.updated_at;

INSERT INTO master_manufacturers (tenant_id, legacy_id, code, name, payload, active, created_at, updated_at)
SELECT tenant_id, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
FROM master_records
WHERE kind = 'manufacturer'
ON CONFLICT (tenant_id, legacy_id) DO UPDATE
SET code = EXCLUDED.code, name = EXCLUDED.name, payload = EXCLUDED.payload,
    active = EXCLUDED.active, updated_at = EXCLUDED.updated_at;

INSERT INTO master_item_groups (tenant_id, legacy_id, code, name, payload, active, created_at, updated_at)
SELECT tenant_id, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
FROM master_records
WHERE kind = 'item_group'
ON CONFLICT (tenant_id, legacy_id) DO UPDATE
SET code = EXCLUDED.code, name = EXCLUDED.name, payload = EXCLUDED.payload,
    active = EXCLUDED.active, updated_at = EXCLUDED.updated_at;

INSERT INTO master_categories (tenant_id, category_kind, legacy_id, code, name, payload, active, created_at, updated_at)
SELECT tenant_id, kind, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
FROM master_records
WHERE kind IN ('category', 'item_category', 'customer_category', 'supplier_category', 'manufacturer_category')
ON CONFLICT (tenant_id, category_kind, legacy_id) DO UPDATE
SET code = EXCLUDED.code, name = EXCLUDED.name, payload = EXCLUDED.payload,
    active = EXCLUDED.active, updated_at = EXCLUDED.updated_at;

INSERT INTO master_godowns (tenant_id, legacy_id, code, name, payload, active, created_at, updated_at)
SELECT tenant_id, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
FROM master_records
WHERE kind = 'godown'
ON CONFLICT (tenant_id, legacy_id) DO UPDATE
SET code = EXCLUDED.code, name = EXCLUDED.name, payload = EXCLUDED.payload,
    active = EXCLUDED.active, updated_at = EXCLUDED.updated_at;

INSERT INTO master_aliases (tenant_id, item_id, alias_kind, alias_value, normalized_value)
SELECT i.tenant_id, i.id, 'legacy_id', i.legacy_id, lower(i.legacy_id)
FROM master_items i
ON CONFLICT (tenant_id, alias_kind, normalized_value) DO NOTHING;

INSERT INTO master_aliases (tenant_id, item_id, alias_kind, alias_value, normalized_value)
SELECT i.tenant_id, i.id, 'alias', p.alias_value, lower(p.alias_value)
FROM master_items i
CROSS JOIN LATERAL (
    VALUES
        (NULLIF(i.payload->>'CustomICode', '')),
        (NULLIF(i.payload->>'AliasName', '')),
        (NULLIF(i.payload->>'Barcode', '')),
        (NULLIF(i.payload->>'BarCode', ''))
) AS p(alias_value)
WHERE p.alias_value IS NOT NULL
ON CONFLICT (tenant_id, alias_kind, normalized_value) DO NOTHING;

INSERT INTO item_suppliers (
    tenant_id, item_id, supplier_id, legacy_item_id, legacy_supplier_id,
    priority, rate, discount_percent, quantity, bonus, days, payload,
    created_at, updated_at
)
SELECT l.tenant_id, i.id, s.id, l.legacy_item_id, l.legacy_supplier_id,
       l.priority, l.rate, l.discount_percent, l.sale_quantity,
       l.bonus_quantity, l.scheme_days, l.payload, l.created_at, l.updated_at
FROM item_supplier_links l
LEFT JOIN master_items i
    ON i.tenant_id = l.tenant_id AND i.legacy_id = l.legacy_item_id
LEFT JOIN master_parties s
    ON s.tenant_id = l.tenant_id AND s.party_type = 'supplier'
   AND s.legacy_id = l.legacy_supplier_id
ON CONFLICT (tenant_id, legacy_item_id, legacy_supplier_id) DO UPDATE
SET item_id = EXCLUDED.item_id, supplier_id = EXCLUDED.supplier_id,
    priority = EXCLUDED.priority, rate = EXCLUDED.rate,
    discount_percent = EXCLUDED.discount_percent, quantity = EXCLUDED.quantity,
    bonus = EXCLUDED.bonus, days = EXCLUDED.days, payload = EXCLUDED.payload,
    updated_at = EXCLUDED.updated_at;

-- Read adapters keep Phase E/importer consumers compatible while preferring
-- normalized rows. They are deliberately read-only and do not bypass RLS on
-- the underlying tables for API queries.
CREATE OR REPLACE VIEW master_items_compat AS
SELECT i.tenant_id, i.id, i.legacy_id, i.code, i.name, i.payload, i.active,
       i.created_at, i.updated_at
FROM master_items i
UNION ALL
SELECT m.tenant_id, m.id, COALESCE(NULLIF(m.legacy_id, ''), m.code),
       m.code, m.name, m.payload, m.active, m.created_at, m.updated_at
FROM master_records m
WHERE m.kind = 'item'
  AND NOT EXISTS (
      SELECT 1 FROM master_items i
      WHERE i.tenant_id = m.tenant_id
        AND i.legacy_id = COALESCE(NULLIF(m.legacy_id, ''), m.code)
  );

CREATE OR REPLACE VIEW master_parties_compat AS
SELECT p.tenant_id, p.id, p.party_type, p.legacy_id, p.code, p.name,
       p.payload, p.active, p.created_at, p.updated_at
FROM master_parties p
UNION ALL
SELECT m.tenant_id, m.id, m.kind, COALESCE(NULLIF(m.legacy_id, ''), m.code),
       m.code, m.name, m.payload, m.active, m.created_at, m.updated_at
FROM master_records m
WHERE m.kind IN ('customer', 'supplier')
  AND NOT EXISTS (
      SELECT 1 FROM master_parties p
      WHERE p.tenant_id = m.tenant_id AND p.party_type = m.kind
        AND p.legacy_id = COALESCE(NULLIF(m.legacy_id, ''), m.code)
  );

CREATE OR REPLACE VIEW item_suppliers_compat AS
SELECT s.tenant_id, s.id, s.legacy_item_id, s.legacy_supplier_id,
       s.priority, s.rate, s.discount_percent, s.quantity, s.bonus, s.days,
       s.payload, s.created_at, s.updated_at
FROM item_suppliers s
UNION ALL
SELECT l.tenant_id, l.id, l.legacy_item_id, l.legacy_supplier_id,
       l.priority, l.rate, l.discount_percent, l.sale_quantity,
       l.bonus_quantity, l.scheme_days, l.payload, l.created_at, l.updated_at
FROM item_supplier_links l
WHERE NOT EXISTS (
    SELECT 1 FROM item_suppliers s
    WHERE s.tenant_id = l.tenant_id
      AND s.legacy_item_id = l.legacy_item_id
      AND s.legacy_supplier_id = l.legacy_supplier_id
);
