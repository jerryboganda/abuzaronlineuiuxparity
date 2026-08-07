# Phase K finance vertical-slice evidence — 2026-08-06

## Scope

This is the first posted cash/credit-sale and supplier-purchase finance slice. It is not a claim of
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
  `reports.read` boundary. The party ledger now includes posted source-backed
  historical payment allocations when a canonical party match exists, while
  unresolved legacy identities remain visible in the statement report.
  Posted document responses include a finance summary only when a journal and
  ledger entry actually exist.
- Existing document and compatibility transaction routes were preserved.
  Posted pack/loose/opening purchases and purchase returns now create balanced supplier payable/input-tax projections; voucher tables remain placeholders.
- Canonical posted sales, purchases, sale returns, and purchase returns support
  append-only compensating void journals and reversed party-ledger entries when
  their completed projections exist; dependent posted documents block the
  source void. See `docs/PHASE_T_VOID_REVERSAL_EVIDENCE_2026-08-07.md`.

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

- The imported `VirtualGl` projection is now available through the bounded
  `GL Journal` report slice, but it has not been reconciled against the new
  GL; historical opening balances are intentionally absent. See
  `docs/PHASE_K_HISTORICAL_GL_EVIDENCE_2026-08-07.md`.
- Trial Balance now unions the same imported `historical_gl_entries` with
  posted canonical GL lines and labels unmatched legacy account codes as
  Historical. Opening balances, fiscal-period grouping, and exact legacy
  account semantics remain unverified.
- Accounts Ledger now unions imported `historical_gl_entries` with posted
  canonical journal lines. The cash-only ledger remains canonical-only until
  VirtualGl account mapping is reviewed; historical account naming and opening
  balance semantics remain unverified.
- Legacy `SaleLedger` and `Purledger` have not been migrated or reconciled.
  The guarded `-wave payments` path now retains source-backed supplier
  `PurPayment` and customer receipt/direct-payment rows for statement reads,
  but source counts, invoice allocation, historical balances, and exact legacy
  settlement semantics remain open.
- Tax source tables, legacy tax ordering/rates, tax registers, returns,
  reversals, and credit-limit behavior remain open Phase L/H/K work.
- Voucher posting, cash-book semantics, historical account mapping, and
  sampled-party statement parity remain open.
- Canonical interactive payment-entry/settlement, `SaleReceivableAdj`,
  `PRAllocationDetail`, and `SRAllocationDetail` adjustment semantics remain
  open for exact legacy posting/grouping. `SaleReceivableAdj` is now retained
  separately and projected into statements through the guarded
  `-wave party-adjustments` path; see
  `docs/PHASE_K_PARTY_PAYMENT_EVIDENCE_2026-08-07.md`.
- `SRAllocationHeader/Detail` and `PRAllocationHeader/Detail` are now retained
  as a distinct source-backed return-allocation stream and projected into
  bounded party statements/ledger reads. Aging and canonical balance mutation
  remain unchanged pending reconciliation of legacy posting and duplicate
  semantics.
- COGS currently reflects the Phase J FIFO allocation costs where available;
  weighted-average/legacy valuation and historical stock-cost reconciliation
  remain unresolved.
- Void reversal is verified for canonical projections only. Historical
  `VirtualGl`, `SaleLedger`, `Purledger`, fiscal-period, tax-register, and
  imported-document reversal parity remain unresolved.
