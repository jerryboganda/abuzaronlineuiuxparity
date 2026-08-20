# Phase F Item Price Policy Evidence — 2026-08-07

Status: implementation and focused automated checks green; overall legacy replacement acceptance remains escalated.

## Captured source contract

The captured SQL Server catalog defines `dbo.PricePolicy` with
`PricePolicyCode` (`int`), `Name`, and the owning item `ICode` (`int`). Its
`dbo.PricePolicyDetail` rows retain `PricePolicyCode`, `QtyLimit` (`int`),
`Price` (`numeric(19,4)` source precision), `ExpiryDate`, `ItemFlatDisc`, and
`DiscPerc`. The captured Item Form command is `File > Set Item Price Policy`
(`Ctrl+F11`).

## Implemented slice

- `GET` and `PUT /v1/master/item/{id}/price-policy` require `master.read` and
  `master.write` respectively and bind a selected policy header to the
  authenticated tenant and item `ICode`.
- The API reads and atomically replaces the existing canonical
  `price_policy_tiers` rows while preserving existing row IDs/source mapping
  where supplied. It bounds the tier collection to 100 rows, retains exact
  decimal text through four places, validates ISO dates, and rejects duplicate
  quantity/expiry pairs or cross-policy row IDs.
- The Item Form menu marks the captured command implemented and opens a
  legacy-styled policy/tier editor. The editor exposes quantity limit, price,
  expiry date, flat discount, and discount percent without applying unapproved
  customer/group enforcement rules.
- Contracts, OpenAPI, focused browser coverage, and the acceptance record now
  describe the slice.

## Focused evidence

- `go test ./internal/httpapi -run 'Test(NormalizeItemPricePolicyTiersPreservesSourceDecimalsAndRejectsAmbiguity|NormalizeItemModelCodesPreservesMembershipAndBoundsSourceType|CanonicalMasterRoutesRemainAuthenticated)$' -count=1` — passed.
- `go vet ./internal/httpapi` — passed.
- `cmd /c pnpm check` from `apps/web` — `svelte-check found 0 errors and 0 warnings`.
- `cmd /c pnpm exec playwright test tests/phase-f.spec.ts -g "notes command|ItemImage rows|alternate-alias|association command|author command|model command|price-policy command" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 7/7.
- `cmd /c pnpm exec playwright test tests/phase-cd.spec.ts -g "item master|captured" --project=chromium --retries=0 --timeout=15000 --reporter=line` — passed 2/2.

## Remaining acceptance boundary

The source-backed import/reconciliation artifacts exist, but this short slice
did not rerun live SQL Server extraction or apply a new migration. Exact
PowerBuilder policy-picker geometry, keyboard/focus behavior, expiry-date
semantics, PriceType/Module mapping, customer/group assignment/enforcement,
complete promotion behavior, and the 50-invoice golden replay remain open.
This is a locally verified command slice, not exact pricing acceptance or
overall legacy replacement acceptance.
