# Phase I purchase vertical slice evidence — 2026-08-06

## Scope

This phase implements a bounded purchase slice; it is not a claim of full
legacy purchase parity.

- `014_purchase_documents.sql` extends the existing document and command-kind
  checks without changing sale rows, adds supplier/source references and
  batch/tax metadata, and configures explicit payable/input-tax accounts.
- Pack, loose, and opening purchase drafts can be saved and posted only with
  an active supplier, godown, batch, expiry (when supplied), and explicit
  non-negative unit cost. Posting creates or updates a non-expired batch and
  stock-in ledger row atomically.
- Purchase orders may be saved, posted, and voided without stock or GL
  projection. They remain neutral until a separate receipt is posted.
- Purchase returns require a posted purchase source, matching supplier, source
  item, explicit source batch allocation, available stock, and an unreturned
  source quantity. They create stock-out and reversal payable/input-tax
  projections only after those checks pass.
- Supplier payable and input-tax GL accounts are required and active; missing
  configuration rejects the transaction and rolls back the document, event,
  revision, stock, and finance work.
- Existing command receipts, immutable sync events, revisions, tenant/branch
  scope, and sale posting paths remain in use.
- `015_sync_event_final_payload.sql` records the canonical pending-to-final
  transition and rejects later business-document event mutation. Older
  state-less event payloads remain readable without being rewritten.

## Observed verification

- `go test ./services/api/internal/httpapi -run 'Test(PurchaseCommandValidationRequiresReceiptMetadataButKeepsPONeutral|PurchaseMigrationExtendsKindsAndPreservesSaleCompatibility|PurchaseVerticalSliceIntegration)$' -count=1`
  — passed against local PostgreSQL.
- The purchase integration test covered PO draft/post neutrality, receipt
  batch creation and stock-in, supplier payable balance, replay idempotency,
  invalid return rollback, valid source/batch return, missing payable-account
  rollback, and tenant isolation.
- Ordered PostgreSQL migrations, including `014_purchase_documents.sql`,
  were applied twice — both passes exited successfully.
- Existing document, stock, and sale-finance integration slices were rerun
  after the change — passed.
- `FinanceSalePostingIntegration` verified the final payload contains the
  stock/finance document snapshot, records `finalized_at`, and rejects a
  second payload update.
- `python -c "import yaml; yaml.safe_load(open('docs/openapi.yaml', encoding='utf-8'))"`
  — passed. OpenAPI now documents all Phase I purchase kinds, supplier/source
  references, batch/expiry/unit-cost/tax fields, and lifecycle response
  semantics. Shared contracts now expose the supplier IDs, purchase batch
  allocation wire name, and finance summary as well. No web UI files were
  changed.

## Remaining Phase I gaps

The following remain intentionally unimplemented or unreconciled:

- Legacy purchase label/slip byte formats, thermal printing, and `Ctrl+M` /
  `Alt+F8` hardware behavior.
- Full GST/PCT/advance-income-tax source tables, legacy tax ordering,
  per-line tax allocation, tax registers, and exact historical GST parity.
- Legacy batch-number format remains a source-reconciliation item. The client
  convenience action currently generates deterministic `AUTO-YYYYMMDD-NNN`
  identifiers for populated rows through `Ctrl+B`; this is not evidence of
  legacy-format parity, and explicit operator-entered batches remain unchanged.
- Purchase history/list restore is live through the canonical transaction
  history endpoint. PO-to-invoice fetch still requires source-specific mapping
  and remains a follow-on reconciliation item.
- Supplier schemes beyond the existing deterministic pricing inputs.
- Full purchase report projections and legacy byte-identical slip/GRN output
  remain open; the shared report surface now supplies format selection, print
  preview, CSV, browser PDF, and Excel-compatible workbook export.
- Historical `Purdetail`, `PurOrderDetail`, `Purledger`, `VirtualGl`, batch,
  tax, label, and stock reconciliation/import remain open.
- Weighted-average/legacy valuation, multi-line source-line disambiguation,
  transfers, adjustments, and posted-invoice reversal workflows remain open.

## Frontend canonical purchase follow-on — 2026-08-06

The purchase route now fail-closes all five Phase I document families:
pack/loose/opening receipts, purchase returns, and purchase orders. Active
canonical item lookup results, suppliers, and godowns are required; receipt
lines require explicit batch, expiry, and unit cost; return lines require a
source document UUID plus an explicit source batch UUID/allocation. Supported
families call only the typed `/v1/documents/{kind}` lifecycle, preserve stable
command/idempotency state and expected versions, and reject non-accepted or
wrong-status responses without rendering success. Purchase orders report their
stock/GL-neutral response. Legacy compatibility events remain available only
for route kinds outside the supported Phase I set. Purchase history/list restore
and the deterministic client-convenience `AUTO-YYYYMMDD-NNN` batch helper are
live; slip/label hardware output, source-specific PO fetch, and legacy
batch-format reconciliation remain follow-on work.

Focused evidence:

- `apps/web/tests/purchase-canonical.spec.ts` — 6/6 passed: no fallback or
  false success, canonical receipt payload and revision state, missing identity
  fail-closed, purchase-order neutrality, and return source/batch validation.
- `apps/web/tests/phase-cd.spec.ts` — contextual purchase tests passed with the
  canonical PO shell integration; the Auto Batch Generation check only asserts
  the documented client-convenience identifier, not legacy-format parity.
- Focused browser run: `pack purchase Ctrl+B generates a deterministic batch
  identifier` passed (1/1). No sandbox evidence for the legacy batch format is
  claimed by this test.
- `pnpm --filter @abuzar/web check` — 0 errors and 0 warnings.

## Canonical history population follow-up — 2026-08-07

Purchase history now carries canonical document identity and the new
`GET /v1/documents/{id}` read path hydrates persisted lines, supplier/godown,
batch/expiry, discount, and tax metadata. The contextual `Populate Purchase
Invoice` and `Populate Purchase Return Invoice` commands now select the
appropriate canonical history source and populate a new draft without
reusing the selected posted document identity. Full evidence is recorded in
[`PHASE_I_PURCHASE_HISTORY_POPULATION_EVIDENCE_2026-08-07.md`](PHASE_I_PURCHASE_HISTORY_POPULATION_EVIDENCE_2026-08-07.md).
- `pnpm --filter @abuzar/web build` — production build passed.
- `pnpm --filter @abuzar/web test -- --workers=1 --reporter=line` — the current
  69-test suite completed successfully: 69 passed serially.

## Purchase return source allocation follow-up — 2026-08-07

Purchase returns now retain and edit all source-batch allocations from a
hydrated canonical document instead of collapsing them to the first batch.
The return screen provides a legacy-compatible source batch ID field plus an
explicit multi-batch editor backed by active godown availability; the command
serializes every selected allocation and rejects missing IDs, duplicate batch
IDs, invalid quantities, and allocation totals that do not equal the line
quantity. The new focused Playwright contract is present but was not executed
in this short verification slice. The web type check passed with no errors or
warnings, Playwright discovery listed 9 purchase contracts, and the focused
Go purchase tests passed. Exact source-document allocation reconciliation and
approved PowerBuilder raster/UAT evidence remain open.
