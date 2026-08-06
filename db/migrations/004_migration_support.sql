-- Migration bookkeeping. These records make legacy-to-new mapping and
-- reconciliation auditable without modifying the source SQL Server database.

CREATE TABLE IF NOT EXISTS legacy_id_mappings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source_system text NOT NULL DEFAULT 'sqlserver',
    source_schema text NOT NULL,
    source_table text NOT NULL,
    legacy_id text NOT NULL,
    target_table text NOT NULL,
    target_id text,
    status text NOT NULL DEFAULT 'mapped' CHECK (status IN ('mapped', 'exception', 'skipped')),
    note text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, source_system, source_schema, source_table, legacy_id)
);

CREATE TABLE IF NOT EXISTS migration_exceptions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source_schema text NOT NULL,
    source_table text NOT NULL,
    legacy_id text,
    reason_code text NOT NULL,
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved', 'ignored')),
    created_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz
);

CREATE TABLE IF NOT EXISTS migration_reconciliation (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source_schema text NOT NULL,
    source_table text NOT NULL,
    source_count bigint,
    target_table text NOT NULL,
    target_count bigint,
    source_total numeric(28, 8),
    target_total numeric(28, 8),
    status text NOT NULL CHECK (status IN ('matched', 'mismatched', 'missing_target', 'exception')),
    generated_at timestamptz NOT NULL DEFAULT now()
);
