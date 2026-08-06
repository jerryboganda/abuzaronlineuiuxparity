# AbuzarNext production cutover runbook

**Status:** Not approved for go-live  
**Runbook owner:** Release manager / Operations  
**Applies to:** AbuzarNext central API, PostgreSQL, branch edge, Windows client, and
the legacy WASEELA ABUZAR V3 fallback  
**Decision date:** 2026-08-06

> **Go-live is blocked.** Do not schedule the production switch until all open
> S0/S1 gaps are closed, the complete canonical migration is reconciled, physical
> devices are signed off, and functional and pixel acceptance are green. The
> current repository contains a runnable foundation and partial migration waves,
> not a replacement-ready production system.

## 1. Current release decision

The following are release blockers:

- **S0:** G1 data migration, G2 business logic, and G3 reports.
- **S1:** G4 contextual menus, G5 exact rights enforcement, G6 master-data
  parity, G7 transaction workflows, and G8 maintenance/manage workflows.
- Full canonical migration and reconciliation: the current canonical evidence
  covers masters/configuration, security, and bounded purchase-header slices;
  sales, detail lines, stock/batches, ledgers, and the remaining source tables
  remain open.
- Physical printer, label printer, scanner, cash drawer, biometric, SMS, and
  SMTP acceptance.
- Full-volume performance and the eight-hour soak.
- Complete functional UAT and the 1936x1048 pixel sweep.

Do not convert a passing automated test, a bounded performance proxy, a
deterministic ESC/POS golden, or an exact shell screenshot into a production
approval. Each proves only its stated scope.

Authoritative status and evidence:

- [Gap analysis](GAP_ANALYSIS_2026-08-06.md) and
  [implementation boundaries](IMPLEMENTATION_STATUS.md)
- [Phase E canonical evidence](../migration/PHASE_E_CANONICAL_STATUS_2026-08-06.md)
  and [historical-wave evidence](../migration/PHASE_E_HISTORICAL_STATUS_2026-08-06.md)
- [Parity status](PARITY_STATUS.md), [security evidence](../tmp/phase-r-backend-evidence.md),
  and [security UI evidence](../tmp/phase-r-security-ui-evidence.md)
- [Hardware evidence](PHASE_U_HARDWARE_EVIDENCE.md) and
  [performance evidence](PHASE_W_PERFORMANCE_EVIDENCE_2026-08-06.md)
- [Release artifacts and hashes](RELEASE_ARTIFACTS.md)

## 2. Roles and change control

No cutover step may proceed without a change record containing the target
tenant, branch, counters, release hash, freeze time, migration reports, backup
locations, named approvers, and rollback decision authority.

| Role | Responsibility |
|---|---|
| Release manager | Owns the go/no-go decision, timeline, and incident bridge |
| Migration operator | Runs the reviewed import and reconciliation commands |
| DBA | Confirms PostgreSQL backup/restore, roles, RLS, and database health |
| Branch operator | Confirms counters, shifts, terminals, printers, scanners, and cash drawers |
| UAT/business approver | Signs daily workflows, totals, reports, and pixel exceptions |
| Incident lead | Coordinates severity response and preserves evidence |

The release manager must record **GO**, **HOLD**, or **ROLLBACK** at each gate.
Silence is HOLD.

## 3. Operator prerequisites

### 3.1 Infrastructure and client

- Production PostgreSQL, API, HTTPS termination, branch edge, DNS, and
  monitoring are provisioned.
- PostgreSQL client tools (`psql`, `pg_dump`, and `pg_restore`) and Go are
  available to the migration operator.
- All migrations have been applied before the API starts. Use the existing
  [migration scripts](../ops/postgres/) rather than ad-hoc SQL.
- The target application role is non-owner and least privilege. The API must
  not inherit the schema-owner DSN.
- The branch edge has durable storage, a tested backup, and a configured
  synchronization route.
- Approved Windows client artifacts are available. Verify the SHA-256 values in
  [RELEASE_ARTIFACTS.md](RELEASE_ARTIFACTS.md) before installation.
