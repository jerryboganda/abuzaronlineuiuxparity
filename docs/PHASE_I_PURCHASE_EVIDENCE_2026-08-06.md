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
- Legacy batch-number format and `Ctrl+B` auto-generation; callers must supply
  a batch number. No synthetic batch is generated.
- PO-to-invoice fetch and purchase history helpers; no command reports success
  for these unsupported verbs.
- Supplier schemes beyond the existing deterministic pricing inputs.
- Purchase reports, formats, print preview, exports, GRN/slip UI, and full
  legacy purchase workflow parity.
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
for route kinds outside the supported Phase I set, and unsupported purchase
history, slip, label, and automatic batch verbs are explicitly labelled
“Not implemented”.

Focused evidence:

- `apps/web/tests/purchase-canonical.spec.ts` — 5/5 passed: no fallback or
  false success, canonical receipt payload and revision state, missing identity
  fail-closed, purchase-order neutrality, and return source/batch validation.
- `apps/web/tests/phase-cd.spec.ts` — 9/9 passed with the canonical PO shell
  integration; the Auto Batch Generation check confirms no client batch is
  synthesized and reports “Not implemented”.
- `pnpm --filter @abuzar/web check` — 0 errors and 0 warnings.
- `pnpm --filter @abuzar/web build` — production build passed.
- `pnpm --filter @abuzar/web test -- --workers=1 --retries=1` — exit 0;
  42 tests completed, with one unrelated browser-context startup flake passing
  on retry.
