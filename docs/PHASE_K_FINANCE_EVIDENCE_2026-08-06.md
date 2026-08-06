# Phase K finance vertical-slice evidence — 2026-08-06

## Scope

This is the first posted cash/credit-sale finance slice. It is not a claim of
historical financial parity or completion of the Phase K plan.

## Implemented

- `013_finance_ledgers.sql` adds tenant-scoped account categories and charts,
  immutable source-linked GL journals/lines, customer/cash party-ledger
  entries and balances, and voucher-category/entry placeholders.
- New tenants receive explicit safe posting categories and six system accounts
  (cash, receivable, inventory, output tax, sales revenue, and COGS). No
  opening or legacy balance rows are created.
- Composite tenant/branch keys, forced RLS, source document/event uniqueness,
  and deferred database balance checks reject unbalanced journals.
- Posted cash and credit sales now post atomically after FIFO stock allocation:
  cash or receivable debit, sales-revenue credit, supplied pricing tax credit,
  and allocated-cost COGS/inventory lines. Customer sales also update the
  running customer balance.
- Drafts do not post finance. Existing journal/party projections are
  idempotent and missing account configuration fails the enclosing transaction.
- Guarded `GET /v1/finance/accounts`, `/v1/finance/journals`, and
  `/v1/finance/ledger?partyId=...` reads use the existing authenticated
  `reports.read` boundary. Posted document responses include a finance
  summary only when a journal and ledger entry actually exist.
- Existing document and compatibility transaction routes were preserved.
  Purchases/payables have no posting implementation; voucher tables are
  explicit placeholders only.

## Verification

- `go test ./services/api/internal/httpapi -run
  TestFinanceSalePostingIntegration -count=1` passed against the disposable
  PostgreSQL cluster. The test drives `documentCommand` itself and verifies
  the real HTTP command path returns a balanced finance summary, replays the
  same idempotency key without another journal, leaves drafts finance-free,
  and rolls back document/stock changes when COGS configuration is inactive.
- Focused API unit/contract and PostgreSQL integration coverage exercises cash
  versus credit debit/credit semantics, tax, balanced totals, customer
  balance, allocated stock-cost COGS, replay idempotency, draft no-op, tenant
  isolation, and rollback when a required account is inactive.
- `go test ./services/api/... ./services/edge/... ./migration/...` passed.
- Ordered migrations, including `013_finance_ledgers.sql`, applied twice with
  exit code 0; both passes reported 14 migrations applied.

## Unresolved historical and parity gaps

- Legacy `VirtualGl` has not been migrated or reconciled against the new GL;
  historical opening balances are intentionally absent.
- Legacy `SaleLedger` and `Purledger` have not been migrated or reconciled.
  Supplier/payables posting is not implemented.
- Tax source tables, legacy tax ordering/rates, tax registers, returns,
  reversals, and credit-limit behavior remain open Phase L/H/K work.
- Voucher posting, cash-book semantics, historical account mapping, and
  sampled-party statement parity remain open.
- COGS currently reflects the Phase J FIFO allocation costs where available;
  weighted-average/legacy valuation and historical stock-cost reconciliation
  remain unresolved.
