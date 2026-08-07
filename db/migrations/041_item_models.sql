-- Phase F Item Form model membership.
--
-- The captured dbo.ItemInModel source stores the owning ICode and ModelCode.
-- Preserve the source code directly; the separate dbo.Model master remains a
-- distinct migration/reconciliation concern.

CREATE TABLE IF NOT EXISTS master_item_models (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    item_id uuid NOT NULL REFERENCES master_items(id) ON DELETE CASCADE,
    legacy_item_id text NOT NULL,
    model_code smallint NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, item_id, model_code)
);

CREATE INDEX IF NOT EXISTS idx_master_item_models_item
    ON master_item_models (tenant_id, item_id, active, model_code);

ALTER TABLE master_item_models ENABLE ROW LEVEL SECURITY;
ALTER TABLE master_item_models FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS master_item_models_tenant_scope ON master_item_models;
CREATE POLICY master_item_models_tenant_scope ON master_item_models
    USING (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    );
