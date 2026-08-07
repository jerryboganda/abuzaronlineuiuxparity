# Phase Q Deleted Sale Items Log evidence — 2026-08-07

## Status

The captured `Item Reports > Deleted Sale Items Log` leaf now has a
source-backed target, a guarded historical loader, and a tenant/branch-scoped
report projection. This closes the compatibility-only implementation gap at
the code and contract level. Overall legacy acceptance remains open.

## Source and target

The reviewed SQL Server catalog exposes `dbo.DeletedSaleItem` with the captured
item/godown codes, pack units, quantity, bonus quantity, sale price, discount,
flat discount, unit sales tax, GST percentage, deletion date, machine, user,
and sale-invoice fields. Migration `030_historical_deleted_sale_items.sql`
retains those typed values, the source row identity, and the complete source
payload under tenant and branch scope. The target has an immutable conflict key
of `(tenant_id, branch_id, legacy_id)`, optional item resolution, indexes for
date/invoice and item history, and restrictive row-level security policies.

## Loader

`migration/cmd/bulk-historical` accepts `-wave deleted-sale-items` and includes
it in `-wave all`. The SQL Server query is read-only and deterministic. It
fails closed for missing required source columns or identity/date values,
stages batches through PostgreSQL `COPY`, resolves an item when available, and
keeps the raw source payload for reconciliation. A canonical run still
requires `-allow-canonical`, `-tenant-id`, and `-branch-id`:

```powershell
go run ./migration/cmd/bulk-historical `
  -source $source -target $target -allow-canonical `
  -tenant-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01 `
  -branch-id 6f25fd3e-5f66-4b4e-a31d-254c9e6b0a02 `
  -wave deleted-sale-items
```

The loader was not run against the canonical source in this coding slice, so
source-row counts, target counts, and reconciliation totals remain open.

## Report contract

The API and Svelte report registry expose six bounded fields: Sale Invoice,
Deleted At, Machine / User, Item / Godown, Qty + Bonus, and Sale Price. Reads
are tenant/branch/date/text scoped and paginated over
`historical_deleted_sale_items`; the source-backed note explicitly retains the
captured discount/tax fields even though they are not yet visible in this
bounded contract.

## Focused evidence

- `report_q_test.go` covers registry status, source-backed projection notes,
  scope predicates, payload filtering, and pagination.
- `historical_test.go` covers the migration shape, RLS, source-row retention,
  master-item foreign key, and absence of destructive statements.
- `phase-q.spec.ts` covers the title, projection note, representative row, and
  visible labels for the Svelte leaf.
- The short Go, loader compile, Svelte check, and whitespace checks are recorded
  in `docs/ACCEPTANCE_EVIDENCE_2026-08-07.md` after execution.

## Remaining acceptance evidence

Exact PowerBuilder DataWindow columns, deletion-order semantics, calculated
discount/tax presentation, filters, print layout, golden output replay,
canonical source/target count reconciliation, and live database-backed route
results remain unverified. These are acceptance boundaries, not claims of
failure in the source-backed implementation.
