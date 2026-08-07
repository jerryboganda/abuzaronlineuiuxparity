-- Source-backed withholding-tax deductions from dbo.PurPayment.
--
-- Withholding is a payment deduction, not the purchase-line advance-tax
-- snapshot. Keep it separate so the report cannot silently reinterpret
-- advance tax as withholding.

CREATE TABLE IF NOT EXISTS historical_withholding_tax_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL,
    legacy_id text NOT NULL,
    payment_code text NOT NULL,
    purchase_invoice_code text NOT NULL DEFAULT '',
    supplier_legacy_id text NOT NULL DEFAULT '',
    occurred_at timestamptz NOT NULL,
    posted boolean NOT NULL DEFAULT false,
    account_code text NOT NULL DEFAULT '',
    taxable_base numeric(19, 4) NOT NULL DEFAULT 0,
    rate numeric(19, 4) NOT NULL DEFAULT 0,
    amount numeric(19, 4) NOT NULL DEFAULT 0,
    check_number text NOT NULL DEFAULT '',
    remarks text NOT NULL DEFAULT '',
    user_legacy_id text NOT NULL DEFAULT '',
    source_table text NOT NULL,
    source_table_row text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, legacy_id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_historical_withholding_tax_scope
    ON historical_withholding_tax_entries
       (tenant_id, branch_id, occurred_at DESC, purchase_invoice_code);
CREATE INDEX IF NOT EXISTS idx_historical_withholding_tax_supplier
    ON historical_withholding_tax_entries
       (tenant_id, branch_id, supplier_legacy_id, occurred_at DESC);

ALTER TABLE historical_withholding_tax_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE historical_withholding_tax_entries FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS historical_withholding_tax_entries_tenant_scope
    ON historical_withholding_tax_entries;
CREATE POLICY historical_withholding_tax_entries_tenant_scope
    ON historical_withholding_tax_entries
    USING (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    );

DROP POLICY IF EXISTS historical_withholding_tax_entries_branch_tenant_hardening
    ON historical_withholding_tax_entries;
CREATE POLICY historical_withholding_tax_entries_branch_tenant_hardening
    ON historical_withholding_tax_entries AS RESTRICTIVE FOR ALL
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
