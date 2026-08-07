-- Source-backed customer receipts and supplier payments from the reviewed
-- PowerBuilder payment streams. These rows are historical evidence, not a
-- synthetic replacement for the canonical posting workflow.

CREATE TABLE IF NOT EXISTS historical_party_payment_allocations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL,
    party_id uuid,
    party_legacy_id text NOT NULL DEFAULT '',
    counterparty_kind text NOT NULL CHECK (counterparty_kind IN ('customer', 'supplier')),
    source_document_id uuid,
    source_document_table text NOT NULL DEFAULT '',
    source_document_legacy_id text NOT NULL DEFAULT '',
    payment_code text NOT NULL DEFAULT '',
    payment_amount numeric(19, 4) NOT NULL DEFAULT 0,
    net_amount numeric(19, 4) NOT NULL DEFAULT 0,
    outstanding_amount numeric(19, 4) NOT NULL DEFAULT 0,
    occurred_at timestamptz NOT NULL,
    posted boolean NOT NULL DEFAULT false,
    payment_mode text NOT NULL DEFAULT '',
    account_code text NOT NULL DEFAULT '',
    payment_account_code text NOT NULL DEFAULT '',
    check_number text NOT NULL DEFAULT '',
    reference text NOT NULL DEFAULT '',
    remarks text NOT NULL DEFAULT '',
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

CREATE INDEX IF NOT EXISTS idx_historical_party_payment_scope
    ON historical_party_payment_allocations
       (tenant_id, branch_id, counterparty_kind, occurred_at DESC, id);
CREATE INDEX IF NOT EXISTS idx_historical_party_payment_party
    ON historical_party_payment_allocations
       (tenant_id, branch_id, party_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_historical_party_payment_document
    ON historical_party_payment_allocations
       (tenant_id, branch_id, source_document_legacy_id, occurred_at DESC);

ALTER TABLE historical_party_payment_allocations ENABLE ROW LEVEL SECURITY;
ALTER TABLE historical_party_payment_allocations FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS historical_party_payment_allocations_tenant_scope
    ON historical_party_payment_allocations;
CREATE POLICY historical_party_payment_allocations_tenant_scope
    ON historical_party_payment_allocations
    USING (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    );

DROP POLICY IF EXISTS historical_party_payment_allocations_branch_tenant_hardening
    ON historical_party_payment_allocations;
CREATE POLICY historical_party_payment_allocations_branch_tenant_hardening
    ON historical_party_payment_allocations AS RESTRICTIVE FOR ALL
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
