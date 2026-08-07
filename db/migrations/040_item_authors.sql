-- Phase F Item Form author relationships.
--
-- The captured dbo.ItemAuthor source stores ICode, AuthorCode, Priority, and
-- ROWID. Preserve those relationship fields directly; the separate Author
-- master and its source-selection semantics remain an import/acceptance gate.

CREATE TABLE IF NOT EXISTS master_item_authors (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    item_id uuid NOT NULL REFERENCES master_items(id) ON DELETE CASCADE,
    legacy_item_id text NOT NULL,
    author_code integer NOT NULL,
    priority smallint NOT NULL DEFAULT 0,
    row_id integer NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, item_id, row_id),
    UNIQUE (tenant_id, item_id, author_code),
    CHECK (author_code > 0),
    CHECK (priority >= 0 AND priority <= 255),
    CHECK (row_id > 0)
);

CREATE INDEX IF NOT EXISTS idx_master_item_authors_item
    ON master_item_authors (tenant_id, item_id, active, priority, row_id);

ALTER TABLE master_item_authors ENABLE ROW LEVEL SECURITY;
ALTER TABLE master_item_authors FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS master_item_authors_tenant_scope ON master_item_authors;
CREATE POLICY master_item_authors_tenant_scope ON master_item_authors
    USING (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    );
