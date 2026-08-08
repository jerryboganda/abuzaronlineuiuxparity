-- Phase H bounded return/reversal contract.
--
-- 022/023 introduced return names in two successive migrations, but the
-- latter check did not retain the canonical open-return names.  Keep the
-- compatibility names readable while making the four canonical return kinds
-- and their source/line relationships explicit.

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

ALTER TABLE business_documents
    DROP CONSTRAINT IF EXISTS business_documents_sale_return_source_check,
    DROP CONSTRAINT IF EXISTS business_documents_open_return_source_check,
    DROP CONSTRAINT IF EXISTS business_documents_neutral_source_check;
ALTER TABLE business_documents
    ADD CONSTRAINT business_documents_sale_return_source_check CHECK (
        kind NOT IN ('cash-return', 'credit-return')
        OR source_document_id IS NOT NULL
    ),
    ADD CONSTRAINT business_documents_open_return_source_check CHECK (
        kind NOT IN ('open-cash-return', 'open-credit-return')
        OR source_document_id IS NULL
    ),
    ADD CONSTRAINT business_documents_neutral_source_check CHECK (
        kind NOT IN ('quotation', 'refused-sale')
        OR source_document_id IS NULL
    );

ALTER TABLE business_document_lines
    ADD COLUMN IF NOT EXISTS source_line_id uuid;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'business_document_lines_source_line_scope_fkey'
    ) THEN
        ALTER TABLE business_document_lines
            ADD CONSTRAINT business_document_lines_source_line_scope_fkey
            FOREIGN KEY (tenant_id, source_line_id)
            REFERENCES business_document_lines(tenant_id, id);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'business_document_lines_source_line_unique'
    ) THEN
        ALTER TABLE business_document_lines
            ADD CONSTRAINT business_document_lines_source_line_unique
            UNIQUE (tenant_id, document_id, source_line_id);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS business_document_reversals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    source_document_id uuid NOT NULL,
    reversal_document_id uuid NOT NULL,
    reversal_kind text NOT NULL CHECK (reversal_kind IN ('cash-return', 'credit-return')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, id),
    UNIQUE (tenant_id, reversal_document_id),
    FOREIGN KEY (tenant_id, source_document_id)
        REFERENCES business_documents(tenant_id, id),
    FOREIGN KEY (tenant_id, reversal_document_id)
        REFERENCES business_documents(tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id)
        REFERENCES branches(tenant_id, id),
    CHECK (source_document_id <> reversal_document_id)
);

CREATE INDEX IF NOT EXISTS idx_business_document_reversals_source
    ON business_document_reversals (tenant_id, branch_id, source_document_id);

CREATE OR REPLACE FUNCTION validate_sale_return_source_025()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    source_kind text;
    source_status text;
    source_branch uuid;
    source_customer uuid;
BEGIN
    IF NEW.kind NOT IN ('cash-return', 'credit-return') THEN
        IF NEW.kind IN ('open-cash-return', 'open-credit-return',
                        'quotation', 'refused-sale')
           AND NEW.source_document_id IS NOT NULL THEN
            RAISE EXCEPTION '% cannot reference a source document', NEW.kind;
        END IF;
        RETURN NEW;
    END IF;

    IF NEW.source_document_id IS NULL THEN
        RAISE EXCEPTION '% requires a source document', NEW.kind;
    END IF;

    SELECT kind, status, branch_id, customer_id
      INTO source_kind, source_status, source_branch, source_customer
      FROM business_documents
     WHERE tenant_id = NEW.tenant_id AND id = NEW.source_document_id
     FOR SHARE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'sale return source document is not in the tenant';
    END IF;
    IF source_branch <> NEW.branch_id THEN
        RAISE EXCEPTION 'sale return source document is not in the branch';
    END IF;
    IF (NEW.kind = 'cash-return' AND source_kind <> 'cash-sale')
       OR (NEW.kind = 'credit-return' AND source_kind <> 'credit-sale') THEN
        RAISE EXCEPTION '% must reverse the matching cash or credit sale', NEW.kind;
    END IF;
    IF NEW.status = 'posted' AND source_status <> 'posted' THEN
        RAISE EXCEPTION 'sale return source document must be posted';
    END IF;
    IF NEW.kind = 'credit-return'
       AND (NEW.customer_id IS NULL OR NEW.customer_id IS DISTINCT FROM source_customer) THEN
        RAISE EXCEPTION 'credit return customer must match the source sale';
    END IF;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS business_documents_sale_return_source_025
    ON business_documents;