- Every terminal has a named branch/counter, a supported Windows profile, a
  tested network path, and an assigned operator.

### 3.2 Protected configuration

Set these values in the deployment secret store or a protected operator shell.
Do not place values in this runbook, mapping JSON, screenshots, tickets,
command transcripts, or parity reports.

| Variable | Used by | Policy |
|---|---|---|
| `ABUZAR_SOURCE_SQLSERVER_URL` | Inspector/importer/reconciler | Canonical source; read-only connection |
| `ABUZAR_TARGET_POSTGRES_URL` | Importer/reconciler | Controlled migration connection; not the API DSN |
| `ABUZAR_ADMIN_DATABASE_URL` | Migrations/role provisioning | Schema-owner DSN; never inherited by the API |
| `ABUZAR_APP_DATABASE_URL` and `DATABASE_URL` | API | Non-owner application DSN only |
| `ABUZAR_APP_ROLE` | Role provisioning | Dedicated non-owner role |
| `ABUZAR_EDGE_CENTRAL_URL` / `ABUZAR_CENTRAL_API` | Branch edge | HTTPS central endpoint |
| `ABUZAR_EDGE_SHARED_SECRET` | Branch edge/client provisioning | Secret-store value; never print or return it |
| `ABUZAR_EDGE_CENTRAL_SESSION` | Optional edge synchronizer | Short-lived protected session material |
| `ABUZAR_COOKIE_SECURE` | API | `true`/`1` behind HTTPS |
| `ABUZAR_CUTOVER_TENANT_ID` / `ABUZAR_CUTOVER_BRANCH_ID` / `ABUZAR_CUTOVER_COUNTER_ID` | Import/reconciliation scope | Approved UUIDs; must match the change record |

Use environment expansion, not literal credentials:

```powershell
cd D:\ABUZAR\AbuzarNext
$env:ABUZAR_SOURCE_SQLSERVER_URL = '<protected-secret-store-value>'
$env:ABUZAR_TARGET_POSTGRES_URL = '<protected-secret-store-value>'
$env:ABUZAR_IMPORT_CONFIG = 'D:\secure-evidence\cutover\approved-final-map.json'
$env:ABUZAR_RECONCILE_METRICS = 'D:\secure-evidence\cutover\approved-final-metrics.json'
```

Do not use `Get-ChildItem Env:`, `Write-Host` on DSN variables, verbose process
logging, or a command-line literal containing a password. Clear migration-only
variables after the run. The importer and reconciler emit redacted connection
labels; retain those reports, but still review them before distribution.

### 3.3 Legacy safety

The canonical legacy application and `FazalDinPP19DataBaseV2` are read-only
reference systems throughout this runbook:

- Do not save, post, cancel, restore, or repair a transaction in the legacy app.
- Do not run DML, DDL, backup, security, or configuration changes against the
  canonical source from the migration workstation.
- Use `D:\ABUZAR\LegacyReferenceSandbox` and `AbuzarLegacyReference` for
  write-path experiments and migration rehearsals.
- Enforce the production freeze at the access-control/terminal boundary, not
  by editing the legacy application or database.
- Keep the legacy executable and database intact for lookback. Do not uninstall,
  overwrite, or repoint its data files.

## 4. Required release gates

Attach an artifact to every row. A verbal approval is not evidence.

