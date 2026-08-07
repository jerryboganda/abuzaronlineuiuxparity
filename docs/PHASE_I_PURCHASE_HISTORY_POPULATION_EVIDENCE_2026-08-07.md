# Phase I purchase history population follow-up — 2026-08-07

## Scope

This is a bounded workflow-parity increment. It does not claim complete
PowerBuilder purchase parity, exact PO conversion rules for every legacy
source, or acceptance of the full rebuild.

## Implemented

- Canonical purchase transaction-history rows now carry the scoped
  `business_documents.id` as `documentId`.
- `GET /v1/documents/{id}` reads one canonical document only inside the
  authenticated tenant and branch, then applies `purchases.read` or
  `sales.read` according to the stored document kind.
- The response includes persisted line identity, supplier/godown/source
  references, batch/expiry, unit cost, discount percentage, and line tax
  snapshots. Legacy line pricing/tax payloads are preferred, with typed tax
  rate columns as a fallback.
- Pack Purchase contextual commands now distinguish browsing from
  `Populate Purchase Invoice`, `Populate Purchase Return Invoice`, and
  `Fetch Purchase Invoice From Other Sources`. Selecting a canonical history
  row hydrates all available lines and source identity into a new draft; it
  does not reuse the selected posted document's id/version for mutation.
- Purchase commands preserve a populated source document reference whenever
  the draft has one, allowing the server-side source validation to remain the
  authority.

## Verification evidence

| Check | Result |
|---|---|
| `go test ./services/api/internal/httpapi -run 'TestCanonicalPurchaseHistoryHydratesDocumentIdentityAndDetail' -count=1` | Passed against local PostgreSQL; history identity, tenant/branch read, line batch/expiry, discount, and GST metadata asserted. |
| `pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings. |
| `pnpm --filter @abuzar/web exec playwright test tests/phase-cd.spec.ts -g "purchase .* population" --workers=1 --reporter=line` | Passed: 2/2; invoice hydration, return source/batch preservation, and hydrated grid fields verified. |
| `docs/openapi.yaml` | Documents the canonical document detail read and line tax summary contract. |

## Remaining boundary

Compatibility history rows that have no canonical document identity remain
summary-only and cannot be hydrated through this endpoint. The exact legacy
source-specific PO fetch, pending-due selection, sale-template conversion,
multi-line report/list semantics, and PowerBuilder dialog/raster behavior
still require captured source evidence and acceptance comparison.
