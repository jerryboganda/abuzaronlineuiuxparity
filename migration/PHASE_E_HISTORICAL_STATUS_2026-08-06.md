# Phase E historical waves — sandbox evidence

Date: 2026-08-06. Source: `AbuzarLegacyReference` only. Target: the isolated
`Legacy Reference Sandbox` tenant. No canonical SQL Server database was opened
by the import commands.

## Implemented

- Added reviewed document/line maps for sales, purchases, sale returns,
  purchase returns, and purchase orders.
- Added reviewed sync-event, stock-batch/ledger, stock-snapshot, party-ledger,
  historical GL-entry, tax/rate, price-tier, and explicit ambiguity maps.
- Added deterministic UUID generation, reviewed source predicates/expressions,
  composite lookups, text lookups, resumable mapping ranges, released failed-row
  savepoints, small import batches, and no-op conflict handling to the importer.
- Added migration `020_historical_migration_wave.sql`. It preserves source
  identities, branch scope, legacy payloads, document status/date, and explicit
  unsupported-field artifacts without fabricating operators or batch identities.

## Observed source and target counts

| Wave | Source rows | Target rows | Result |
|---|---:|---:|---|
| `SaleLedger` | 291,361 | 291,361 | matched |
| `Saledetail` | 620,619 | 527,350 | partial; restartable |
| `Purledger` | 6,419 | 6,396 | category/status sub-wave remains |
| `Purdetail` | 113,564 | 15,600 | partial; restartable |
| `SRLedger` | 30,704 | 0 | pending |
| `SRdetail` | 44,579 | 0 | pending |
| `PRLedger` | 634 | 0 | pending |
| `PRdetail` | 2,481 | 0 | pending |
| `PurOrderHeader` | 2,810 | 0 | pending |
| `PurOrderDetail` | 108,423 | 0 | pending |
| `StockReport` | 3,215,967 | 0 | pending |
| `VirtualGl` | 1,021,852 | 0 | pending |
| `SalesTaxSchedule` | 7 | 7 | matched |
| `TaxCategory` | 3 | 3 | matched |
| `PricePolicyDetail` | 30,052 | 30,052 | matched |
| item tax assignments | 60,104 | 60,104 | matched |
| explicit tax-rule ambiguities | 16 | 16 | recorded |

The source counts above are the live sandbox counts emitted by the reconciler;
they are not substituted from the canonical-database baseline.

## Reconciliation artifacts

- `parity/catalog/phase-e-historical-documents-reconciliation.json`
- `parity/catalog/phase-e-credit-sale-retry.json`
- `parity/catalog/phase-e-sync-events-import.json`
- `parity/catalog/phase-e-tax-rates-import.json`
- `parity/catalog/phase-e-tax-rates-reconciliation.json`
- `parity/catalog/phase-e-stock-finance-reconciliation.json`
- `migration/maps/phase-e-historical-reconciliation-metrics.json`

The document reconciliation ran 17 read-only business metrics. Matched
examples include the 291,361 sale-header count, sale total
`234003081.00`, 291,361 sale events, seven GST schedules, and 30,052 price
tiers. Partial waves are reported as mismatched rather than being presented as
complete.

## Exceptions and remaining inventory

The first partial attempt produced 4 open sale-header exceptions caused by
credit-sale customer enforcement. The source customer is present, the
credit-header retry is recorded, and the final bookkeeping check has 0 open
exceptions. Resolved historical bookkeeping exceptions remain auditable in
`migration_exceptions`; no source values or credentials are copied into the
reports.

The reviewed maps cover 49 unique source tables. Against the 763-table
inspector manifest, 714 tables remain unmapped. In particular, the remaining
work includes full return/order line promotion, the 3.2M-row stock snapshot
and stock-ledger completion, the 1M-row historical GL promotion, supplier
party-ledger completion, and the remaining report/configuration families.

## Continuation evidence — 2026-08-06 21:00+

The resumed dependency waves completed the return headers/lines and purchase
order headers. Current sandbox counts are:

| Target projection | Rows |
|---|---:|
| `business_documents` | 331,928 |
| `Saledetail` lines | 620,615 |
| `Purdetail` lines | 113,528 |
| `SRdetail` lines | 44,579 |
| `PRdetail` lines | 2,481 |
| `PurOrderDetail` lines | 113,812 |
| `stock_batches` | 7,595 |
| `party_ledger_entries` | 8,650 |
| `stock_ledger` | 0 |
| `historical_gl_entries` | 0 |