| Gate | Required result |
|---|---|
| Migration inventory | Canonical inspector manifest covers the expected source inventory; the final reviewed map set accounts for all in-scope tables, with no unexplained unmapped tables |
| Counts | Every in-scope source/target count matches; zero duplicate or open exception rows |
| Business metrics | At least 12 reviewed read-only metrics for sales, purchases, returns, stock, receivables/payables, GL, taxes, totals, and invoice sequences; absolute difference `<= 0.01` where applicable |
| Historical replay | Approved historical documents reproduce price, discount, tax, stock, GL, and ledger results |
| Rights | All four legacy groups, 726 measured `GroupRights` rows, allow scopes, menu gating, API denies, and RLS checks pass for the target tenant |
| Hardware | Real-device print/label comparison, scanner-to-line-add, cash-drawer, and any used biometric/SMS/SMTP flows are signed by the branch operator |
| Performance | On full migrated volume: POS line-add `<150 ms`, post `<1 s`, heavy report p95 `<5 s`, and approved eight-hour soak |
| Pixel | All required screens, dialogs, loading/error/permission/report/print states at 1936x1048 are within the agreed tolerance or have a signed exception |
| Functional UAT | A parallel trading day, day-end reports, returns, purchases, permissions, backup/restore, offline recovery, and reconciliation are signed |
| Rollback rehearsal | Terminal repoint and recovery have been rehearsed without modifying the canonical legacy database |

The current Phase W synthetic fixture and current exact shell comparison are
useful evidence, but do not satisfy the full-volume or complete pixel gates.

## 5. D-1 preparation and rehearsal

1. Open the change record and record the release hashes, target scope, operator
   roster, terminal inventory, and escalation contacts.
2. Refresh the canonical source inventory without copying credentials into the
   manifest:

   ```powershell
   cd D:\ABUZAR\AbuzarNext
   $env:ABUZAR_SOURCE_SQLSERVER_URL = '<protected-read-only-canonical-DSN>'
   go run ./migration/cmd/inspect `
     -out 'D:\secure-evidence\cutover\source-schema.json'
   $manifest = Get-Content 'D:\secure-evidence\cutover\source-schema.json' |
     ConvertFrom-Json
   $tableCount = @($manifest.columns |
     Group-Object schema, table).Count
   if ($tableCount -ne 763) {
     throw "Expected 763 canonical base tables; observed $tableCount. HOLD."
   }
   ```

   A changed inventory is not automatically a failure, but it requires a
   reviewed map/risk decision before proceeding.
3. Apply target migrations from a protected schema-owner shell:

   ```powershell
   $env:ABUZAR_ADMIN_DATABASE_URL = '<protected-schema-owner-DSN>'
   powershell -ExecutionPolicy Bypass -File .\ops\postgres\apply-migrations.ps1
   ```

4. Provision/verify the application role, then remove
   `ABUZAR_ADMIN_DATABASE_URL` before starting the API:

   ```powershell
   $env:ABUZAR_APP_ROLE = 'abuzar_app'
   powershell -ExecutionPolicy Bypass -File .\ops\postgres\provision-app-role.ps1
   Remove-Item Env:ABUZAR_ADMIN_DATABASE_URL -ErrorAction SilentlyContinue
   ```

5. Run a sandbox rehearsal using only the reviewed maps bound to
   `AbuzarLegacyReference`. Do not substitute the canonical database.
6. Rehearse the complete migration, reconciliation, terminal switch, device
   checks, incident bridge, and mechanical rollback with disposable or sandbox
   data.
7. Verify the backup restore in a separate database. Never test restore over
   the production target.
8. Confirm all release gates in Section 4 are green. If any S0/S1 item remains
   open, stop and record **HOLD**.

## 6. Backup and restore

Backups must be encrypted, retained outside the primary host, access-controlled,
and periodically restored in a test environment. Do not store them under the
repository.

### 6.1 Pre-cutover target backup

Use a protected DSN and a secure path with restricted ACLs:

```powershell
$env:ABUZAR_BACKUP_DATABASE_URL = '<protected-target-DSN>'
$backup = 'D:\SecureBackups\AbuzarNext\cutover-pre-<UTC>.dump'
pg_dump --format=custom --file $backup $env:ABUZAR_BACKUP_DATABASE_URL
Get-FileHash $backup -Algorithm SHA256
Remove-Item Env:ABUZAR_BACKUP_DATABASE_URL -ErrorAction SilentlyContinue
```

Record the hash, backup completion time, PostgreSQL version, and restore owner
in the change record. The command must return success before freeze.

### 6.2 Restore rehearsal and emergency restore

Restore only to a disposable or explicitly approved target:

```powershell
$env:ABUZAR_RESTORE_DATABASE_URL = '<protected-disposable-or-approved-target-DSN>'
pg_restore --clean --if-exists --no-owner --dbname $env:ABUZAR_RESTORE_DATABASE_URL `
  'D:\SecureBackups\AbuzarNext\cutover-pre-<UTC>.dump'
Remove-Item Env:ABUZAR_RESTORE_DATABASE_URL -ErrorAction SilentlyContinue
```

