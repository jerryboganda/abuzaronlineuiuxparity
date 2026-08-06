-- Phase V preference scope and reviewed-registry support.
-- Existing tenant defaults remain readable as a fallback; writes made with an
-- operational context are isolated to that tenant branch.

ALTER TABLE tenant_preferences
    ADD COLUMN IF NOT EXISTS branch_id uuid;

ALTER TABLE tenant_preferences
    DROP CONSTRAINT IF EXISTS tenant_preferences_pkey;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'tenant_preferences_branch_fk'
          AND conrelid = 'tenant_preferences'::regclass
    ) THEN
        ALTER TABLE tenant_preferences
            ADD CONSTRAINT tenant_preferences_branch_fk
            FOREIGN KEY (tenant_id, branch_id) REFERENCES branches (tenant_id, id)
            ON DELETE CASCADE;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_preferences_scope
    ON tenant_preferences (
        tenant_id,
        COALESCE(branch_id, '00000000-0000-0000-0000-000000000000'::uuid),
        category,
        caption
    );

CREATE INDEX IF NOT EXISTS idx_tenant_preferences_branch_category
    ON tenant_preferences (tenant_id, branch_id, category, position, caption);

DROP POLICY IF EXISTS tenant_preferences_tenant_scope ON tenant_preferences;
DROP POLICY IF EXISTS tenant_preferences_branch_scope ON tenant_preferences;
CREATE POLICY tenant_preferences_branch_scope ON tenant_preferences
    USING (
        current_setting('app.authenticating', true) = 'true'
        OR branch_id IS NULL
        OR branch_id = NULLIF(current_setting('app.branch_id', true), '')::uuid
        OR NULLIF(current_setting('app.allow_tenant_scope', true), '') = 'true'
    )
    WITH CHECK (
        current_setting('app.authenticating', true) = 'true'
        OR branch_id IS NULL
        OR branch_id = NULLIF(current_setting('app.branch_id', true), '')::uuid
        OR NULLIF(current_setting('app.allow_tenant_scope', true), '') = 'true'
    );
