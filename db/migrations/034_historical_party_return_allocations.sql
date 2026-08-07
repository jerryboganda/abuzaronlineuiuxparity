-- Source-backed sale/purchase return allocations from the reviewed PowerBuilder
-- allocation streams. These rows remain distinct from receipts, payments, and
-- canonical document postings until the legacy balance semantics are replayed.

CREATE TABLE IF NOT EXISTS historical_party_return_allocations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL,
    party_id uuid,
    party_legacy_id text NOT NULL DEFAULT '',
    counterparty_kind text NOT NULL CHECK (counterparty_kind IN ('customer', 'supplier')),
    return_kind text NOT NULL CHECK (return_kind IN ('sale', 'purchase')),
    return_document_id uuid,
    return_source_table text NOT NULL DEFAULT '',
    return_source_legacy_id text NOT NULL DEFAULT '',
    source_document_id uuid,
    source_document_table text NOT NULL DEFAULT '',
    source_document_legacy_id text NOT NULL DEFAULT '',
    allocation_code text NOT NULL DEFAULT '',
    allocation_row_id text NOT NULL DEFAULT '',
    allocation_amount numeric(19, 4) NOT NULL DEFAULT 0,
    outstanding_amount numeric(19, 4) NOT NULL DEFAULT 0,
    debit_amount numeric(19, 4) NOT NULL DEFAULT 0,
    credit_amount numeric(19, 4) NOT NULL DEFAULT 0,
    occurred_at timestamptz NOT NULL,
    posted boolean NOT NULL DEFAULT false,
    user_legacy_id text NOT NULL DEFAULT '',
    source_table text NOT NULL,
    source_table_row text NOT NULL,
    source_legacy_id text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, source_legacy_id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_historical_party_return_allocation_scope
    ON historical_party_return_allocations
       (tenant_id, branch_id, counterparty_kind, occurred_at DESC, id);
CREATE INDEX IF NOT EXISTS idx_historical_party_return_allocation_party
    ON historical_party_return_allocations
       (tenant_id, branch_id, party_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_historical_party_return_allocation_document
    ON historical_party_return_allocations
       (tenant_id, branch_id, source_document_legacy_id, occurred_at DESC);

ALTER TABLE historical_party_return_allocations ENABLE ROW LEVEL SECURITY;
ALTER TABLE historical_party_return_allocations FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS historical_party_return_allocations_tenant_scope
    ON historical_party_return_allocations;
CREATE POLICY historical_party_return_allocations_tenant_scope
    ON historical_party_return_allocations
    USING (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    );

DROP POLICY IF EXISTS historical_party_return_allocations_branch_tenant_hardening
    ON historical_party_return_allocations;
CREATE POLICY historical_party_return_allocations_branch_tenant_hardening
    ON historical_party_return_allocations AS RESTRICTIVE FOR ALL
    USING (
        current_setting('app.authenticating', true) = 'true'
        OR (
            tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
            AND (
                NULLIF(current_setting('app.allow_tenant_scope', true), '') = 'true'
                OR branch_id = NULLIF(current_setting('app.branch_id', true), '')::uuid
            )
        )
    )
    WITH CHECK (
        current_setting('app.authenticating', true) = 'true'
        OR (
            tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
            AND (
                NULLIF(current_setting('app.allow_tenant_scope', true), '') = 'true'
                OR branch_id = NULLIF(current_setting('app.branch_id', true), '')::uuid
            )
        )
    );
