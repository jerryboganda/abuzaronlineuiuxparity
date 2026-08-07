# Phase F Item Un-Posted Transaction Report Evidence — 2026-08-07

Status: implementation and focused automated checks green; overall legacy replacement acceptance remains escalated.

## Captured source contract

The captured Item Form menu defines `File > Show Un-Posted Transaction Report`
(`Ctrl+F1`). The command is intended to show transaction rows that have not
been posted for the selected item.

## Implemented slice

- The Item Form menu marks the command implemented and maps it to the
  tenant-scoped `master.read` permission.
- `GET /v1/master/item/{id}/unposted-transactions` reads canonical
  `business_documents` and `business_document_lines` for the authenticated
  tenant and branch, matching the selected Item UUID, `status = 'draft'`, and
  `deleted_at IS NULL`.
- The response preserves document kind/number/date, line identity, item
  legacy identity, quantity, unit price, and line total. It is bounded to 200
  rows and reports truncation explicitly.
- The Item Form opens a legacy-styled report dialog with an empty state,
  loading/error states, row table, and bounded-result notice. The command does
  not claim to reconstruct every historical SQL Server temporary/buffer table.
- Migration `043_item_unposted_transaction_index.sql`, contracts, OpenAPI,
  focused API tests, and browser coverage document the slice.

## Focused evidence

- `go test ./internal/httpapi -run 'Test(ItemUnpostedTransactionsQueryIsScopedAndBounded|CanonicalMasterRoutesRemainAuthenticated)$' -count=1` — passed.
- `go vet ./internal/httpapi` — passed.
- `cmd /c pnpm check` from `apps/web` — `svelte-check found 0 errors and 0 warnings`.
- `cmd /c pnpm exec playwright test tests/phase-f.spec.ts -g "unposted transaction report" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 1/1.
- `cmd /c pnpm exec playwright test tests/phase-cd.spec.ts -g "item master|captured" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 2/2.

## Remaining acceptance boundary

The migration was not applied to a live target in this short pass. Historical
SQL Server buffer/temporary transaction extraction, every legacy transaction
family and field, exact PowerBuilder report geometry/focus behavior, and
full-volume performance remain unverified. This is a locally verified
canonical draft-line workflow, not full Item Form or overall parity acceptance.
