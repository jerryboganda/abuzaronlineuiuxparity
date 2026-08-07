-- Phase F: make the captured Item Form's unposted-transaction report bounded
-- when the target contains a large set of draft business documents.

CREATE INDEX IF NOT EXISTS idx_business_document_lines_item_scope_043
    ON business_document_lines (tenant_id, branch_id, item_id, document_id, line_number);
