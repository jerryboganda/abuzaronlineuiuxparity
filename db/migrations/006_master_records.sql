-- Online-only master-data records for the first parity waves. The payload
-- preserves the legacy form fields while the stable code/name columns support
-- fast list/search operations. Business transactions remain immutable events.
CREATE TABLE IF NOT EXISTS master_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('customer', 'supplier', 'item', 'user', 'category', 'manufacturer', 'template')),
    code text NOT NULL,
    name text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, kind, code),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

CREATE INDEX IF NOT EXISTS idx_master_records_scope ON master_records (tenant_id, kind, active, name);
ALTER TABLE master_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE master_records FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS master_records_tenant_scope ON master_records;
CREATE POLICY master_records_tenant_scope ON master_records
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid OR current_setting('app.authenticating', true) = 'true')
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid OR current_setting('app.authenticating', true) = 'true');