The live sandbox source was re-counted during this continuation as
`Saledetail=620,619`, `Purdetail=113,564`, and
`PurOrderDetail=113,995`. The four missing sale lines, 36 purchase lines, and
183 order lines remain visible as reconciliation mismatches. Remaining
purchase-detail rows with zero quantity and order rows with zero quantity are
retained as row exceptions rather than coerced into canonical positive
quantities.

New artifacts:

- `parity/catalog/phase-e-sr-headers-import.json`
- `parity/catalog/phase-e-sr-lines-import.json`
- `parity/catalog/phase-e-pr-headers-import.json`
- `parity/catalog/phase-e-pr-lines-import.json`
- `parity/catalog/phase-e-order-headers-import.json`
- `parity/catalog/phase-e-order-lines-resume-1961.json`
- `parity/catalog/phase-e-historical-documents-reconciliation-continued.json`
- `parity/catalog/phase-e-historical-orders-reconciliation.json`
- `parity/catalog/phase-e-party-ledger-import.json`
- `parity/catalog/phase-e-party-ledger-reconciliation.json`
- `parity/catalog/phase-e-stock-ledger-reconciliation.json`
- `parity/catalog/phase-e-historical-gl-reconciliation.json`

The continuation is **not complete**. The final bookkeeping query showed
`resolved=180,511` and `open=320,865` exception records, including
superseded retry records and genuine unsupported/zero-quantity rows. Stock
ledger, StockReport snapshot, and VirtualGl promotion still require completion
with a larger safe execution window.

## Deadlock recovery and migration replay — 2026-08-07

At `2026-08-06 23:30:35.950 PKT`, PostgreSQL reported a deadlock between the
`010_master_normalized.sql` `master_items` backfill and a concurrent historical
`business_document_lines` insert. PostgreSQL aborted the migration transaction;
the importer was then stopped and verified idle before replay.

Post-recovery checks found:

- no active importer transaction or ungranted database locks;
- zero invalid indexes;
- zero orphan business-document lines;
- zero orphan stock-batch item references;
- `master_items=30,052` and `master_records(kind=item)=30,052`;
- no canonical database connection.

The ordered migration runner was replayed through migrations `001`–`022`
successfully after the importer was idle. Migrations `014` and `022` now use
`NOT VALID` compatibility checks because existing application rows contain
return/lifecycle values introduced by later waves; new writes remain checked.
The recovery artifact is
`migration/PHASE_E_DEADLOCK_RECOVERY_2026-08-07.json`.

Final read-only reconciliation artifacts:

- `parity/catalog/phase-e-historical-documents-reconciliation-final.json`
- `parity/catalog/phase-e-historical-orders-reconciliation-final.json`
- `parity/catalog/phase-e-party-ledger-reconciliation-final.json`
- `parity/catalog/phase-e-stock-ledger-reconciliation-final.json`

The historical import remains incomplete and is not marked complete.

## Ordered migration replay through current set — 2026-08-07

The ordered runner processed all 27 current SQL files (`001` through `026`,
including `023`, `024`, and `025`). Run 1 succeeded. Run 2 exposed an
idempotency defect in `024_preferences_branch_scope.sql` because its branch
foreign key was created without an existence guard. That migration was
hardened with a catalog-backed `DO` guard, and run 3 succeeded.

The compatibility tradeoff is explicit: the party-ledger checks from `014`
remain `NOT VALID`, so existing historical party-ledger rows are not
retroactively rejected; new writes remain checked. The document-kind checks
introduced by `014`/`022` were replaced by the later `023`/`025` checks and
are currently validated. The `023` and `025` checks validated successfully
on the existing data, and the 025 return triggers enforce source/line/reversal
rules. The 024 foreign key was not relaxed.

Final checks: ungranted locks `0`, invalid indexes `0`, orphan document lines
`0`, orphan stock-batch item references `0`, documents `331,928`, lines
`895,015`, and open exceptions `320,865`. The exact run record is
`migration/ORDERED_MIGRATION_REPLAY_2026-08-07.json`.
