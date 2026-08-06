-- Abuzar Next tenancy and operational foundation.
-- The application sets app.tenant_id/app.branch_id in each transaction before access.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS tenants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    legal_name text NOT NULL,
    code text NOT NULL UNIQUE,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS branches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    code text NOT NULL,
    name text NOT NULL,
    timezone text NOT NULL DEFAULT 'Asia/Karachi',
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, code),
    UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS counters (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    branch_id uuid NOT NULL REFERENCES branches(id),
    code text NOT NULL,
    name text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, code),
    UNIQUE (tenant_id, branch_id, id),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    username text NOT NULL,
    display_name text NOT NULL,
    password_hash text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, username),
    UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    code text NOT NULL,
    name text NOT NULL,
    UNIQUE (tenant_id, code),
    UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS user_memberships (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role_id uuid NOT NULL REFERENCES roles(id),
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, role_id) REFERENCES roles(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS user_branch_assignments (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, branch_id),
    FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_counter_assignments (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    counter_id uuid NOT NULL REFERENCES counters(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, counter_id),
    FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, counter_id) REFERENCES counters(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS tenant_sequences (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    sequence_name text NOT NULL,
    next_value bigint NOT NULL DEFAULT 1 CHECK (next_value > 0),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, branch_id, sequence_name),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS shifts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    branch_id uuid NOT NULL REFERENCES branches(id),
    counter_id uuid NOT NULL REFERENCES counters(id),
    operator_id uuid NOT NULL REFERENCES users(id),
    opened_at timestamptz NOT NULL,
    closed_at timestamptz,
    status text NOT NULL CHECK (status IN ('open', 'closed', 'review')),
    opening_amount numeric(19, 4) NOT NULL DEFAULT 0,
    closing_amount numeric(19, 4),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id, counter_id) REFERENCES counters(tenant_id, branch_id, id),
    FOREIGN KEY (tenant_id, operator_id) REFERENCES users(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS audit_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    branch_id uuid REFERENCES branches(id),
    operator_id uuid REFERENCES users(id),
    action text NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, operator_id) REFERENCES users(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS sync_events (
    sequence_no bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
    event_id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    branch_id uuid NOT NULL REFERENCES branches(id),
    counter_id uuid REFERENCES counters(id),
    operator_id uuid REFERENCES users(id),
    aggregate text NOT NULL,
    aggregate_id uuid NOT NULL,
    idempotency_key text NOT NULL,
    schema_version integer NOT NULL DEFAULT 1,
    payload jsonb NOT NULL,
    occurred_at timestamptz NOT NULL,
    accepted_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, idempotency_key),
    UNIQUE (tenant_id, event_id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id, counter_id) REFERENCES counters(tenant_id, branch_id, id),
    FOREIGN KEY (tenant_id, operator_id) REFERENCES users(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS sync_cursors (
    branch_id uuid PRIMARY KEY REFERENCES branches(id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    cursor_value bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS conflict_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    branch_id uuid NOT NULL REFERENCES branches(id),
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    local_value jsonb NOT NULL,
    server_value jsonb NOT NULL,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved', 'dismissed')),
    resolution jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id)
);

-- Representative transaction tables establish the required scope and idempotency pattern.
-- Existing business tables will be added through subsequent migration waves.
CREATE TABLE IF NOT EXISTS sales_documents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    branch_id uuid NOT NULL REFERENCES branches(id),
    counter_id uuid NOT NULL REFERENCES counters(id),
    operator_id uuid NOT NULL REFERENCES users(id),
    shift_id uuid REFERENCES shifts(id),
    document_number text NOT NULL,
    status text NOT NULL CHECK (status IN ('draft', 'posted', 'voided')),
    total_amount numeric(19, 4) NOT NULL DEFAULT 0,
    occurred_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, document_number),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id, counter_id) REFERENCES counters(tenant_id, branch_id, id),
    FOREIGN KEY (tenant_id, operator_id) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, shift_id) REFERENCES shifts(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS inventory_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    branch_id uuid NOT NULL REFERENCES branches(id),
    source_event_id uuid NOT NULL REFERENCES sync_events(event_id),
    item_legacy_id text NOT NULL,
    quantity numeric(19, 4) NOT NULL,
    direction text NOT NULL CHECK (direction IN ('in', 'out', 'adjustment')),
    occurred_at timestamptz NOT NULL,
    UNIQUE (tenant_id, source_event_id, item_legacy_id),
    FOREIGN KEY (tenant_id, source_event_id) REFERENCES sync_events(tenant_id, event_id)
);

CREATE INDEX IF NOT EXISTS idx_branches_tenant ON branches (tenant_id);
CREATE INDEX IF NOT EXISTS idx_counters_branch ON counters (tenant_id, branch_id);
CREATE INDEX IF NOT EXISTS idx_users_tenant ON users (tenant_id);
CREATE INDEX IF NOT EXISTS idx_shifts_scope ON shifts (tenant_id, branch_id, counter_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_shifts_one_open ON shifts (tenant_id, branch_id, counter_id) WHERE status = 'open';
CREATE INDEX IF NOT EXISTS idx_audit_scope_time ON audit_events (tenant_id, branch_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_sync_events_branch_time ON sync_events (tenant_id, branch_id, accepted_at);
CREATE INDEX IF NOT EXISTS idx_conflicts_open ON conflict_records (tenant_id, branch_id, status);
CREATE INDEX IF NOT EXISTS idx_sales_scope_time ON sales_documents (tenant_id, branch_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_inventory_scope_time ON inventory_movements (tenant_id, branch_id, occurred_at DESC);

-- The tenant catalog uses its primary key as the scope key rather than a
-- separate tenant_id column.
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenants_scope ON tenants;
CREATE POLICY tenants_scope ON tenants
    USING (id = NULLIF(current_setting('app.tenant_id', true), '')::uuid OR current_setting('app.authenticating', true) = 'true')
    WITH CHECK (id = NULLIF(current_setting('app.tenant_id', true), '')::uuid OR current_setting('app.authenticating', true) = 'true');

-- Defense-in-depth tenant policies. The API must still authorize memberships and branches.
DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'branches', 'counters', 'users', 'roles', 'user_memberships',
        'user_branch_assignments', 'user_counter_assignments', 'shifts',
        'tenant_sequences', 'audit_events', 'sync_events', 'sync_cursors', 'conflict_records',
        'sales_documents', 'inventory_movements'
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
