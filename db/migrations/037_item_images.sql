-- Phase F Item Form image collection.
--
-- The reviewed legacy ItemImage source has a per-item row identifier,
-- description, image blob, and image type. The canonical target keeps those
-- fields tenant-scoped and replaces a selected item's image collection
-- atomically through the contextual Item Form command.

CREATE TABLE IF NOT EXISTS master_item_images (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    item_id uuid NOT NULL REFERENCES master_items(id) ON DELETE CASCADE,
    legacy_item_id text NOT NULL,
    row_id integer NOT NULL,
    image_description text NOT NULL DEFAULT '',
    image_data bytea NOT NULL,
    image_type text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, item_id, row_id),
    CHECK (octet_length(image_data) <= 8388608)
);

CREATE INDEX IF NOT EXISTS idx_master_item_images_item
    ON master_item_images (tenant_id, item_id, active, row_id);

ALTER TABLE master_item_images ENABLE ROW LEVEL SECURITY;
ALTER TABLE master_item_images FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS master_item_images_tenant_scope ON master_item_images;
CREATE POLICY master_item_images_tenant_scope ON master_item_images
    USING (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    );