CREATE TRIGGER business_documents_sale_return_source_025
BEFORE INSERT OR UPDATE ON business_documents
FOR EACH ROW EXECUTE FUNCTION validate_sale_return_source_025();

CREATE OR REPLACE FUNCTION validate_sale_return_line_source_025()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    document_kind text;
    document_status text;
    source_document uuid;
    source_item uuid;
BEGIN
    SELECT kind, status, source_document_id
      INTO document_kind, document_status, source_document
      FROM business_documents
     WHERE tenant_id = NEW.tenant_id AND id = NEW.document_id;

    IF document_kind IN ('cash-return', 'credit-return') THEN
        IF NEW.source_line_id IS NULL AND document_status = 'posted' THEN
            RAISE EXCEPTION 'posted sale return line requires a source sale line';
        END IF;
        IF NEW.source_line_id IS NOT NULL THEN
            SELECT item_id
              INTO source_item
              FROM business_document_lines
             WHERE tenant_id = NEW.tenant_id
               AND id = NEW.source_line_id
               AND document_id = source_document
             FOR SHARE;
            IF NOT FOUND OR source_item IS DISTINCT FROM NEW.item_id THEN
                RAISE EXCEPTION 'sale return line source does not match the source sale item';
            END IF;
        END IF;
    ELSIF document_kind = 'purchase-return' THEN
        -- Historical/unlinked legacy purchase returns are an accepted state, so
        -- the posting requirement lives at the API boundary; the trigger only
        -- guarantees that a supplied source line is referentially consistent.
        IF NEW.source_line_id IS NOT NULL THEN
            SELECT item_id
              INTO source_item
              FROM business_document_lines
             WHERE tenant_id = NEW.tenant_id
               AND id = NEW.source_line_id
               AND document_id = source_document
              FOR SHARE;
            IF NOT FOUND OR source_item IS DISTINCT FROM NEW.item_id THEN
                RAISE EXCEPTION 'purchase return line source does not match the source purchase item';
            END IF;
        END IF;
    ELSIF NEW.source_line_id IS NOT NULL THEN
        RAISE EXCEPTION 'source sale line is only valid for a source-bound sale return';
    END IF;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS business_document_lines_sale_return_source_025
    ON business_document_lines;
CREATE TRIGGER business_document_lines_sale_return_source_025
BEFORE INSERT OR UPDATE ON business_document_lines
FOR EACH ROW EXECUTE FUNCTION validate_sale_return_line_source_025();

CREATE OR REPLACE FUNCTION maintain_sale_return_reversal_025()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.kind IN ('cash-return', 'credit-return') AND NEW.status = 'posted' THEN
        INSERT INTO business_document_reversals
            (tenant_id, branch_id, source_document_id, reversal_document_id, reversal_kind)
        VALUES
            (NEW.tenant_id, NEW.branch_id, NEW.source_document_id, NEW.id, NEW.kind)
        ON CONFLICT (tenant_id, reversal_document_id) DO UPDATE
            SET source_document_id = EXCLUDED.source_document_id,
                branch_id = EXCLUDED.branch_id,
                reversal_kind = EXCLUDED.reversal_kind;
    ELSE
        DELETE FROM business_document_reversals
         WHERE tenant_id = NEW.tenant_id AND reversal_document_id = NEW.id;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS business_documents_sale_return_reversal_025
    ON business_documents;
CREATE TRIGGER business_documents_sale_return_reversal_025
AFTER INSERT OR UPDATE ON business_documents
FOR EACH ROW EXECUTE FUNCTION maintain_sale_return_reversal_025();

ALTER TABLE business_document_reversals ENABLE ROW LEVEL SECURITY;
ALTER TABLE business_document_reversals FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS business_document_reversals_tenant_scope
    ON business_document_reversals;
CREATE POLICY business_document_reversals_tenant_scope
ON business_document_reversals
USING (
    current_setting('app.authenticating', true) = 'true'
    OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
)
WITH CHECK (
    current_setting('app.authenticating', true) = 'true'
    OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
);
DROP POLICY IF EXISTS business_document_reversals_branch_tenant_hardening
    ON business_document_reversals;
CREATE POLICY business_document_reversals_branch_tenant_hardening
ON business_document_reversals AS RESTRICTIVE
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
