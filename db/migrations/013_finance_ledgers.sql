-- Phase K financial core.
--
-- This is a posting schema, not a historical-ledger import.  Account
-- balances are derived from immutable journals and party entries; this
-- migration deliberately creates no opening or legacy balance rows.

CREATE UNIQUE INDEX IF NOT EXISTS uq_master_parties_tenant_id
    ON master_parties (tenant_id, id);

CREATE TABLE IF NOT EXISTS finance_account_categories (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    code text NOT NULL CHECK (code IN (
        'asset', 'liability', 'equity', 'revenue', 'expense', 'tax'
    )),
    name text NOT NULL,
    normal_balance text NOT NULL CHECK (normal_balance IN ('debit', 'credit')),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, code)
);

CREATE TABLE IF NOT EXISTS finance_accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    category_code text NOT NULL,
    system_key text,
    code text NOT NULL,
    name text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code),
    UNIQUE (tenant_id, system_key),
    FOREIGN KEY (tenant_id, category_code)
        REFERENCES finance_account_categories(tenant_id, code)
);

CREATE TABLE IF NOT EXISTS gl_journals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    source_event_id uuid NOT NULL,
    source_document_id uuid NOT NULL,
    kind text NOT NULL,
    status text NOT NULL DEFAULT 'posted' CHECK (status = 'posted'),
    description text NOT NULL DEFAULT '',
    posted_at timestamptz NOT NULL,
    total_debit numeric(19, 4) NOT NULL CHECK (total_debit > 0),
    total_credit numeric(19, 4) NOT NULL CHECK (total_credit > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, id),
    UNIQUE (tenant_id, branch_id, source_event_id),
    UNIQUE (tenant_id, branch_id, source_document_id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, source_event_id)
        REFERENCES sync_events(tenant_id, event_id),
    FOREIGN KEY (tenant_id, source_document_id)
        REFERENCES business_documents(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS gl_lines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    journal_id uuid NOT NULL,
    line_number integer NOT NULL CHECK (line_number > 0),
    account_id uuid NOT NULL,
    party_id uuid,
    debit_amount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (debit_amount >= 0),
    credit_amount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (credit_amount >= 0),
    memo text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, id),
    UNIQUE (tenant_id, journal_id, line_number),
    CHECK (
        (debit_amount > 0 AND credit_amount = 0)
        OR (credit_amount > 0 AND debit_amount = 0)
    ),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id, journal_id)
        REFERENCES gl_journals(tenant_id, branch_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, account_id)
        REFERENCES finance_accounts(tenant_id, id),
    FOREIGN KEY (tenant_id, party_id)
        REFERENCES master_parties(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS party_ledger_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    party_id uuid,
    counterparty_kind text NOT NULL CHECK (counterparty_kind IN ('customer', 'cash')),
    source_event_id uuid NOT NULL,
    source_document_id uuid NOT NULL,
    entry_kind text NOT NULL CHECK (entry_kind IN ('sale', 'voucher')),
    debit_amount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (debit_amount >= 0),
    credit_amount numeric(19, 4) NOT NULL DEFAULT 0 CHECK (credit_amount >= 0),
    balance_after numeric(19, 4),
    occurred_at timestamptz NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, id),
    UNIQUE (tenant_id, branch_id, source_event_id),
    UNIQUE (tenant_id, branch_id, source_document_id),
    CHECK (
        (debit_amount > 0 AND credit_amount = 0)
        OR (credit_amount > 0 AND debit_amount = 0)
    ),
    CHECK (
        (counterparty_kind = 'customer' AND party_id IS NOT NULL)
        OR counterparty_kind = 'cash'
    ),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, source_event_id)
        REFERENCES sync_events(tenant_id, event_id),
    FOREIGN KEY (tenant_id, source_document_id)
        REFERENCES business_documents(tenant_id, id),
    FOREIGN KEY (tenant_id, party_id)
        REFERENCES master_parties(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS party_ledger_balances (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    party_id uuid NOT NULL,
    debit_total numeric(19, 4) NOT NULL DEFAULT 0 CHECK (debit_total >= 0),
    credit_total numeric(19, 4) NOT NULL DEFAULT 0 CHECK (credit_total >= 0),
    balance numeric(19, 4) NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, branch_id, party_id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, party_id) REFERENCES master_parties(tenant_id, id)
);

-- Voucher storage is intentionally a placeholder for the later voucher
-- workflow.  Purchases/payables are not posted by this phase.
CREATE TABLE IF NOT EXISTS voucher_categories (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    code text NOT NULL,
    name text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, code)
);

