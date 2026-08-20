# Phase G pricing workflow evidence — 2026-08-07

## Scope

This bounded follow-up closes two executable pricing-workflow gaps against the
captured sale and purchase forms. It does not claim full PowerBuilder pricing
parity, customer/group `GroupAllowedPrice` semantics, or a 50-invoice golden
replay.

## Implemented

- The sales `SalePrice:#` selector now exposes all ten captured price levels.
- Item lookup preserves the captured `SalePrice1` through `SalePrice10` values
  (with the existing legacy payload fallbacks) instead of collapsing them to
  the currently displayed value.
- Changing the selector reprices already-selected grid rows and recalculates
  their line totals. Manual edits update the selected tier before the next
  preview/post request.
- The authenticated pricing-preview request now sends the actual captured
  tier array through the selected level, so the deterministic pricing engine
  resolves the same tier the grid displays.
- Canonical purchase documents now resolve a tenant-scoped ItemSuppliers
  scheme for the selected canonical supplier when the command does not supply
  an explicit scheme. Discount percentage, qualifying quantity, and bonus
  quantity are persisted in the pricing snapshot and participate in the same
  exact-decimal calculation. A source row with a bonus but no qualifying
  quantity is retained but not promoted without an approved legacy rule.

## Evidence

- `pnpm --filter @abuzar/web check` — passed with 0 errors and 0 warnings.
- `pnpm --filter @abuzar/web test -- --workers=1 --retries=0 --reporter=line apps/web/tests/sales-canonical.spec.ts` — passed 6/6, including the new selector/tier-array regression.
- `pnpm --filter @abuzar/web test -- --workers=1 --retries=0 --reporter=line --grep "purchase" apps/web/tests/phase-cd.spec.ts` — passed 5/5 purchase browser workflows.
- `$env:DATABASE_URL=postgres://.../abuzar_next go test ./services/api/internal/httpapi -run 'TestCanonicalPurchaseLoadsItemSupplierScheme|TestCanonicalPurchaseHistoryHydratesDocumentIdentityAndDetail|TestPricingPreviewRequestMapsTiersDiscountsAndTaxes' -count=1` — passed.
- `$env:DATABASE_URL=postgres://.../abuzar_next go test ./services/api/internal/httpapi -run 'TestPurchaseVerticalSliceIntegration|TestCanonicalPurchaseLoadsItemSupplierScheme' -count=1` — passed.

## Remaining acceptance boundary

The migrated `price_policy_tiers` rows are reconciled as source data, but
customer/group price assignment, `PricePolicyDetail` date semantics, full
price-policy promotion, ItemSuppliers day semantics, and replay against
approved historical `Saledetail` golden totals remain open. Those rules must
be observed or approved from the read-only PowerBuilder/source boundary before
they are applied automatically.