An emergency restore requires the release manager and DBA to stop all API/edge
writes, preserve the post-backup event/log evidence, verify the backup hash,
restore, run migrations/health checks, and explicitly approve repointing
clients. Never restore over a live target while terminals are posting.

## 7. Cutover procedure

### 7.1 T-60 to T-15: pre-freeze checks

1. Announce the maintenance window and open the incident bridge.
2. Confirm all operators have closed or documented open shifts, cash totals,
   pending returns, purchase receiving, printer queues, and edge queues.
3. Confirm no migration or reconciliation process is running from another
   workstation.
4. Verify the target:

   ```powershell
   Invoke-RestMethod 'https://<approved-api-host>/v1/health'
   Invoke-RestMethod 'https://<approved-edge-host>/v1/health'
   ```

   Both must report `status=ok` and `database=ok`.
5. Capture a baseline from the central `GET /v1/metrics` endpoint and the
   provider's structured logs. `/v1/metrics` is process-local observability; it
   is not business reconciliation evidence.
6. Check the branch edge queue with the authenticated endpoint. Keep the bearer
   value in the protected environment only:

   ```powershell
   $headers = @{ Authorization = "Bearer " + $env:ABUZAR_EDGE_SHARED_SECRET }
   Invoke-RestMethod 'https://<approved-edge-host>/v1/sync/status' -Headers $headers
   ```

   `pendingEvents` must be zero unless the release manager has approved and
   recorded the queue disposition.

### 7.2 T-15: freeze

1. Announce **LEGACY WRITE FREEZE**. Remove write-capable access from legacy
   terminals using the approved access-control procedure; do not change the
   legacy executable or database.
2. Record the UTC freeze timestamp, operator acknowledgements, last source
   sequence/invoice per document family, and the final source count snapshot.
3. Leave the legacy app and database available for read-only lookback only.
4. Confirm no open source transaction can be posted. If this cannot be proven,
   record **HOLD** and do not migrate.

### 7.3 T-15 to T+30: final incremental migration

The importer is idempotent and supports reviewed mapping ranges, but it does
not infer a safe business watermark. The final map must explicitly define the
approved source window from the last reconciled watermark through the freeze
timestamp. Do not invent a timestamp column, source predicate, branch mapping,
or table range at the console.

The current `migration/maps/phase-e-historical-*.json` files are partial
rehearsal/reference maps; they are not a complete canonical final-incremental
map. The following is the executable command form once the reviewed final map
and metrics files exist outside the repository:

```powershell
cd D:\ABUZAR\AbuzarNext
$env:ABUZAR_SOURCE_SQLSERVER_URL = '<protected-read-only-canonical-DSN>'
$env:ABUZAR_TARGET_POSTGRES_URL = '<protected-migration-target-DSN>'
$env:ABUZAR_IMPORT_CONFIG = 'D:\secure-evidence\cutover\approved-final-map.json'

go run ./migration/cmd/import `
  -config $env:ABUZAR_IMPORT_CONFIG `
  -allow-canonical `
  -tenant-id $env:ABUZAR_CUTOVER_TENANT_ID `
  -branch-id $env:ABUZAR_CUTOVER_BRANCH_ID `
  -counter-id $env:ABUZAR_CUTOVER_COUNTER_ID `
  -out 'D:\secure-evidence\cutover\final-incremental-import.json'
```

Run each approved wave in dependency order: reference/configuration, masters,
documents and lines, returns/orders, stock/batches, ledgers/GL, tax, and
security. For a reviewed resumable range, use `-from-table` and `-to-table`.
Use `-upsert` only when the change record explicitly authorizes a reviewed
rerun. Do not pass an unreviewed `-source-filter`.

