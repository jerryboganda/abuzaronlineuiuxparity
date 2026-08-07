-- Phase F Item Form association pairs.
--
-- The captured dbo.ItemAssociation source contains ICode and AssocICode.
-- Keep both legacy identities even when a future historical import cannot yet
-- resolve the associated item into the canonical master.

CREATE TABLE IF NOT EXISTS master_item_associations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    item_id uuid NOT NULL REFERENCES master_items(id) ON DELETE CASCADE,
    associated_item_id uuid REFERENCES master_items(id) ON DELETE SET NULL,
    legacy_item_id text NOT NULL,
    associated_legacy_item_id text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, item_id, associated_legacy_item_id),
    CHECK (legacy_item_id <> associated_legacy_item_id),
    CHECK (associated_item_id IS NULL OR item_id <> associated_item_id)
);

CREATE INDEX IF NOT EXISTS idx_master_item_associations_item
    ON master_item_associations (tenant_id, item_id, active, associated_legacy_item_id);

ALTER TABLE master_item_associations ENABLE ROW LEVEL SECURITY;
ALTER TABLE master_item_associations FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS master_item_associations_tenant_scope ON master_item_associations;
CREATE POLICY master_item_associations_tenant_scope ON master_item_associations
    USING (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    );
