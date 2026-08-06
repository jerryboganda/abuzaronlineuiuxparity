-- Phase R/E security-data import adaptation.
--
-- The legacy security tables use numeric composite keys while the application
-- identity model uses UUID roles/users. These columns retain the reviewed
-- source identifiers and payloads without weakening the existing tenant/role
-- foreign keys. Migration tooling resolves the UUID joins from the retained
-- legacy keys; it does not infer them from names.

ALTER TABLE roles
    ADD COLUMN IF NOT EXISTS legacy_group_id text,
    ADD COLUMN IF NOT EXISTS legacy_payload jsonb NOT NULL DEFAULT '{}'::jsonb;

CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_legacy_group_id
    ON roles (tenant_id, legacy_group_id)
    WHERE legacy_group_id IS NOT NULL;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS legacy_user_id text,
    ADD COLUMN IF NOT EXISTS legacy_payload jsonb NOT NULL DEFAULT '{}'::jsonb;

DROP INDEX IF EXISTS idx_users_legacy_user_id;
CREATE UNIQUE INDEX idx_users_legacy_user_id
    ON users (tenant_id, legacy_user_id);

ALTER TABLE user_memberships
    ADD COLUMN IF NOT EXISTS legacy_user_code text,
    ADD COLUMN IF NOT EXISTS legacy_group_code text;

ALTER TABLE legacy_groups
    ADD COLUMN IF NOT EXISTS legacy_payload jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE group_rights
    ADD COLUMN IF NOT EXISTS legacy_group_code text,
    ADD COLUMN IF NOT EXISTS legacy_payload jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE group_allowed_scopes
    DROP CONSTRAINT IF EXISTS group_allowed_scopes_scope_kind_check;

ALTER TABLE group_allowed_scopes
    ADD CONSTRAINT group_allowed_scopes_scope_kind_check CHECK (scope_kind IN (
        'branch', 'godown', 'price', 'report', 'startup_right',
        'header', 'cash_account', 'group', 'recipient', 'service_category'
    ));

ALTER TABLE group_allowed_scopes
    ADD COLUMN IF NOT EXISTS legacy_group_code text,
    ADD COLUMN IF NOT EXISTS legacy_scope_id text,
    ADD COLUMN IF NOT EXISTS legacy_payload jsonb NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_group_allowed_scopes_legacy_source
    ON group_allowed_scopes (tenant_id, legacy_table, legacy_group_code, legacy_scope_id);
