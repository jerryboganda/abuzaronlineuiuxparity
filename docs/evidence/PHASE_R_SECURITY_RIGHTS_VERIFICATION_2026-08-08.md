# Phase R Security Rights Verification — 2026-08-08

Scope: `services/api/internal/httpapi/access.go`, `access_integration_test.go`,
`apps/web/tests/phase-r.spec.ts`.

Claim under test: Go middleware enforcement AND web-app menu-gating match the
legacy rights matrix (4 groups: ADMINISTRATOR, REMOTE, SALES OFFICER, SHIFT
INCHARGE; 726 `GroupRights` rows) migrated into `group_rights` /
`legacy_groups` by `db/migrations/009_legacy_security_rights.sql` and
`019_security_data_import_adaptation.sql`, exactly, per group, per right-code.

## Verdict

**Not confirmed as "exactly, per group, per right-code."** The enforcement
mechanism genuinely *is* data-driven — `loadOperatorAccess` (auth.go) reads
`group_rights` row by row via SQL, not a hardcoded Go table — but two findings
mean the migrated 726-row matrix currently does not drive real per-right-code
enforcement outcomes the way the claim implies:

1. **ADMINISTRATOR bypasses the table entirely.** `hasTenantAdminRole` treats
   any role whose code case-insensitively matches `owner`, `tenant_admin`,
   `admin`, or `administrator` as a full permission/scope bypass — and
   `ADMINISTRATOR` is exactly one of the four legacy group codes. Its 486
   `group_rights` rows are read into `operator.LegacyRights` for
   display/audit but never consulted by `hasPermission`/`scopeAllowed`
   because the bypass short-circuits first. Enforcement for this group is
   role-name-driven, not table-driven.
2. **The other three groups' right codes don't resolve to any modern
   permission.** Every one of the 726 migrated rows (both tenants that have
   imported data) has `permission IS NULL` and `allowed = true`; `right_code`
   is a raw legacy numeric identifier (e.g. `"1"`, `"5256"`). The effective-
   permission SQL falls back to `lower(right_code)` when `permission` is
   unset, so REMOTE/SALES OFFICER/SHIFT INCHARGE operators' effective
   `Permissions` are numeric strings that never equal any of the ~20
   permission names `requirePermission()` actually gates on (`sales.read`,
   `manage.groups`, `master.write`, …). Net effect: for this tenant's real
   data, none of the 726 rows currently unlock a single `requirePermission`-
   gated Go endpoint or a single `requiredPermission`-gated web menu item for
   the three non-admin groups.

Both layers (Go `requirePermission`/`scopeAllowed` and the web app's
`applyMenuAccess`) read from the *same* derived `operator.Permissions` /
`access.permissions`, so this gap is consistent — and confirmed — across both
enforcement points.

## 1–2. access.go / access_integration_test.go: mechanism and existing coverage

`access.go` itself has no rights logic — it's request/response plumbing
(`access`, `roles`, `roleRights`, `updateRoleRights`, operator CRUD) around
data assembled in `auth.go`:

- `loadOperatorAccess` (auth.go:329) is the single source of truth. It runs
  three queries scoped by `user_memberships ⋈ group_rights` /
  `group_allowed_scopes`, all tenant/user parameterized (no hardcoded role
  logic): effective `Permissions` (a `UNION` of allowed `role_permissions`
  and allowed `group_rights`, each with a `NOT EXISTS` anti-join suppressing
  entries that have a matching explicit `group_rights` deny — deny wins),
  `Scopes`, and raw `LegacyRights`.
- `hasPermission` (auth.go:304) / `requirePermission` (auth.go:320): tenant-
  admin bypass first, else membership in `operator.Permissions`.
- `scopeAllowed` / `canonicalGodownScopeAllowed` (auth.go:478,500): same
  admin bypass, else `operator.Scopes[kind][key]`.

Existing tests (`access_integration_test.go`, before this change) covered
scope-update tenant isolation and a canonical-godown-scope UUID case — both
against synthetic seeded data, not the real migrated tenant. `server_test.go`
had unit tests (`TestLegacyGroupEquivalentRolePaths`,
`TestRevokedLegacyRightFailsClosed`) exercising `hasPermission` against
hand-built `sessionContext.Permissions` slices — i.e. they assumed the
`operator.Permissions` array is correct rather than deriving it from
`group_rights` through `loadOperatorAccess`. **No existing test queried the
real migrated `group_rights` table for tenant
`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee` or asserted `loadOperatorAccess`'s
output against it.**

