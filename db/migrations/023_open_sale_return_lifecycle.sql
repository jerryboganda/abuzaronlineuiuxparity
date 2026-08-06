-- Distinct canonical document kinds for the legacy Open Sale Return leaves.
-- Open returns intentionally have no source invoice; stock is received into an
-- explicit godown and the normal sale-return finance projection reverses the
-- settlement/revenue/tax at the returned line cost.

ALTER TABLE business_documents
    DROP CONSTRAINT IF EXISTS business_documents_kind_check;
ALTER TABLE business_documents
    ADD CONSTRAINT business_documents_kind_check CHECK (kind IN (
        'cash-sale', 'credit-sale', 'cash-return', 'credit-return',
        'open-cash-return', 'open-credit-return',
        'cash-sale-return', 'credit-sale-return', 'open-sale-return',
        'quotation', 'refused-sale', 'sale-order',
        'pack-purchase', 'loose-purchase', 'opening-purchase',
        'purchase-return', 'purchase-order', 'purchase-quotation'
    ));

ALTER TABLE command_receipts
    DROP CONSTRAINT IF EXISTS command_receipts_kind_check;
ALTER TABLE command_receipts
    ADD CONSTRAINT command_receipts_kind_check CHECK (kind IN (
        'cash-sale', 'credit-sale', 'cash-return', 'credit-return',
        'open-cash-return', 'open-credit-return',
        'cash-sale-return', 'credit-sale-return', 'open-sale-return',
        'quotation', 'refused-sale', 'sale-order',
        'pack-purchase', 'loose-purchase', 'opening-purchase',
        'purchase-return', 'purchase-order', 'purchase-quotation'
    ));

CREATE INDEX IF NOT EXISTS idx_business_documents_open_sale_returns
    ON business_documents (tenant_id, branch_id, kind, status, occurred_at);
