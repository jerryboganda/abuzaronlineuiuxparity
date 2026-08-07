-- Phase T: posted-document voiding through append-only compensating reversals.
--
-- A legacy Void action must not delete or mutate stock/GL history.  The API
-- changes the document status to void only in the same transaction that
-- writes inverse stock movements, a reversing GL journal, and a reversing
-- party-ledger entry.  A source document can have at most one such reversal.

ALTER TABLE gl_journals
    ADD COLUMN IF NOT EXISTS reversal_of_journal_id uuid;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'gl_journals_reversal_of_journal_scope_fkey'
    ) THEN
        ALTER TABLE gl_journals
            ADD CONSTRAINT gl_journals_reversal_of_journal_scope_fkey
            FOREIGN KEY (tenant_id, branch_id, reversal_of_journal_id)
            REFERENCES gl_journals (tenant_id, branch_id, id);
    END IF;
END $$;

-- The original schema allowed only one journal per source document.  Keep
-- that invariant for ordinary journals while permitting one void-reversal
-- journal for the same source document.
DO $$
DECLARE
    constraint_name text;
BEGIN
    FOR constraint_name IN
        SELECT conname
        FROM pg_constraint
        WHERE conrelid = 'gl_journals'::regclass
          AND contype = 'u'
          AND pg_get_constraintdef(oid) LIKE '%source_document_id%'
    LOOP
        EXECUTE format('ALTER TABLE gl_journals DROP CONSTRAINT IF EXISTS %I', constraint_name);
    END LOOP;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_gl_journals_primary_source_document
    ON gl_journals (tenant_id, branch_id, source_document_id)
    WHERE kind <> 'void-reversal';
CREATE UNIQUE INDEX IF NOT EXISTS uq_gl_journals_void_source_document
    ON gl_journals (tenant_id, branch_id, source_document_id)
    WHERE kind = 'void-reversal';

ALTER TABLE party_ledger_entries
    DROP CONSTRAINT IF EXISTS party_ledger_entries_entry_kind_check;
ALTER TABLE party_ledger_entries
    ADD CONSTRAINT party_ledger_entries_entry_kind_check
        CHECK (entry_kind IN ('sale', 'sale-return', 'purchase', 'purchase-return', 'voucher', 'void')) NOT VALID;

DO $$
DECLARE
    constraint_name text;
BEGIN
    FOR constraint_name IN
        SELECT conname
        FROM pg_constraint
        WHERE conrelid = 'party_ledger_entries'::regclass
          AND contype = 'u'
          AND pg_get_constraintdef(oid) LIKE '%source_document_id%'
    LOOP
        EXECUTE format('ALTER TABLE party_ledger_entries DROP CONSTRAINT IF EXISTS %I', constraint_name);
    END LOOP;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_party_ledger_primary_source_document
    ON party_ledger_entries (tenant_id, branch_id, source_document_id)
    WHERE entry_kind <> 'void';
CREATE UNIQUE INDEX IF NOT EXISTS uq_party_ledger_void_source_document
    ON party_ledger_entries (tenant_id, branch_id, source_document_id)
    WHERE entry_kind = 'void';

CREATE TABLE IF NOT EXISTS business_document_void_reversals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    source_document_id uuid NOT NULL,
    reversal_event_id uuid NOT NULL,
    reversal_journal_id uuid NOT NULL,
    reason text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, id),
    UNIQUE (tenant_id, branch_id, source_document_id),
    UNIQUE (tenant_id, branch_id, reversal_event_id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, source_document_id) REFERENCES business_documents(tenant_id, id),
    FOREIGN KEY (tenant_id, reversal_event_id) REFERENCES sync_events(tenant_id, event_id),
    FOREIGN KEY (tenant_id, branch_id, reversal_journal_id) REFERENCES gl_journals(tenant_id, branch_id, id)
);

CREATE INDEX IF NOT EXISTS idx_business_document_void_reversals_source
    ON business_document_void_reversals (tenant_id, branch_id, source_document_id);

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['business_document_void_reversals'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', table_name);
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', table_name || '_tenant_scope', table_name);
        EXECUTE format(
            'CREATE POLICY %I ON %I USING (current_setting(''app.authenticating'', true) = ''true'' OR tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid) WITH CHECK (current_setting(''app.authenticating'', true) = ''true'' OR tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid)',
            table_name || '_tenant_scope', table_name
        );
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', table_name || '_branch_scope', table_name);
        EXECUTE format(
            'CREATE POLICY %I ON %I AS RESTRICTIVE USING (current_setting(''app.authenticating'', true) = ''true'' OR (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid AND (NULLIF(current_setting(''app.allow_tenant_scope'', true), '''') = ''true'' OR branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid))) WITH CHECK (current_setting(''app.authenticating'', true) = ''true'' OR (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid AND (NULLIF(current_setting(''app.allow_tenant_scope'', true), '''') = ''true'' OR branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid)))',
            table_name || '_branch_scope', table_name
        );
    END LOOP;
END $$;
