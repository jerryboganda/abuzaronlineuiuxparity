-- Tenant-scoped preference values. The caption and category preserve the
-- legacy preference grid while position keeps the captured ordering stable.
CREATE TABLE IF NOT EXISTS tenant_preferences (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    category text NOT NULL,
    caption text NOT NULL,
    value text NOT NULL DEFAULT '',
    position integer NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, category, caption),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

CREATE INDEX IF NOT EXISTS idx_tenant_preferences_category ON tenant_preferences (tenant_id, category, position);
ALTER TABLE tenant_preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_preferences FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_preferences_tenant_scope ON tenant_preferences;
CREATE POLICY tenant_preferences_tenant_scope ON tenant_preferences
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid OR current_setting('app.authenticating', true) = 'true')
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid OR current_setting('app.authenticating', true) = 'true');
