-- Phase I purchase vertical slice.
--
-- Existing sale rows remain valid. Purchase-specific fields are nullable for
-- existing documents and are enforced by the service at the transition where
-- they become required (posting).

ALTER TABLE business_documents
    ADD COLUMN IF NOT EXISTS supplier_id uuid,
    ADD COLUMN IF NOT EXISTS source_document_id uuid,
    ADD COLUMN IF NOT EXISTS source_document_number text NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'business_documents_supplier_scope_fkey'
    ) THEN
        ALTER TABLE business_documents
            ADD CONSTRAINT business_documents_supplier_scope_fkey
            FOREIGN KEY (tenant_id, supplier_id)
            REFERENCES master_parties(tenant_id, id);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'business_documents_source_document_scope_fkey'
    ) THEN
        ALTER TABLE business_documents
            ADD CONSTRAINT business_documents_source_document_scope_fkey
            FOREIGN KEY (tenant_id, source_document_id)
            REFERENCES business_documents(tenant_id, id);
    END IF;
END $$;

ALTER TABLE business_documents
    DROP CONSTRAINT IF EXISTS business_documents_kind_check;
ALTER TABLE business_documents
    ADD CONSTRAINT business_documents_kind_check CHECK (kind IN (
        'cash-sale', 'credit-sale',
        'quotation', 'refused-sale',
        'pack-purchase', 'loose-purchase', 'opening-purchase',
        'purchase-return', 'purchase-order'
    )) NOT VALID;

ALTER TABLE command_receipts
    DROP CONSTRAINT IF EXISTS command_receipts_kind_check;
ALTER TABLE command_receipts
    ADD CONSTRAINT command_receipts_kind_check CHECK (kind IN (
        'cash-sale', 'credit-sale',
        'quotation', 'refused-sale',
        'pack-purchase', 'loose-purchase', 'opening-purchase',
        'purchase-return', 'purchase-order'
    )) NOT VALID;

ALTER TABLE business_document_lines
    ADD COLUMN IF NOT EXISTS batch_number text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS expiry_date date,
    ADD COLUMN IF NOT EXISTS unit_cost numeric(19, 4) NOT NULL DEFAULT 0
        CHECK (unit_cost >= 0),
    ADD COLUMN IF NOT EXISTS gst_rate numeric(9, 4) NOT NULL DEFAULT 0
        CHECK (gst_rate >= 0),
    ADD COLUMN IF NOT EXISTS pct_rate numeric(9, 4) NOT NULL DEFAULT 0
        CHECK (pct_rate >= 0),
    ADD COLUMN IF NOT EXISTS advance_tax_rate numeric(9, 4) NOT NULL DEFAULT 0
        CHECK (advance_tax_rate >= 0),
    ADD COLUMN IF NOT EXISTS tax_amount numeric(19, 4) NOT NULL DEFAULT 0
        CHECK (tax_amount >= 0);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'business_documents_purchase_supplier_check'
    ) THEN
        ALTER TABLE business_documents
            ADD CONSTRAINT business_documents_purchase_supplier_check
            CHECK (
                kind NOT IN (
                    'pack-purchase', 'loose-purchase', 'opening-purchase',
                    'purchase-return', 'purchase-order'
                ) OR supplier_id IS NOT NULL
            );
    END IF;
END $$;

-- Payables are deliberately explicit chart configuration. A tenant with
-- either account disabled/missing is rejected by the posting service rather
-- than receiving a fabricated success response.
INSERT INTO finance_accounts (tenant_id, category_code, system_key, code, name)
SELECT t.id, source.category_code, source.system_key, source.code, source.name
FROM tenants t
CROSS JOIN (VALUES
    ('liability', 'accounts_payable', '2000', 'Accounts payable'),
    ('tax', 'input_tax', '2200', 'Input tax recoverable')
) AS source(category_code, system_key, code, name)
ON CONFLICT (tenant_id, system_key) DO NOTHING;

CREATE OR REPLACE FUNCTION seed_purchase_finance_accounts_for_tenant()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM set_config('app.authenticating', 'true', true);
    INSERT INTO finance_accounts (tenant_id, category_code, system_key, code, name)
    VALUES
        (NEW.id, 'liability', 'accounts_payable', '2000', 'Accounts payable'),
        (NEW.id, 'tax', 'input_tax', '2200', 'Input tax recoverable')
    ON CONFLICT (tenant_id, system_key) DO NOTHING;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS tenants_seed_purchase_finance_accounts ON tenants;
CREATE TRIGGER tenants_seed_purchase_finance_accounts
AFTER INSERT ON tenants
FOR EACH ROW EXECUTE FUNCTION seed_purchase_finance_accounts_for_tenant();

ALTER TABLE party_ledger_entries
    DROP CONSTRAINT IF EXISTS party_ledger_entries_counterparty_kind_check,
    DROP CONSTRAINT IF EXISTS party_ledger_entries_entry_kind_check,
    DROP CONSTRAINT IF EXISTS party_ledger_entries_party_check,
    DROP CONSTRAINT IF EXISTS party_ledger_entries_check1;
ALTER TABLE party_ledger_entries
    ADD CONSTRAINT party_ledger_entries_counterparty_kind_check
        CHECK (counterparty_kind IN ('customer', 'supplier', 'cash')) NOT VALID,
    ADD CONSTRAINT party_ledger_entries_entry_kind_check
        CHECK (entry_kind IN ('sale', 'purchase', 'purchase-return', 'voucher')) NOT VALID,
    ADD CONSTRAINT party_ledger_entries_party_check
        CHECK (
            (counterparty_kind IN ('customer', 'supplier') AND party_id IS NOT NULL)
            OR counterparty_kind = 'cash'
        ) NOT VALID;

CREATE INDEX IF NOT EXISTS idx_business_documents_supplier
    ON business_documents (tenant_id, branch_id, supplier_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_business_documents_source
    ON business_documents (tenant_id, branch_id, source_document_id);
CREATE INDEX IF NOT EXISTS idx_business_document_lines_batch
    ON business_document_lines (tenant_id, item_id, batch_number, expiry_date);
