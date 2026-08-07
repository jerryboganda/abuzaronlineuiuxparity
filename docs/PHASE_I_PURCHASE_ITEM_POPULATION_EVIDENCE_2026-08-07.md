# Phase I purchase item population follow-up - 2026-08-07

## Scope

This is a bounded implementation of the captured purchase-window
Populate Items and Populate From Sale Template commands. It uses the existing
active canonical item/master APIs and does not claim the original PowerBuilder
source-selection dialog or all legacy pending-due/import rules.

## Implemented

- Quick-search or unresolved item-name values in purchase rows are resolved
  through the authenticated active canonical item lookup.
- Exact code, name, legacy ID, and alias matches are accepted; a single
  returned active item is accepted as a unique match.
- Successful resolution writes the canonical UUID, legacy identity, and
  normalized item name into the row and reuses the existing godown batch
  refresh path.
- Multiple rows are processed independently and unresolved searches remain
  visible for correction. A blank row may use the current lookup result when
  one is already available.
- Free-text rows without a canonical identity continue to fail closed during
  purchase save/post validation.
- Populate From Sale Template now loads active tenant-scoped sale-template
  masters, accepts supported `rows`, `lines`, or `items` payloads into a new
  draft, and re-runs canonical item resolution; templates without a supported
  line payload remain unchanged with an explicit operator message.

## Verification evidence

| Check | Result |
|---|---|
| `cmd /c pnpm --filter @abuzar/web check` | Passed: 0 errors and 0 warnings. |
| `cmd /c pnpm --filter @abuzar/web exec playwright test tests/purchase-canonical.spec.ts -g "Populate Items resolves purchase" --workers=1 --reporter=line` | Not completed: the single browser command produced no test output within the quick-check window and was stopped; no browser pass is claimed. The template-picker regression was added but not run. |
| `git diff --check` | Passed; only existing LF/CRLF normalization warnings were emitted. |

## Remaining boundary

The exact PowerBuilder Populate Items source dataset, selection dialog,
multi-source/template/pending-due rules, calculated price/tax/discount
side-effects, print behavior, and legacy-vs-new raster/operator acceptance
remain open. Database-backed lookup was not replayed against the canonical
tenant in this run because `DATABASE_URL` was unset.
