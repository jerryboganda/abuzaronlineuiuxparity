-- Phase L tax configuration.
--
-- Tax configuration is operational master data, not a historical tax import.
-- Every row carries the branch scope so a branch-scoped application role
-- cannot read or mutate another branch's configuration.

CREATE TABLE IF NOT EXISTS tax_rates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL,
    tax_kind text NOT NULL CHECK (tax_kind IN ('gst', 'pct', 'advance')),
    code text NOT NULL,
    name text NOT NULL,
    rate numeric(9, 4) NOT NULL CHECK (rate >= 0 AND rate <= 100),
    inclusive boolean NOT NULL DEFAULT false,
    effective_from date NOT NULL,
    effective_to date,
    source_table text NOT NULL DEFAULT '',
    source_legacy_id text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, id),
    UNIQUE (tenant_id, branch_id, tax_kind, code),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    CHECK (effective_to IS NULL OR effective_to >= effective_from)
);

CREATE TABLE IF NOT EXISTS item_tax_assignments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL,
    item_id uuid NOT NULL,
    tax_rate_id uuid NOT NULL,
    effective_from date NOT NULL,
    effective_to date,
    source_table text NOT NULL DEFAULT '',
    source_legacy_id text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, id),
    UNIQUE (tenant_id, branch_id, item_id, tax_rate_id, effective_from),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, item_id) REFERENCES master_items(tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id, tax_rate_id)
        REFERENCES tax_rates(tenant_id, branch_id, id),
    CHECK (effective_to IS NULL OR effective_to >= effective_from)
);

CREATE TABLE IF NOT EXISTS party_tax_assignments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL,
    party_id uuid NOT NULL,
    tax_rate_id uuid NOT NULL,
    effective_from date NOT NULL,
    effective_to date,
    source_table text NOT NULL DEFAULT '',
    source_legacy_id text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, id),
    UNIQUE (tenant_id, branch_id, party_id, tax_rate_id, effective_from),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, party_id) REFERENCES master_parties(tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id, tax_rate_id)
        REFERENCES tax_rates(tenant_id, branch_id, id),
    CHECK (effective_to IS NULL OR effective_to >= effective_from)
);

CREATE INDEX IF NOT EXISTS idx_tax_rates_effective
    ON tax_rates (tenant_id, branch_id, tax_kind, effective_from, effective_to);
CREATE INDEX IF NOT EXISTS idx_item_tax_assignments_effective
    ON item_tax_assignments (tenant_id, branch_id, item_id, effective_from, effective_to);
CREATE INDEX IF NOT EXISTS idx_party_tax_assignments_effective
    ON party_tax_assignments (tenant_id, branch_id, party_id, effective_from, effective_to);

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'tax_rates', 'item_tax_assignments', 'party_tax_assignments'
    ] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', table_name);
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I',
            table_name || '_tenant_scope', table_name);
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I',
            table_name || '_branch_tenant_hardening', table_name);
        EXECUTE format(
            'CREATE POLICY %I ON %I
             USING (
                 current_setting(''app.authenticating'', true) = ''true''
                 OR tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid
             )
             WITH CHECK (
                 current_setting(''app.authenticating'', true) = ''true''
                 OR tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid
             )',
            table_name || '_tenant_scope', table_name
        );
        EXECUTE format(
            'CREATE POLICY %I ON %I AS RESTRICTIVE FOR ALL
             USING (
                 current_setting(''app.authenticating'', true) = ''true''
                 OR (
                     tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid
                     AND branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid
                 )
             )
             WITH CHECK (
                 current_setting(''app.authenticating'', true) = ''true''
                 OR (
                     tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid
                     AND branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid
                 )
             )',
            table_name || '_branch_tenant_hardening', table_name
        );
    END LOOP;
END $$;
