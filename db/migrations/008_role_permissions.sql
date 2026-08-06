-- Tenant-scoped permissions for the captured Manage -> Groups workflow.
CREATE TABLE IF NOT EXISTS role_permissions (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission text NOT NULL,
    allowed boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, role_id, permission),
    FOREIGN KEY (tenant_id, role_id) REFERENCES roles(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_role_permissions_role ON role_permissions (tenant_id, role_id, permission);
ALTER TABLE role_permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE role_permissions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS role_permissions_tenant_scope ON role_permissions;
CREATE POLICY role_permissions_tenant_scope ON role_permissions
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid OR current_setting('app.authenticating', true) = 'true')
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid OR current_setting('app.authenticating', true) = 'true');
