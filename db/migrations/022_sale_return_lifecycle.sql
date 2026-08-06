-- Canonical cash/credit sale-return lifecycle.
-- Returns restore the source sale's allocated batches and reverse its
-- settlement/revenue/tax/COGS projections in the same transaction.

ALTER TABLE business_documents
    DROP CONSTRAINT IF EXISTS business_documents_kind_check;
ALTER TABLE business_documents
    ADD CONSTRAINT business_documents_kind_check CHECK (kind IN (
        'cash-sale', 'credit-sale', 'cash-return', 'credit-return',
        'cash-sale-return', 'credit-sale-return', 'open-sale-return',
        'quotation', 'refused-sale', 'sale-order',
        'pack-purchase', 'loose-purchase', 'opening-purchase',
        'purchase-return', 'purchase-order', 'purchase-quotation'
    )) NOT VALID;

ALTER TABLE command_receipts
    DROP CONSTRAINT IF EXISTS command_receipts_kind_check;
ALTER TABLE command_receipts
    ADD CONSTRAINT command_receipts_kind_check CHECK (kind IN (
        'cash-sale', 'credit-sale', 'cash-return', 'credit-return',
        'cash-sale-return', 'credit-sale-return', 'open-sale-return',
        'quotation', 'refused-sale', 'sale-order',
        'pack-purchase', 'loose-purchase', 'opening-purchase',
        'purchase-return', 'purchase-order', 'purchase-quotation'
    )) NOT VALID;

ALTER TABLE party_ledger_entries
    DROP CONSTRAINT IF EXISTS party_ledger_entries_entry_kind_check;

ALTER TABLE party_ledger_entries
    ADD CONSTRAINT party_ledger_entries_entry_kind_check
        CHECK (entry_kind IN ('sale', 'sale-return', 'purchase', 'purchase-return', 'voucher')) NOT VALID;

CREATE INDEX IF NOT EXISTS idx_business_documents_sale_returns
    ON business_documents (tenant_id, branch_id, source_document_id, kind, status);

CREATE INDEX IF NOT EXISTS idx_stock_ledger_sale_return_source
    ON stock_ledger (tenant_id, branch_id, source_document_id, direction);
