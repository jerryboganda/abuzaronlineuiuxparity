-- Legacy security compatibility.
--
-- roles/user_memberships remain the canonical identity model. These tables
-- retain the legacy Groups, GroupRights and GroupAllowed* concepts so an
-- import can preserve the source codes and explicit revocations without
-- weakening the existing role_permissions contract.

CREATE TABLE IF NOT EXISTS legacy_groups (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    legacy_group_code text NOT NULL,
    legacy_group_name text NOT NULL,
    legacy_group_id text,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, role_id),
    UNIQUE (tenant_id, legacy_group_code),
    FOREIGN KEY (tenant_id, role_id) REFERENCES roles(tenant_id, id) ON DELETE CASCADE
);

-- permission is the normalized API right; right_code preserves the original
-- GroupRights value (including command identifiers) for audit and re-import.
-- A false row is an explicit deny and takes precedence over the legacy role's
-- compatible role_permissions row for that role.
CREATE TABLE IF NOT EXISTS group_rights (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    right_code text NOT NULL,
    permission text,
    allowed boolean NOT NULL DEFAULT false,
    legacy_status text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, role_id, right_code),
    FOREIGN KEY (tenant_id, role_id) REFERENCES roles(tenant_id, id) ON DELETE CASCADE
);

-- This is the normalized representation of GroupAllowedGodown,
-- GroupAllowedPrice, GroupAllowedStartupRight, GroupAllowedHeader,
-- GroupCashAccount and the other legacy allow-list tables. scope_kind keeps
-- the source concept explicit while scope_key retains the source identifier.
CREATE TABLE IF NOT EXISTS group_allowed_scopes (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    scope_kind text NOT NULL,
    scope_key text NOT NULL,
    scope_label text,
    allowed boolean NOT NULL DEFAULT true,
    legacy_table text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, role_id, scope_kind, scope_key),
    FOREIGN KEY (tenant_id, role_id) REFERENCES roles(tenant_id, id) ON DELETE CASCADE,
    CHECK (scope_kind IN (
        'branch', 'godown', 'price', 'report', 'startup_right',
        'header', 'cash_account'
    ))
);

CREATE INDEX IF NOT EXISTS idx_legacy_groups_tenant
    ON legacy_groups (tenant_id, legacy_group_code);
CREATE INDEX IF NOT EXISTS idx_group_rights_effective
    ON group_rights (tenant_id, role_id, permission, allowed);
CREATE INDEX IF NOT EXISTS idx_group_allowed_scopes_effective
    ON group_allowed_scopes (tenant_id, role_id, scope_kind, scope_key, allowed);

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'legacy_groups', 'group_rights', 'group_allowed_scopes'
    ] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', table_name);
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', table_name || '_tenant_scope', table_name);
        EXECUTE format(
            'CREATE POLICY %I ON %I USING (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid OR current_setting(''app.authenticating'', true) = ''true'') WITH CHECK (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid OR current_setting(''app.authenticating'', true) = ''true'')',
            table_name || '_tenant_scope', table_name
        );
    END LOOP;
END $$;

-- The four source Groups are represented for every existing tenant. No
-- memberships or rights are invented here; imported GroupRights rows and the
-- existing role_permissions rows remain the source of effective access.
SELECT set_config('app.authenticating', 'true', true);

INSERT INTO roles (tenant_id, code, name)
SELECT t.id, source.code, source.name
FROM tenants t
CROSS JOIN (VALUES
    ('ADMINISTRATOR', 'ADMINISTRATOR'),
    ('REMOTE', 'REMOTE'),
    ('SALES OFFICER', 'SALES OFFICER'),
    ('SHIFT INCHARGE', 'SHIFT INCHARGE')
) AS source(code, name)
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO legacy_groups (
    tenant_id, role_id, legacy_group_code, legacy_group_name
)
SELECT r.tenant_id, r.id, r.code, r.name
FROM roles r
WHERE r.code IN ('ADMINISTRATOR', 'REMOTE', 'SALES OFFICER', 'SHIFT INCHARGE')
ON CONFLICT (tenant_id, legacy_group_code) DO UPDATE
SET role_id = EXCLUDED.role_id,
    legacy_group_name = EXCLUDED.legacy_group_name,
    updated_at = now();

-- Bootstrap and future tenant provisioning must receive the same four
-- representational Groups, without assigning any operator or inventing rights.
CREATE OR REPLACE FUNCTION seed_legacy_groups_for_tenant()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    seeded record;
BEGIN
    PERFORM set_config('app.authenticating', 'true', true);
    FOR seeded IN
        SELECT * FROM (VALUES
            ('ADMINISTRATOR', 'ADMINISTRATOR'),
            ('REMOTE', 'REMOTE'),
            ('SALES OFFICER', 'SALES OFFICER'),
            ('SHIFT INCHARGE', 'SHIFT INCHARGE')
        ) AS source(code, name)
    LOOP
        INSERT INTO roles (tenant_id, code, name)
        VALUES (NEW.id, seeded.code, seeded.name)
        ON CONFLICT (tenant_id, code) DO NOTHING;

        INSERT INTO legacy_groups (
            tenant_id, role_id, legacy_group_code, legacy_group_name
        )
        SELECT NEW.id, r.id, r.code, r.name
        FROM roles r
        WHERE r.tenant_id = NEW.id AND r.code = seeded.code
        ON CONFLICT (tenant_id, legacy_group_code) DO UPDATE
        SET role_id = EXCLUDED.role_id,
            legacy_group_name = EXCLUDED.legacy_group_name,
            updated_at = now();
    END LOOP;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS tenants_seed_legacy_groups ON tenants;
CREATE TRIGGER tenants_seed_legacy_groups
AFTER INSERT ON tenants
FOR EACH ROW EXECUTE FUNCTION seed_legacy_groups_for_tenant();