Stop if any import report contains an exception, unexpected duplicate,
unmapped identifier, scope injection error, or count that differs from the
approved source snapshot.

### 7.4 T+30 to T+60: final reconciliation

Run the same reviewed map and metric set against the frozen source and target:

```powershell
$env:ABUZAR_RECONCILE_METRICS = 'D:\secure-evidence\cutover\approved-final-metrics.json'

go run ./migration/cmd/reconcile `
  -config $env:ABUZAR_IMPORT_CONFIG `
  -tenant $env:ABUZAR_CUTOVER_TENANT_ID `
  -allow-canonical `
  -branch-id $env:ABUZAR_CUTOVER_BRANCH_ID `
  -counter-id $env:ABUZAR_CUTOVER_COUNTER_ID `
  -out 'D:\secure-evidence\cutover\final-reconciliation.json'
```

The report must contain only `matched` table/metric results, zero exceptions,
zero unexplained rows, and all business differences within the approved
tolerance. Review invoice-number ranges, stock by item/godown/batch, customer
and supplier balances, GL balance, tax totals, and day-end totals separately.
The release manager signs **GO** only after the DBA and business approver sign
this report.

## 8. Terminal repoint

### 8.1 Install the approved client

Verify the selected installer against
[RELEASE_ARTIFACTS.md](RELEASE_ARTIFACTS.md), then use the approved endpoint
management method. Example for an approved MSI deployment:

```powershell
$msi = 'D:\ABUZAR\AbuzarNext\apps\desktop\src-tauri\target\release\bundle\msi\Abuzar Next_0.1.0_x64_en-US.msi'
Get-FileHash $msi -Algorithm SHA256
Start-Process msiexec.exe -Wait -ArgumentList @('/i', $msi, '/qn', '/norestart')
```

The installer contains no PostgreSQL credentials, SQL Server drivers, license
keys, or device fingerprints. Provision the approved central API/edge URL and
device settings through the deployment secret store or Windows Credential
Manager. Do not put them in a shortcut, repository file, or installer
argument.

### 8.2 Switch one pilot terminal

1. Close the legacy client and preserve its shortcut as `LEGACY - READ ONLY`.
2. Start the approved AbuzarNext client on one pilot terminal.
3. Select the assigned tenant, branch, counter, and operator.
4. Verify login, `/v1/health`, branch-edge health, `/v1/sync/status`, and
   `/v1/hardware/capabilities`.
5. Run the approved pilot smoke transaction, print it, verify the drawer, and
   confirm the transaction/report/reconciliation evidence.
6. Switch remaining terminals in batches. Record terminal, operator, time, and
   result in the change record.

There is no repository script that performs a production fleet repoint. Do not
create an unreviewed shortcut or configuration script during the window.

## 9. Rollback and recovery

### 9.1 Rollback triggers

Declare **ROLLBACK** for data loss or duplication, incorrect totals, failed
posting/stock/ledger invariants, authentication or rights bypass, unavailable
hardware required for trading, unrecoverable API/edge failure, or an S0/S1
defect that cannot be contained within the agreed decision window.

### 9.2 Mechanical terminal fallback

1. Stop new AbuzarNext posts and preserve API/edge logs, metrics, queue state,
   screenshots, request IDs, and the current database backup.
2. Mark all terminals `ROLLBACK IN PROGRESS`; do not uninstall the new client.
3. Repoint a terminal to the intact legacy executable only for approved
   read-only investigation:

   ```powershell
   Start-Process `
     -FilePath 'D:\ABUZAR\V2_AbuzarSoftware\Application\abuzar.exe' `
     -WorkingDirectory 'D:\ABUZAR\V2_AbuzarSoftware\Application'
   ```

4. Keep the legacy database read-only. The canonical policy does **not** permit
   silently resuming legacy trading after new transactions have been posted:
   AbuzarNext events are not automatically written back to SQL Server.
