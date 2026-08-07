-- Phase F Item Form notes blob.
--
-- The captured legacy ItemNotes table stores one opaque image/blob per item.
-- Keep the canonical value as tenant-scoped bytea so UTF-8, RTF, and other
-- PowerBuilder rich-text encodings can round-trip without lossy conversion.

CREATE TABLE IF NOT EXISTS master_item_notes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    item_id uuid NOT NULL REFERENCES master_items(id) ON DELETE CASCADE,
    legacy_item_id text NOT NULL,
    notes_data bytea,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, item_id)
);

CREATE INDEX IF NOT EXISTS idx_master_item_notes_item
    ON master_item_notes (tenant_id, item_id, active);

ALTER TABLE master_item_notes ENABLE ROW LEVEL SECURITY;
ALTER TABLE master_item_notes FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS master_item_notes_tenant_scope ON master_item_notes;
CREATE POLICY master_item_notes_tenant_scope ON master_item_notes
    USING (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    );
