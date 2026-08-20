# Phase E bookkeeping and purchase-order wave evidence — 2026-08-07

## Scope

This evidence covers the migration bookkeeping guard, the reviewed
`PurOrderHeader`/`PurOrderDetail` map and metrics, and the canonical
`Purdetail` exception audit path. It does not claim a fresh SQL Server import.

## Verified target state

The local PostgreSQL probe was tenant-scoped and read-only:

| Scope | `migration_exceptions` open/resolved/ignored | `migration_ambiguous_records` open |
|---|---:|---:|
| Canonical `6f25fd3e-5f66-4b4e-a31d-254c9e6b0a01` | 32 / 48 / 0 | 0 |
| Sandbox `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee` | 0 / 500,976 / 404 | 16 |
| Aggregate | 32 / 501,024 / 404 | 16 |

The 32 canonical exceptions are `dbo.Purdetail` rows with
`non_positive_quantity`. The 16 sandbox ambiguities are four each from
`AdditionalTaxRule`, `ExtraTaxRule`, `IncomeTaxRule`, and `UnitSalesTaxRules`,
all with `tax_rule_has_no_numeric_rate`. The reconciler now reports both
classes as `bookkeeping.status=review_required` and can fail closed with
`-fail-on-open-bookkeeping` after writing its report.

## Purchase-order wave readiness

`migration/maps/phase-e-historical-orders.json` is the reviewed two-table map.
`migration/maps/phase-e-historical-order-reconciliation-metrics.json` adds
header count, line count, header total, and line quantity checks. The runbook
requires the header range before the detail range, explicit canonical tenant,
branch, and counter scopes, and a reconciliation run that fails on open
bookkeeping. It remains an unexecuted acceptance wave until a reviewed
read-only SQL Server connection is restored.

## Exception auditability

`migration/cmd/bulkpurchaselines` now stores the retained purchase ID, row ID,
item ID, quantity calculation inputs (`PackQty`, `LooseQty`, `PackUnits`), and
reviewed price/tax fields in `migration_exceptions.details` for invalid source
rows. This preserves the evidence needed to decide whether a non-positive
legacy row represents a valid historical adjustment, a zero-value artifact, or
an unsupported row; it does not coerce the row into the positive canonical line
contract.

## Verification

- `go test ./migration/... -count=1` passed.
- `go vet ./migration/...` passed.
- Both reviewed JSON maps parse successfully.
- `git diff --check` reported no whitespace errors; only existing LF/CRLF
  normalization warnings.
- The source probe was not completed: SQL Server Integrated Authentication
  returned the untrusted-domain login boundary. No credentials were read or
  copied into evidence.