## 3. Migrated data shape (read against DATABASE_URL)

```
tenant eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee, group_rights:
  ADMINISTRATOR   486 rows, all allowed=true
  REMOTE            6 rows, all allowed=true
  SALES OFFICER   111 rows, all allowed=true
  SHIFT INCHARGE  123 rows, all allowed=true
  total: 726  (matches the claimed count)

Across BOTH tenants with imported data (1,452 group_rights rows total):
  allowed=false rows:                0
  permission column populated:       0
  right_code format:                 100% numeric (legacy RightCode ids)

role_permissions for this tenant:    0 rows
group_allowed_scopes for this tenant: ADMINISTRATOR 70, REMOTE 60,
                                       SALES OFFICER 22, SHIFT INCHARGE 21
```

Sample row (`right_code=1`, four legacy `GroupCode`s folded into distinct
target roles):
```
right_code | permission | allowed | legacy_status | legacy_group_code | legacy_payload
1          |            | t       | 1              | 12                | {"Status":1,"GroupCode":12,"RightCode":1}
```

## Code-path trace (≥5 right codes, ≥2 groups)

Traced via the new Go integration tests (below), which exercise the exact
`loadOperatorAccess` → `hasPermission`/`requirePermission` path against real
`user_memberships ⋈ group_rights` joins:

| Group | Right code (source: real migrated sample unless noted) | `allowed` in table | Effective permission derived | `operator.Permissions` contains it? | `requirePermission`/menu outcome |
|---|---|---|---|---|---|
| SALES OFFICER | `1` (real) | true | `1` | yes | never matches a gated permission name — no endpoint unlocked |
| SALES OFFICER | `10` (real) | true | `10` | yes | same |
| SHIFT INCHARGE | `1` (real) | true | `1` | yes | same |
| SHIFT INCHARGE | `5256` (real, shared with REMOTE) | true | `5256` | yes | same |
| REMOTE | `5286` (real, all 6 of REMOTE's real rows traced) | true | `5286` | yes | same |
| SALES OFFICER | `http-trace-1` → `permission=reports.read` (synthetic, mapping populated) | true | `reports.read` | yes | `requirePermission(..., "reports.read")` → **granted** — proves the mechanism is genuinely table-driven once `permission` is populated |
| SALES OFFICER | `http-trace-2` / `http-trace-2-deny` → both `permission=master.write` (synthetic pair) | true / **false** | `master.write` | **no** (deny wins) | `requirePermission(..., "master.write")` → **denied**, confirming deny-precedence |
| ADMINISTRATOR | `http-trace-3` → `permission=manage.groups`, allowed=**false** (synthetic, no competing allow) | false | `manage.groups` | table says no | `requirePermission(..., "manage.groups")` → **granted anyway** — the `hasTenantAdminRole("ADMINISTRATOR")` bypass overrides the table's explicit deny |

This is 7+ right codes across all 4 groups (ADMINISTRATOR, REMOTE, SALES
OFFICER, SHIFT INCHARGE), demonstrating: (a) real rows are read row-by-row
and land in `operator.Permissions`/`LegacyRights` unmodified; (b) deny-wins
precedence works when a right maps to a real permission; (c) ADMINISTRATOR's
enforcement is role-name-hardcoded, not table-driven.

## 4. phase-r.spec.ts — prior UI coverage

All 6 pre-existing tests drove `page.route('**/v1/access', ...)` and
`**/v1/roles/*/rights` with **hand-typed fixture JSON**
(e.g. `legacyRights: [{ rightCode: 'LEGACY-SALE', permission: 'sales.read', allowed: true, mapping: 'explicit' }]`,
`scopes: { report: { 'sale-detail': true, 'sale-summary': false } }`). None
of them read the database. This is legitimate for exercising the
editor/menu **components** in isolation, but it means the "does the imported
right actually gate the menu" question was answered against invented data,
not the real 726-row import — exactly the drift risk the task asked about.
Menu-gating itself (`apps/web/src/lib/legacy-menu.ts`, `applyMenuAccess`) is
driven by a **static** `requirementFor()` map from legacy menu path → modern
permission string (e.g. `Sales → sales.read`, `Manage > Groups →
manage.groups`), checked against `access.permissions` — the same array
`loadOperatorAccess` builds server-side, so the Go-side finding above
(numeric right codes never match a modern permission name) applies equally
to web menu-gating.

## 5. Test coverage added

**Go** (`services/api/internal/httpapi/access_integration_test.go`), two new
integration tests, plus a bugfix (see below):

- `TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups` — for
  all 4 groups, samples real rows from the migrated table for tenant
  `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee` (19 rows each for ADMINISTRATOR/
  SALES OFFICER/SHIFT INCHARGE — capped only by the `LIMIT`; REMOTE covers
  **all 6** of its real rows, since that is REMOTE's entire real matrix and
  padding to 20 would mean fabricating rows that don't exist in the source),
  copies them verbatim into an isolated disposable tenant, binds a real
  operator via `user_memberships`, runs the real `loadOperatorAccess`, and
  asserts every sampled row surfaces unchanged in `operator.Permissions` and
  `operator.LegacyRights`. It also adds one synthetic allow/deny pair per
  group (the real table has zero denies to sample) to prove deny-wins, and
  asserts none of the real sampled right codes collide with any of the ~20
  modern permission names — pinning today's "unmapped" reality so a future
  change is forced to touch this test consciously.
  **Coverage note:** REMOTE's group only has 6 right codes total across all
  migrated data (not 726/4 ≈ 20 as might be assumed) — "at least 20 per
  group" is only achievable for ADMINISTRATOR/SALES OFFICER/SHIFT INCHARGE
  from real data; REMOTE gets 100% real coverage (6/6) instead.
- `TestGroupRightsHTTPEnforcementReflectsTableAndAdministratorBypassesIt` —
  closes the loop to actual HTTP-level `requirePermission` outcomes for
  SALES OFFICER (grant via table, deny-wins via table) and demonstrates the
  ADMINISTRATOR bypass overriding an explicit table deny.

**Bugfix in the same file:** all four tests (the two new ones and the two
pre-existing ones) seed a disposable tenant via `seedDocumentTenant` and
previously cleaned up with a bare
`defer database.ExecContext(ctx, "DELETE FROM tenants WHERE id = $1::uuid", tenantID)`.
This silently failed — `branches`, `counters`, `users`, `roles`, and
`audit_events` reference `tenants` with `NO ACTION` (non-cascading) foreign
keys, and the deferred call's error was discarded — leaving orphaned
`document-test-*` tenants behind on every run. **389 such orphaned tenants
had accumulated in the shared dev database since 2026-08-06** (this predates
this session; not all of them are from this file). Added
`cleanupIsolatedLegacyTenant` (deletes `audit_events` → `users` → `roles` →
`counters` → `branches` → `tenants`, in FK-safe order) and switched all four
tests to use it; verified repeated runs now leave zero debris. The two
tenants this session created before the fix, plus the shared target tenant's
own data, were confirmed untouched/cleaned up (`group_rights` for
`eeeeeeee-…` still has exactly 726 rows). The broader fix (the same bug
exists in other `*_test.go` files that call `seedDocumentTenant` — out of
this task's file scope) was flagged as a separate background task rather
than fixed here.

**Web** (`apps/web/tests/phase-r.spec.ts`), one new test: *"REMOTE group
menu-gating reflects the live migrated group_rights matrix, not a hardcoded
fixture."* It shells out to `psql` (via Node's built-in `child_process`, no
new dependency) to read REMOTE's real effective permissions from
`group_rights` for the source tenant, feeds that real array into the mocked
`/v1/access` response, and asserts `File > Save And Post` under the
cash-sale context (`requiredPermission: 'sales.write'`) stays disabled —
because the real data never grants `sales.write`. Skips gracefully
(`test.skip`) if `DATABASE_URL`/`psql` aren't reachable, so it won't break
the full suite running elsewhere without DB access.

## 6. Verification runs

```
$ go vet ./internal/httpapi/...
(clean, no output)

$ DATABASE_URL=postgres://postgres@127.0.0.1:5432/abuzar_next?sslmode=disable \
  go test ./internal/httpapi/... -run \
  'TestImportedGroupScopeUpdateIsTenantScopedAndAudited$|TestCanonicalGodownLookupHonorsExplicitUUIDScopes$|TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups$|TestGroupRightsHTTPEnforcementReflectsTableAndAdministratorBypassesIt$|TestPermissionCheckAllowsAdminAndAssignedPermissionOnly$|TestLegacyGroupEquivalentRolePaths$|TestRevokedLegacyRightFailsClosed$|TestImportedRightResolutionPreservesExplicitDenyAndAmbiguity$|TestImportedScopeResolutionPreservesExplicitDeny$|TestDeniedPermissionReturnsAuditableProblemResponse$|TestNormalizeRolePermissions$|TestLegacyAllowedScopesFailClosedWhenAnAllowListExists$|TestCompositeImportedGodownScopesDeferUntilCanonicalMappingExists$' \
  -count=1 -v

=== RUN   TestImportedGroupScopeUpdateIsTenantScopedAndAudited
--- PASS: TestImportedGroupScopeUpdateIsTenantScopedAndAudited (1.26s)
=== RUN   TestCanonicalGodownLookupHonorsExplicitUUIDScopes
--- PASS: TestCanonicalGodownLookupHonorsExplicitUUIDScopes (1.41s)
=== RUN   TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups
=== RUN   TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups/ADMINISTRATOR
=== RUN   TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups/REMOTE
=== RUN   TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups/SALES_OFFICER
=== RUN   TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups/SHIFT_INCHARGE
--- PASS: TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups (1.87s)
    --- PASS: .../ADMINISTRATOR (0.02s)
    --- PASS: .../REMOTE (0.01s)
    --- PASS: .../SALES_OFFICER (0.01s)
    --- PASS: .../SHIFT_INCHARGE (0.01s)
=== RUN   TestGroupRightsHTTPEnforcementReflectsTableAndAdministratorBypassesIt
--- PASS: TestGroupRightsHTTPEnforcementReflectsTableAndAdministratorBypassesIt (1.53s)
=== RUN   TestNormalizeRolePermissions
--- PASS: TestNormalizeRolePermissions (0.00s)
=== RUN   TestPermissionCheckAllowsAdminAndAssignedPermissionOnly
--- PASS: TestPermissionCheckAllowsAdminAndAssignedPermissionOnly (0.00s)
=== RUN   TestLegacyGroupEquivalentRolePaths
--- PASS: TestLegacyGroupEquivalentRolePaths (0.00s)  (4 subtests pass)
=== RUN   TestRevokedLegacyRightFailsClosed
--- PASS: TestRevokedLegacyRightFailsClosed (0.00s)
=== RUN   TestLegacyAllowedScopesFailClosedWhenAnAllowListExists
--- PASS: TestLegacyAllowedScopesFailClosedWhenAnAllowListExists (0.00s)
=== RUN   TestCompositeImportedGodownScopesDeferUntilCanonicalMappingExists
--- PASS: TestCompositeImportedGodownScopesDeferUntilCanonicalMappingExists (0.00s)
=== RUN   TestImportedRightResolutionPreservesExplicitDenyAndAmbiguity
--- PASS: TestImportedRightResolutionPreservesExplicitDenyAndAmbiguity (0.00s)
=== RUN   TestImportedScopeResolutionPreservesExplicitDeny
--- PASS: TestImportedScopeResolutionPreservesExplicitDeny (0.00s)
=== RUN   TestDeniedPermissionReturnsAuditableProblemResponse
--- PASS: TestDeniedPermissionReturnsAuditableProblemResponse (0.00s)
PASS
ok  	github.com/abuzar/abuzar-next/services/api/internal/httpapi	6.176s
```

```
$ cd apps/web && DATABASE_URL=postgres://postgres@127.0.0.1:5432/abuzar_next?sslmode=disable \
  pnpm exec playwright test tests/phase-r.spec.ts --workers=1 --retries=0

Running 7 tests using 1 worker
  ok 1 Groups captured File commands drive the canonical role editor (1.5s)
  ok 2 Groups rights matrix edits imported denies without losing normalized permissions (1.5s)
  ok 3 Group Allowed Price Setting edits retained composite GroupAllowedPrice scopes (1.3s)
  ok 4 imported group scope setting leaves isolate their approved scope kinds (2.3s)
  ok 5 revoked contextual command is disabled while tenant admin retains it (2.7s)
  ok 6 report menu applies the imported report scope filter (1.5s)
  ok 7 REMOTE group menu-gating reflects the live migrated group_rights matrix, not a hardcoded fixture (1.7s)
  7 passed (14.0s)
```

Also confirmed: without `DATABASE_URL` set, the new Playwright test
`test.skip`s cleanly (verified separately) rather than failing the suite.

`business_document_lines` and `stock_ledger` were not touched by any change
or query in this task. The shared Go API server and Postgres instance were
not restarted. `git commit` was not run.

## Files changed

- `D:\ABUZAR\AbuzarNext\services\api\internal\httpapi\access_integration_test.go`
  — added `cleanupIsolatedLegacyTenant` (FK-safe teardown, replacing a
  silently-failing one-liner in all 4 tests in the file),
  `TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups`,
  `TestGroupRightsHTTPEnforcementReflectsTableAndAdministratorBypassesIt`.
- `D:\ABUZAR\AbuzarNext\apps\web\tests\phase-r.spec.ts` — added
  `fetchRealEffectivePermissions` helper and the REMOTE live-data
  menu-gating test.
- `services/api/internal/httpapi/access.go` — read only, unchanged.
- `services/api/internal/httpapi/auth.go` — read only (out of stated file
  scope but necessary to trace enforcement; unchanged).
- `apps/web/src/lib/legacy-menu.ts`, `LegacyMenuBar.svelte`,
  `LegacyWorkflowSurface.svelte` — read only, unchanged.

## Flagged out-of-scope issue

Filed as a background task rather than fixed here (touches
`documents_integration_test.go` and other `*_test.go` files outside this
task's file scope): the `seedDocumentTenant` cleanup pattern used across many
integration tests has the same silently-failing-DELETE bug fixed locally in
`access_integration_test.go` above, and has left 389 orphaned
`document-test-*` tenants in the shared dev database since 2026-08-06.

## 2026-08-08 permission backfill (finding #2 fixed)

This section fixes finding #2 above ("the other three groups' right codes
don't resolve to any modern permission") for the already-migrated tenant.
Finding #1 (ADMINISTRATOR bypasses the table via `hasTenantAdminRole`) is
untouched and remains intentional, per the task that produced this fix.

### Modern permission strings (found via grep, full set)

Grepped every `requirePermission(r, w, operator, "...")` call site plus the
indirect ones that pass a variable (`documents.go` `writePermission`/
`permission`, `history.go` `readPermission`, `maintenance.go`
`maintenancePermission(kind)`, `business.go`/`canonical.go` local
`permission` variables) across `auth.go`, `access.go`, `business.go`,
`canonical.go`, `documents.go`, `finance.go`, `history.go`, `inventory.go`,
`item_unposted.go`, `maintenance.go`, `preferences.go`, `pricing.go`,
`reports.go`, `stock.go`, `tax.go`. 19 distinct strings are actually gated on
(a 20th, `tax.override`, exists in `access.go`'s `supportedRolePermissions`
allow-list for role editing but is not consulted by any `requirePermission`
call site today):

`branch.read`, `manage.groups`, `manage.users`, `master.read`,
`master.write`, `sales.read`, `sales.write`, `tenant.read`,
`purchases.read`, `purchases.write`, `sync.push`, `sync.pull`,
`sync.review`, `reports.read`, `preferences.read`, `preferences.write`,
`tax.read`, `tax.write`, `maintenance.write`.

### Mapping methodology

Queried `dbo.Rights` (SQL Server, read-only) for all 486 rows
(`RightCode, RightName, MenuName, IndicesString, RightCatCode`) and
cross-referenced against the 726 migrated `group_rights` rows for the four
groups. The 486 distinct `right_code` values actually used across those 726
rows are **exactly** the 486 rows in `dbo.Rights` (verified by diff) — every
legacy right that exists is referenced by ADMINISTRATOR's full matrix, so no
`dbo.Rights` row needed to be excluded as "never used."

Each row's top-level breadcrumb segment of `RightName` (e.g. `Sales`,
`Purchase`, `Reports`, `Basic Data`, `Maintenance`, `Manage`, `Transactions`,
`E-Prescription`) drove category-level rules, refined per-row where the name
was ambiguous or where a direct code correspondence to a Go document-kind
constant existed (e.g. `businessDocumentKinds`/`isPurchaseDocumentKind` in
`documents.go`, `maintenancePermission()` in `maintenance.go`,
`historyAggregates` in `history.go`). Concretely:

- **Reports** (any `RightName` starting with `Reports`, including
  CRS/Patient/Student/Employee/Service sub-report categories that have no
  modern module of their own) → `reports.read`. The modern app has exactly
  one coarse "can view reports" gate, so every legacy report-view right maps
  to it — this is a many-to-one collapse, not an over-grant, because it
  mirrors the modern app's own granularity.
- **Sales** / **Purchase** — top-level document-type rights matched directly
  to `businessDocumentKinds`/`historyAggregates` entries (`cash-sale`,
  `credit-sale`, `cash-return`, `credit-return`, `open-cash-return`,
  `open-credit-return`, `quotation`, `refused-sale`, `pack-purchase`,
  `loose-purchase`, `opening-purchase`, `purchase-return`, `purchase-order`)
  → `sales.write`/`purchases.write` (matching `documentCommand`'s
  `writePermission` logic). Fine-grained `"... , Rights , ..."` sub-rights
  within Sales/Purchase were split by a `Show`/`View`/`Display`/`Preview`
  keyword in the name → `*.read`; everything else (Modify/Save/Post/
  Fiscalize/Attach/Override/Allow/...) → `*.write`.
- **Manage > Groups** (RightCode 101) → `manage.groups`; **Manage > Users**
  (102) → `manage.users` — exact 1:1, and confirmed by querying
  `group_rights` that only ADMINISTRATOR's real rows contain codes 101/102.
- **Manage > Cashier Management** cluster (1344, 1345, 1346, and the
  `Cashier Activity Window , Rights , ...` sub-codes 5027/5064–5068/5250) →
  `sales.read`, because `maintenancePermission("manage-cashier-job")` in
  `maintenance.go` literally returns `"sales.read"`.
- **Basic Data** — browse/view screens for a master-record entity or lookup
  table → `master.read`; rights with an explicit `Modify`/`Add New`/`Assign`
  verb in the name → `master.write`. `Basic Data > Patient` / `> Student`
  and `Basic Data > Sale Template` were left unmapped (no modern module).
- **Maintenance > Adjustment** (increase/decrease/stock-adjustment/
  inter-godown-transfer) → `maintenance.write`, matching the `"inventory"`
  transaction aggregate and `stock.go`'s `maintenance.write` gate. Database
  utilities / backup / integrity-check / historical-data import / inplace-
  initialization rights → `maintenance.write`, matching
  `maintenancePermission()`'s default branch and `maintenanceExternalOutcome`'s
  kind matching. Item-price/discount-category/basic-data/suppliers/
  reorder-qty/batch-lock rights → `master.write` (they mutate item master
  records, not maintenance-workflow state).
- **Everything else left unmapped**: `Transactions , Accounting Vouchers , ...`
  and `Transactions , Payroll Activities , ...` (no modern
  account/voucher/payroll permission exists at all), `E-Prescription`,
  `Maintenance , Preferences` (a single legacy right that both views and
  edits settings — cannot be safely resolved to only `preferences.read` or
  only `preferences.write` without either under- or over-granting),
  `Maintenance , Change Password` (self-service, not gated), and a handful of
  single-toggle rights with no confident read/write or module
  correspondence (`Item Priority Setting`, `Open (ctrl+G)`,
  `Print Patient Labels`, `Modify Last Transaction Date`,
  `Discount Policy Based Profit Margin`, `Maintenance , Receipt , Receipt`).
  `Manage` sub-features backed by `group_allowed_scopes` (Group Wise Header/
  Price/Cash-Account Setting, Group Wise Supplier Category) were left
  unmapped because they are enforced through the scope table, not a
  `requirePermission` permission string. The legacy parent/child
  server-sync rights (`Control Panel , ... , Import Now/Sync Now/Transmit
  Now/Translate Now/...`) were deliberately **not** mapped to
  `sync.push`/`sync.pull`/`sync.review` despite the tempting name overlap —
  those modern permissions gate a different (business-document event)
  sync architecture, and the correspondence would be speculative.

### Coverage

Of the 486 distinct legacy `RightCode`s in `dbo.Rights` (== the 486 distinct
codes actually used across the 726 migrated rows):

- **433 mapped** (89%): `reports.read` 240, `sales.write` 48, `master.read`
  37, `sales.read` 37, `purchases.write` 29, `master.write` 18,
  `maintenance.write` 14, `purchases.read` 8, `manage.groups` 1,
  `manage.users` 1.
- **53 left unmapped** — see the "everything else" bullet above for the
  categories and reasons; the full per-code list with its specific reason is
  in the comments of `legacy_rights_mapping.go` (every mapped row also
  carries its own reason as a trailing comment).

Applied to the 726 rows for the four migrated groups of tenant
`eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`:

| Group | Total rows | Now mapped |
|---|---|---|
| ADMINISTRATOR | 486 | 433 |
| REMOTE | 6 | 6 |
| SALES OFFICER | 111 | 108 |
| SHIFT INCHARGE | 123 | 120 |

REMOTE — the smallest, most tractable group — is now **100% mapped**: every
one of its 6 real rights resolves to a modern permission
(`reports.read`, `master.read` ×3, `sales.read`, `purchases.write`).

### Files changed

- `services/api/internal/httpapi/legacy_rights_mapping.go:30` — new file, the
  explicit, human-reviewable `legacyRightPermission map[string]string`
  (right_code → permission, each entry commented with the legacy
  `RightName` and the reason it was mapped that way). This is the single
  source of truth; the SQL backfill and this doc's counts are derived from
  it, not maintained independently.
- `db/migrations/044_legacy_rights_permission_backfill.sql:41` — new
  migration, a single tenant-agnostic, idempotent
  `UPDATE group_rights ... FROM (VALUES ...) WHERE gr.right_code = v.right_code
  AND gr.permission IS NULL` built from the same 433-row table. Applied
  directly (`psql ... -f db/migrations/044_legacy_rights_permission_backfill.sql`)
  against `DATABASE_URL` per this task's constraints (not via a shared
  migration-runner restart). Result: `UPDATE 1334` (across both tenants that
  have imported security data, 1452 total `group_rights` rows /
  1334 now-mapped — matches 433+6+108+120 for the 4 groups of the primary
  tenant plus the second tenant's own migrated rows).
- `services/api/internal/httpapi/access_integration_test.go:570` — new test
  `TestLegacyRightPermissionBackfillUnlocksRealNonAdminPermissions` (see
  below).
- This file — this section.

### Runtime enforcement: no code change needed

`loadOperatorAccess` (auth.go) already reads
`lower(COALESCE(g.permission, g.right_code))` — it was already preferring
`permission` over `right_code` whenever `permission` is set; it only ever
fell back to the raw numeric `right_code` because `permission` was NULL for
every row. Backfilling `permission` is therefore sufficient on its own:
`hasPermission`/`requirePermission`/`loadOperatorAccess` needed no changes.
Confirmed directly: `SELECT DISTINCT lower(COALESCE(gr.permission,
gr.right_code)) FROM group_rights gr JOIN legacy_groups lg ... WHERE
legacy_group_name = 'SALES OFFICER'` now returns `maintenance.write,
master.read, master.write, purchases.read, purchases.write, reports.read,
sales.read, sales.write` plus 3 residual raw numeric codes (`675`, `90`,
`91` — all deliberately unmapped, see above) instead of 8 raw numeric codes.

### Test coverage added

`TestLegacyRightPermissionBackfillUnlocksRealNonAdminPermissions`
(`access_integration_test.go`) builds on
`TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups` /
`TestGroupRightsHTTPEnforcementReflectsTableAndAdministratorBypassesIt`
(which only copied `right_code`/`allowed`, never `permission`, into their
isolated tenants — so they remain valid as a pin of the pre-backfill
`right_code`-only behavior and needed no changes). The new test:

1. Samples REMOTE's entire real 6-row matrix and 25 of SALES OFFICER's real
   mapped rows from the live migrated table (source tenant
   `eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee`), including the now-populated
   `permission` column.
2. Cross-checks every sampled row's database `permission` value against
   `legacyRightPermission[right_code]` in Go, failing loudly if the SQL
   backfill and the committed Go table ever drift apart.
3. Copies the sampled rows (right_code + permission + allowed, verbatim)
   into an isolated disposable tenant, binds one real operator per group via
   `user_memberships`, and runs the real `loadOperatorAccess`.
4. **Direction 1 (grant):** for every sampled allowed row, asserts the
   mapped permission appears in `operator.Permissions` AND that
   `requirePermission` at the HTTP layer grants it — for both REMOTE and
   SALES OFFICER, non-admin groups that never touch the `hasTenantAdminRole`
   bypass.
5. **Direction 2 (deny):** asserts both groups are denied `manage.groups`
   and `manage.users` — permissions only right codes 101/102 grant, which
   only ADMINISTRATOR's real rows contain — proving the backfill grants
   exactly what the table says and nothing more.

Uses the existing `cleanupIsolatedLegacyTenant` FK-safe teardown from the
prior verification pass; confirmed no orphaned `rights-backfill-*` tenant
rows remain after the run (`SELECT count(*) FROM tenants WHERE code LIKE
'rights-backfill-%'` → 0).

### Verification run

```
$ cd services/api && go vet ./internal/httpapi/...
(clean, no output)

$ DATABASE_URL=postgres://postgres@127.0.0.1:5432/abuzar_next?sslmode=disable \
  go test ./internal/httpapi/... -run \
  'TestLegacyRightPermissionBackfillUnlocksRealNonAdminPermissions$|TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups$|TestGroupRightsHTTPEnforcementReflectsTableAndAdministratorBypassesIt$|TestImportedGroupScopeUpdateIsTenantScopedAndAudited$|TestCanonicalGodownLookupHonorsExplicitUUIDScopes$|TestNormalizeRolePermissions$|TestPermissionCheckAllowsAdminAndAssignedPermissionOnly$|TestLegacyGroupEquivalentRolePaths$|TestRevokedLegacyRightFailsClosed$' \
  -count=1 -v

=== RUN   TestImportedGroupScopeUpdateIsTenantScopedAndAudited
--- PASS: TestImportedGroupScopeUpdateIsTenantScopedAndAudited (1.50s)
=== RUN   TestCanonicalGodownLookupHonorsExplicitUUIDScopes
--- PASS: TestCanonicalGodownLookupHonorsExplicitUUIDScopes (1.30s)
=== RUN   TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups
=== RUN   TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups/ADMINISTRATOR
=== RUN   TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups/REMOTE
=== RUN   TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups/SALES_OFFICER
=== RUN   TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups/SHIFT_INCHARGE
--- PASS: TestMigratedGroupRightsMatrixIsConsultedRowByRowAcrossFourGroups (2.04s)
    --- PASS: .../ADMINISTRATOR (0.03s)
    --- PASS: .../REMOTE (0.00s)
    --- PASS: .../SALES_OFFICER (0.01s)
    --- PASS: .../SHIFT_INCHARGE (0.01s)
=== RUN   TestGroupRightsHTTPEnforcementReflectsTableAndAdministratorBypassesIt
--- PASS: TestGroupRightsHTTPEnforcementReflectsTableAndAdministratorBypassesIt (1.75s)
=== RUN   TestLegacyRightPermissionBackfillUnlocksRealNonAdminPermissions
--- PASS: TestLegacyRightPermissionBackfillUnlocksRealNonAdminPermissions (1.75s)
=== RUN   TestNormalizeRolePermissions
--- PASS: TestNormalizeRolePermissions (0.00s)
=== RUN   TestPermissionCheckAllowsAdminAndAssignedPermissionOnly
--- PASS: TestPermissionCheckAllowsAdminAndAssignedPermissionOnly (0.00s)
=== RUN   TestLegacyGroupEquivalentRolePaths
--- PASS: TestLegacyGroupEquivalentRolePaths (0.00s)  (4 subtests pass)
=== RUN   TestRevokedLegacyRightFailsClosed
--- PASS: TestRevokedLegacyRightFailsClosed (0.00s)
PASS
ok  	github.com/abuzar/abuzar-next/services/api/internal/httpapi	8.449s
```

`business_document_lines` and `stock_ledger` were not touched by any query
or change in this task. `git commit` was not run. The shared API server was
not restarted; the migration was applied directly with `psql` against
`DATABASE_URL`, matching how `ops/postgres/apply-migrations.sh` applies
every other file in `db/migrations/` (plain autocommit `psql --file`, no
wrapping transaction) — the migration was written as a single UPDATE
statement (not a session temp table) specifically because that runner does
not wrap files in a transaction.

### Remaining gap (unchanged from the original finding)

ADMINISTRATOR's bypass in `hasTenantAdminRole` is untouched and still
intentional — its 486 `group_rights` rows (433 now mapped) are read into
`operator.LegacyRights`/`operator.Permissions` for display/audit but are
never consulted by `hasPermission`/`scopeAllowed` because the role-name
bypass short-circuits first. 53 legacy right codes per group remain
genuinely unmapped (see Coverage above) and will stay inaccessible to
non-admin groups until a future pass extends the modern app to cover the
corresponding legacy features (Transactions/Accounting Vouchers, Payroll,
E-Prescription, etc.) or resolves the currently-ambiguous single-toggle
rights.