CREATE TABLE IF NOT EXISTS voucher_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES branches(id),
    category_code text NOT NULL,
    source_event_id uuid,
    source_document_id uuid,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'posted', 'void')),
    amount numeric(19, 4) NOT NULL CHECK (amount > 0),
    description text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, branch_id, id),
    UNIQUE (tenant_id, branch_id, source_event_id),
    UNIQUE (tenant_id, branch_id, source_document_id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, category_code)
        REFERENCES voucher_categories(tenant_id, code),
    FOREIGN KEY (tenant_id, source_event_id)
        REFERENCES sync_events(tenant_id, event_id),
    FOREIGN KEY (tenant_id, source_document_id)
        REFERENCES business_documents(tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_gl_journals_scope
    ON gl_journals (tenant_id, branch_id, posted_at DESC);
CREATE INDEX IF NOT EXISTS idx_gl_lines_journal
    ON gl_lines (tenant_id, branch_id, journal_id, line_number);
CREATE INDEX IF NOT EXISTS idx_party_ledger_scope
    ON party_ledger_entries (tenant_id, branch_id, party_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_voucher_entries_scope
    ON voucher_entries (tenant_id, branch_id, created_at DESC);

CREATE OR REPLACE FUNCTION reject_finance_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'posted finance records are immutable';
END;
$$;

DROP TRIGGER IF EXISTS gl_journals_immutable ON gl_journals;
CREATE TRIGGER gl_journals_immutable
BEFORE UPDATE OR DELETE ON gl_journals
FOR EACH ROW EXECUTE FUNCTION reject_finance_mutation();

DROP TRIGGER IF EXISTS gl_lines_immutable ON gl_lines;
CREATE TRIGGER gl_lines_immutable
BEFORE UPDATE OR DELETE ON gl_lines
FOR EACH ROW EXECUTE FUNCTION reject_finance_mutation();

DROP TRIGGER IF EXISTS party_ledger_entries_immutable ON party_ledger_entries;
CREATE TRIGGER party_ledger_entries_immutable
BEFORE UPDATE OR DELETE ON party_ledger_entries
FOR EACH ROW EXECUTE FUNCTION reject_finance_mutation();

CREATE OR REPLACE FUNCTION assert_balanced_gl_journal()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    journal_tenant uuid;
    journal_branch uuid;
    target_journal uuid;
    journal_debit numeric(19, 4);
    journal_credit numeric(19, 4);
    line_debit numeric(19, 4);
    line_credit numeric(19, 4);
BEGIN
    journal_tenant := COALESCE(NEW.tenant_id, OLD.tenant_id);
    journal_branch := COALESCE(NEW.branch_id, OLD.branch_id);
    IF TG_TABLE_NAME = 'gl_journals' THEN
        target_journal := COALESCE(NEW.id, OLD.id);
    ELSE
        target_journal := COALESCE(NEW.journal_id, OLD.journal_id);
    END IF;
    SELECT total_debit, total_credit
      INTO journal_debit, journal_credit
      FROM gl_journals
     WHERE tenant_id = journal_tenant
       AND branch_id = journal_branch
       AND id = target_journal;
    SELECT COALESCE(SUM(debit_amount), 0), COALESCE(SUM(credit_amount), 0)
      INTO line_debit, line_credit
      FROM gl_lines
     WHERE tenant_id = journal_tenant
       AND branch_id = journal_branch
       AND journal_id = target_journal;
    IF journal_debit IS NULL
       OR journal_credit IS NULL
       OR journal_debit <> journal_credit
       OR line_debit <> line_credit
       OR line_debit <> journal_debit
       OR line_credit <> journal_credit
    THEN
        RAISE EXCEPTION 'GL journal % is unbalanced', target_journal;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS gl_journals_balance_check ON gl_journals;
CREATE CONSTRAINT TRIGGER gl_journals_balance_check
AFTER INSERT OR UPDATE ON gl_journals
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION assert_balanced_gl_journal();

DROP TRIGGER IF EXISTS gl_lines_balance_check ON gl_lines;
CREATE CONSTRAINT TRIGGER gl_lines_balance_check
AFTER INSERT OR UPDATE OR DELETE ON gl_lines
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION assert_balanced_gl_journal();

CREATE OR REPLACE FUNCTION seed_finance_chart_for_tenant()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM set_config('app.authenticating', 'true', true);

    INSERT INTO finance_account_categories (tenant_id, code, name, normal_balance)
    VALUES
        (NEW.id, 'asset', 'Asset', 'debit'),
        (NEW.id, 'liability', 'Liability', 'credit'),
        (NEW.id, 'equity', 'Equity', 'credit'),
        (NEW.id, 'revenue', 'Revenue', 'credit'),
        (NEW.id, 'expense', 'Expense', 'debit'),
        (NEW.id, 'tax', 'Tax payable', 'credit')
    ON CONFLICT (tenant_id, code) DO NOTHING;

    INSERT INTO finance_accounts
        (tenant_id, category_code, system_key, code, name)
    VALUES
        (NEW.id, 'asset', 'cash', '1000', 'Cash'),
        (NEW.id, 'asset', 'accounts_receivable', '1100', 'Accounts receivable'),
        (NEW.id, 'asset', 'inventory', '1200', 'Inventory'),
        (NEW.id, 'tax', 'output_tax', '2100', 'Output tax payable'),
        (NEW.id, 'revenue', 'sales_revenue', '4000', 'Sales revenue'),
        (NEW.id, 'expense', 'cogs', '5000', 'Cost of goods sold')
    ON CONFLICT (tenant_id, system_key) DO NOTHING;

    INSERT INTO voucher_categories (tenant_id, code, name)
    VALUES
        (NEW.id, 'receipt', 'Receipt'),
        (NEW.id, 'payment', 'Payment'),
        (NEW.id, 'journal', 'Journal')
    ON CONFLICT (tenant_id, code) DO NOTHING;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS tenants_seed_finance_chart ON tenants;
CREATE TRIGGER tenants_seed_finance_chart
AFTER INSERT ON tenants
FOR EACH ROW EXECUTE FUNCTION seed_finance_chart_for_tenant();

INSERT INTO finance_account_categories (tenant_id, code, name, normal_balance)
SELECT t.id, source.code, source.name, source.normal_balance
FROM tenants t
CROSS JOIN (VALUES
    ('asset', 'Asset', 'debit'),
    ('liability', 'Liability', 'credit'),
    ('equity', 'Equity', 'credit'),
    ('revenue', 'Revenue', 'credit'),
    ('expense', 'Expense', 'debit'),
    ('tax', 'Tax payable', 'credit')
) AS source(code, name, normal_balance)
ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO finance_accounts (tenant_id, category_code, system_key, code, name)
SELECT t.id, source.category_code, source.system_key, source.code, source.name
FROM tenants t
CROSS JOIN (VALUES
    ('asset', 'cash', '1000', 'Cash'),
    ('asset', 'accounts_receivable', '1100', 'Accounts receivable'),
    ('asset', 'inventory', '1200', 'Inventory'),
    ('tax', 'output_tax', '2100', 'Output tax payable'),
    ('revenue', 'sales_revenue', '4000', 'Sales revenue'),
    ('expense', 'cogs', '5000', 'Cost of goods sold')
) AS source(category_code, system_key, code, name)
ON CONFLICT (tenant_id, system_key) DO NOTHING;

INSERT INTO voucher_categories (tenant_id, code, name)
SELECT t.id, source.code, source.name
FROM tenants t
CROSS JOIN (VALUES
    ('receipt', 'Receipt'),
    ('payment', 'Payment'),
    ('journal', 'Journal')
) AS source(code, name)
ON CONFLICT (tenant_id, code) DO NOTHING;

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'finance_account_categories', 'finance_accounts', 'gl_journals',
        'gl_lines', 'party_ledger_entries', 'party_ledger_balances',
        'voucher_categories', 'voucher_entries'
    ] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', table_name);
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', table_name || '_tenant_scope', table_name);
        EXECUTE format(
            'CREATE POLICY %I ON %I USING (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid OR current_setting(''app.authenticating'', true) = ''true'') WITH CHECK (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid OR current_setting(''app.authenticating'', true) = ''true'')',
            table_name || '_tenant_scope', table_name
        );
    END LOOP;
END $$;

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'gl_journals', 'gl_lines', 'party_ledger_entries',
        'party_ledger_balances', 'voucher_entries'
    ] LOOP
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', table_name || '_branch_scope', table_name);
        EXECUTE format(
            'CREATE POLICY %I ON %I USING (current_setting(''app.authenticating'', true) = ''true'' OR branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid OR NULLIF(current_setting(''app.allow_tenant_scope'', true), '''') = ''true'') WITH CHECK (current_setting(''app.authenticating'', true) = ''true'' OR branch_id = NULLIF(current_setting(''app.branch_id'', true), '''')::uuid OR NULLIF(current_setting(''app.allow_tenant_scope'', true), '''') = ''true'')',
            table_name || '_branch_scope', table_name
        );
    END LOOP;
END $$;
