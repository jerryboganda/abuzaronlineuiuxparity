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

The current migration tree subsequently added `027_historical_item_history_adjustments.sql`
and `028_business_document_void_reversals.sql`. Both are included in the
current ordered replay and have been validated in the latest disposable
replay; the historical import and reconciliation status above remains
unchanged.

Final checks: ungranted locks `0`, invalid indexes `0`, orphan document lines
`0`, orphan stock-batch item references `0`, documents `331,928`, lines
`895,015`, and open exceptions `320,865`. The exact run record is
`migration/ORDERED_MIGRATION_REPLAY_2026-08-07.json`.

## Exception analysis and bulk stock/finance promotion — 2026-08-07

Before this category pass, the sandbox had `open=320,865` exceptions:

- stale party-ledger payload failures: `320,465`;
- stale/superseded return/header bookkeeping: resolved during target matching;
- non-positive purchase/order rows: `398`;
- no VirtualGl exception records yet.

After target matching, bulk promotion, and reviewed quarantine:

| Status | Count |
|---|---:|
| resolved | 500,976 |
| ignored/quarantined | 404 |
| open | 0 |

The 404 ignored records are explicit decisions: 398 zero-total or
non-positive-quantity rows, 2 residual line-count gap summaries, 1 VirtualGl
duplicate-identity summary, and 1 stock-batch identity discrepancy. They are
not treated as successful canonical rows.

Bulk promotion results:

- `StockReport`: `3,215,967` source rows → `3,215,967`
  `historical_stock_snapshots`.
- `VirtualGl`: `1,021,852` source rows, `1,021,801` distinct reviewed
  identities → `1,021,801` historical GL rows; 51 duplicate source rows were
  quarantined.
- Stock ledger: `781,203` rows (`158,107` inbound and `623,096` outbound).
  Source line metric is `781,243`; 40 residual line gaps are quarantined.
- Party ledger: `329,115` eligible source documents → `329,115` entries.
- Stock batches: `10,507` canonical physical identities; the seven-row
  difference from the reviewed source union count is quarantined rather than
  silently deduplicated.

The final finance/stock metric artifact is
`parity/catalog/phase-e-finance-stock-reconciliation-final.json`.
It reports matched StockReport snapshots, distinct VirtualGl identities,
return ledgers, party ledgers, and zero open exceptions. Sale/purchase stock
line counts remain mismatched by the quarantined 40 rows, so Phase E is not
green or complete.

During the large set-based ledger promotion, a concurrent background migration
runner caused one additional deadlock at `2026-08-07 02:35:16.399 PKT`
(`stock_ledger` transaction versus an `010_master_normalized.sql` lock on
`sync_events`). The identified stale migration backends were terminated by
PID, their transaction was rolled back, and the ledger was resumed in
5,000-invoice ranges. Final checks show no ungranted locks, no invalid
indexes, and no orphan document-line or stock-batch references.

## Tax-rule ambiguity bookkeeping clarification — 2026-08-07

The bulk promotion's `open=0` statement refers to `migration_exceptions` only.
A fresh local PostgreSQL probe found no open rows in that table, but found 16
open rows in the separate `migration_ambiguous_records` table: four each for
`AdditionalTaxRule`, `ExtraTaxRule`, `IncomeTaxRule`, and `UnitSalesTaxRules`.
All use `tax_rule_has_no_numeric_rate`; their retained labels are “TAX ON
ACTUAL AND BONUS QTY”, “TAX ON ACTUAL QTY ONLY”, “TAX ON BONUS QTY ONLY”, and
“NO TAX”. They remain explicitly unpromoted because the captured source rows
do not contain a numeric rate, and the generated finance/stock artifact does
not query this ambiguity table. This is an open source-semantics acceptance
boundary, not a silently resolved migration exception.

## Current bookkeeping recheck — 2026-08-07

A fresh local PostgreSQL probe after the historical wave reported 501,024
resolved, 404 ignored, and 32 open `migration_exceptions` rows in aggregate.
All 32 are `dbo.Purdetail` rows with `non_positive_quantity` in the isolated
canonical tenant; they remain quarantined because
`business_document_lines.quantity` is required to be positive. The sandbox
tenant has no open `migration_exceptions`. Its separate
`migration_ambiguous_records` table has 16 open tax-rule rows, while the
canonical tenant has none. The reconciliation command now emits both counts
and returns `bookkeeping.status=review_required` whenever either table is open.