5. If business continuity requires trading after rollback, the release manager,
   DBA, and business owner must approve a separate controlled transaction
   recovery/re-entry procedure. Until that approval exists, hold trading and
   use read-only lookback.
6. If the target is corrupted, restore the pre-cutover PostgreSQL backup only
   after stopping all writers and preserving post-backup events. Re-run health,
   RLS, reconciliation, and pilot checks before repointing to AbuzarNext.

Rollback is a controlled safety action, not permission to delete new rows,
rewrite audit events, or write the legacy source.

## 10. First 48 hours after switch

| Window | Required checks |
|---|---|
| T+0–15 min | Pilot login/context, API and edge health, queue zero, one sale/return smoke, print/drawer/scanner |
| T+15–60 min | Each terminal online, no duplicate invoice, no 5xx/auth spikes, first transaction and stock/ledger assertions |
| Hours 1–4 | Monitor every 15 minutes; review `/v1/metrics`, structured errors, sync cursor, hardware failures, and operator reports |
| Hours 4–24 | Monitor hourly; run approved sales, purchase, return, report, permission, offline/reconnect, and backup checks |
| Hours 24–48 | Monitor at least every two hours; compare day-end totals, stock, receivables/payables, GL, tax, invoice ranges, and exception counts |
| T+48 h | Business owner signs the first live day; release manager closes the rollback window or extends it with a new decision record |

The legacy installation remains intact and read-only for lookback after T+48.
Retain the pre-cutover backup, final migration/reconciliation reports, day-end
reports, terminal checklist, hardware signoff, incidents, and monitoring
exports according to the approved retention policy.

## 11. Incident severity and escalation

| Severity | Examples | Response |
|---|---|---|
| S0 | Data loss/duplication, wrong financial totals, unsafe stock, auth/RLS bypass, unrecoverable posting | Stop posts immediately; incident lead and release manager on bridge; preserve evidence; decide rollback/hold |
| S1 | A core sale/purchase/return, report, permission, or branch operation is unusable for trading | Contain within 15 minutes; notify business owner/DBA; no further terminal rollout; rollback decision |
| S2 | One device, report format, screen, or non-critical workflow is degraded with a safe workaround | Record request IDs and evidence; assign owner; continue only with release-manager approval |
| S3 | Cosmetic/documentation issue without operational impact | Log for normal remediation; does not override a separate gate |

Escalate immediately on repeated 5xx responses, failed health/database status,
non-zero edge queue growth without an approved cause, invoice collision,
reconciliation drift, hardware adapter failure on a required device, or any
security/permission discrepancy. Do not include passwords, DSNs, cookies,
shared secrets, or customer payloads in the incident channel.

## 12. Post-cutover acceptance record

Close the change only when the evidence register contains:

- Signed gate matrix from Section 4.
- `final-incremental-import.json` and `final-reconciliation.json`, with the
  reviewed source snapshot and metric configuration stored securely.
- Pre-cutover backup hash and successful restore-rehearsal result.
- API/edge health and metrics baselines plus 48-hour monitoring export.
- Terminal-by-terminal repoint and pilot smoke results.
- Physical device signoff and printer/scanner/drawer evidence.
- Day-end business reconciliation and UAT approval.
- Pixel comparison reports produced by the existing tools:

  ```powershell
  powershell -ExecutionPolicy Bypass -File .\parity\tools\capture-window.ps1 `
    -ProcessId <approved-process-id> `
    -OutputPath 'D:\secure-evidence\cutover\candidate.png'

  powershell -ExecutionPolicy Bypass -File .\parity\tools\compare-png.ps1 `
    -Reference 'D:\secure-evidence\cutover\reference.png' `
    -Candidate 'D:\secure-evidence\cutover\candidate.png' `
    -ReportPath 'D:\secure-evidence\cutover\pixel-comparison.json'
  ```

The comparison command must exit successfully and meet the approved tolerance;
any exception must be individually signed. A green cutover record without all
of these artifacts is not final acceptance.
