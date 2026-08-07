# Phase I - purchase credit terms evidence

Date: 2026-08-07

This is a bounded purchase-workflow improvement for supplier credit terms. It
does not claim complete purchase/payment replay or exact aging report parity.

## Implemented boundary

- Canonical purchase drafts and document responses now carry bounded whole-day
  `creditDays` terms.
- Pack, loose, and opening purchase forms expose Credit Days, restore it from
  canonical history, include it in the command signature, and send it through
  the canonical save/post lifecycle.
- The server validates terms to -36500 through 36500 days and stores them in
  the existing pricing result snapshot without adding a migration.
- Imported purchase documents retain their existing `Purledger.CreditDays`
  payload fallback, while payable aging now prefers that source value and falls
  back to the canonical snapshot for newly entered purchases. Purchase returns
  remain explicitly unaged unless a later source contract proves otherwise.
- OpenAPI now documents the purchase term and cash payment contract together
  with the canonical document schemas.

## Focused evidence

- `gofmt -w services/api/internal/httpapi/documents.go services/api/internal/httpapi/documents_test.go services/api/internal/httpapi/report_q_test.go`
- `go test ./services/api/internal/httpapi -run 'Test(NormalizeDocumentCreditDays|WithCreditDaysPricingSnapshot|PhaseQQueriesArePostedAndScopeBound)' -count=1`
- `cmd /c pnpm --filter @abuzar/web check`
- `cmd /c pnpm --filter @abuzar/web exec playwright test tests/purchase-canonical.spec.ts --list`
- `python -c "import yaml; yaml.safe_load(open('docs/openapi.yaml', encoding='utf-8')); print('openapi yaml parse: ok')"`
- `git diff --check`

Results: all listed checks passed. Svelte diagnostics reported 0 errors and 0
warnings; purchase contract discovery reported 9 tests. The focused receipt
runtime test also passed with one worker and no retries after constraining the
quick-search input width so its Lookup button remains hit-testable inside the
legacy grid cell. The first runtime attempt exposed that UI hitbox defect before
the credit-term assertion; no credit-term failure remained after the fix.

No long build, full browser suite, CI flow, SQL Server import, or live database
reconciliation was run for this slice.

## Remaining acceptance evidence

- Reconcile Purledger terms, paid/balance, returns, and due-date counts against
  the reviewed source database, including negative/blank terms and date
  boundaries.
- Confirm the source-backed supplier payment allocation wave against the
  reviewed source, purchase-return treatment, statement grouping, exact
  PowerBuilder aging buckets, and print/PDF/workbook output. The bounded
  statement projection is implemented in
  `docs/PHASE_K_PARTY_PAYMENT_EVIDENCE_2026-08-07.md`; source reconciliation
  and canonical invoice allocation remain open.
- Complete the wider purchase gates: PO/GRN semantics, exact batch generation,
  tax/discount replay, labels/slips, stock/GL/ledger reconciliation, and UAT.
