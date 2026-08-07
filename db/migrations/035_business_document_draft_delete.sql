-- Preserve the captured File > Delete workflow without destroying canonical
-- document history. Only draft documents may be soft-deleted; posted
-- documents continue through the audited void/reversal lifecycle.

ALTER TABLE business_documents
    ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

ALTER TABLE business_document_revisions
    DROP CONSTRAINT IF EXISTS business_document_revisions_action_check;
ALTER TABLE business_document_revisions
    ADD CONSTRAINT business_document_revisions_action_check
    CHECK (action IN ('save', 'post', 'save-and-post', 'void', 'delete'));

ALTER TABLE command_receipts
    DROP CONSTRAINT IF EXISTS command_receipts_action_check;
ALTER TABLE command_receipts
    ADD CONSTRAINT command_receipts_action_check
    CHECK (action IN ('save', 'post', 'save-and-post', 'void', 'delete'));

CREATE INDEX IF NOT EXISTS idx_business_documents_active_scope
    ON business_documents (tenant_id, branch_id, kind, occurred_at DESC)
    WHERE deleted_at IS NULL;
