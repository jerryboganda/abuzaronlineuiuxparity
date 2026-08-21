-- Accounting vouchers post through business_documents so gl_journals can
-- keep its required source_document_id foreign key.

ALTER TABLE business_documents
    DROP CONSTRAINT IF EXISTS business_documents_kind_check;
ALTER TABLE business_documents
    ADD CONSTRAINT business_documents_kind_check CHECK (kind IN (
        'cash-sale', 'credit-sale', 'cash-return', 'credit-return',
        'open-cash-return', 'open-credit-return',
        'cash-sale-return', 'credit-sale-return', 'open-sale-return',
        'quotation', 'refused-sale', 'sale-order',
        'pack-purchase', 'loose-purchase', 'opening-purchase',
        'purchase-return', 'purchase-order', 'purchase-quotation',
        'receipt-voucher', 'payment-voucher', 'journal-voucher'
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
        'purchase-return', 'purchase-order', 'purchase-quotation',
        'receipt-voucher', 'payment-voucher', 'journal-voucher'
    ));
