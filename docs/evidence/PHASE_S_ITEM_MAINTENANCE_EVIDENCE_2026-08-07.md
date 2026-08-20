# Canonical item-maintenance evidence — 2026-08-07

## Scope

This slice promotes four captured maintenance leaves from preference-shaped
acknowledgement to tenant-scoped PostgreSQL item-master mutations:

- `change-items-price`
- `change-item-discount`
- `update-item-basic-data`
- `change-item-reorder-qty`

Each request locks the matching `master_items` row by canonical code or
`legacy_id`, validates the captured field contract, updates the retained
legacy-key payload (and the normalized `name` column for a Name change), and
records the operation/audit payload in the same transaction. The API accepts
both decimal strings and JSON numbers because Svelte `type=number` bindings
serialize browser-entered values as JSON numbers.

## Verified behavior

The focused Go tests passed from the repository root:

```text
go test ./services/api/internal/httpapi -run 'TestCanonicalItemMaintenanceValidation|TestMaintenanceManageOperationsIntegration' -count=1
ok   github.com/abuzar/abuzar-next/services/api/internal/httpapi  1.958s
```

The integration subtest used browser-shaped numeric JSON for price, discount,
and reorder quantities. It verified sale-price aliases, purchase-price
preservation, discount aliases, normalized item name, reorder/minimum values,
operation completion, and previous/new audit values.

The focused browser checks also passed 2/2:

```text
pnpm --filter @abuzar/web test -- --workers=1 --retries=0 --grep "route-specific maintenance fields|Change Items Price submits"
2 passed
```

The browser regression waits for the route's initial state read before filling
the form, submits `Item Code=ITEM-1`, `Price Type=Sale Price`, and `New
Price=12.75`, then proves a POST and the completed response message. Numeric
maintenance inputs now explicitly use `step="any"`; this preserves decimal
values such as `12.75` instead of allowing the browser's default integer step
to block form submission.

## Boundaries still open

- `effectiveDate` is validated and retained in the operation payload; this
  slice does not claim scheduled or historical price-version selection.
- The remaining captured maintenance leaves still require their own reviewed
  business rules and source-backed integration where they are not covered by
  a canonical endpoint. Generic compatibility workbenches are not promoted by
  this evidence.
- Exact PowerBuilder validation messages, keyboard/focus behavior, print
  output, migrated-item sampling, and operator UAT remain separate acceptance
  gates.
