# Legacy runtime accessibility audit — 2026-08-05

## Result

The legacy PowerBuilder shell and a representative set of workflows are now
accessible locally through an isolated reference sandbox. This is a controlled
runtime audit, not a claim that the whole application is 100% complete or
pixel-approved. The canonical installation and its database were left
untouched.

## Isolation boundary

- Canonical fallback: `D:\ABUZAR\V2_AbuzarSoftware`.
- Sandbox application: `D:\ABUZAR\LegacyReferenceSandbox\Application`.
- Sandbox database: SQL Server database `AbuzarLegacyReference` with data files
  under `D:\ABUZAR\LegacyReferenceSandbox`.
- The reference database backup and sandbox credentials remain outside
  `AbuzarNext`; no password, license value, or customer record is copied into
  this repository.
- The sandbox process was stopped after capture. No `abuzar.exe` process is
  left running.

As a final database-boundary check, the canonical `FazalDinPP19DataBaseV2`
still contains the original `SP_WayToMoon` `xp_cmdshell` body, while the
isolated `AbuzarLegacyReference` copy contains the no-op harness. This is the
expected proof that the startup workaround did not alter the fallback system.

## What was verified

### Startup and shell

1. The canonical executable opened its native login window. Its original first
   startup attempt stopped at the application `Database Problem` dialog.
2. A copy-only SQL backup was restored as `AbuzarLegacyReference`.
3. In the clone only, `SP_WayToMoon` was replaced with a no-op because the
   startup procedure calls `xp_cmdshell` to probe legacy DLLs that are absent
   on this machine and `xp_cmdshell` is disabled. The canonical procedure was
   not changed.
4. The sandbox executable's encrypted runtime configuration was changed only
   in the sandbox copy so it selected `AbuzarLegacyReference`.
5. The sandbox was launched with its application directory as the working
   directory, logged in with the existing ADMIN row, and reached the real
   maximized shell. SQL sessions were observed using
   `AbuzarLegacyReference`, proving that the clone—not the canonical database—
   was active.
6. The main shell menu was recursively enumerated: **9 top-level menus and
   275 menu entries**. The complete path/command catalogue is
   `parity/catalog/legacy-menu-tree-2026-08-05.json`.

### Captured shell states

- Native main shell: `parity/captures/legacy/sandbox-main-window-2026-08-05-rerun.png`
- Startup backup-device, backup-information, and backup-database dialogs:
  `sandbox-backup-device-2026-08-05-rerun.png`,
  `sandbox-backup-information-2026-08-05-rerun.png`,
  `sandbox-backup-database-2026-08-05-rerun.png`
- File/Change User confirmation:
  `sandbox-change-user-confirmation-2026-08-05.png`
- Top-level menu captures exist for File, Purchase, Sales, Reports, Basic Data,
  Maintenance, Manage, Window, and Help under
  `parity/captures/legacy/desktop-*-menu-open-2026-08-05.png`.

### Representative workflows

The following screens were opened read-only in the clone and captured without
saving a business transaction:

- Master data: Customer, Supplier, Item, and User forms.
- Transactions: Cash Sale and Pack Purchase.
- Reports: Daily Sales Detail format selector, retrieval arguments, and the
  report's loading state.
- Maintenance: Backup Database and Database Integrity Monitor.
- Preferences: the General, Sale, Sale Return, Purchase, Purchase Return,
  Report, BasicData, Quotation, Schedule, Adjustment, Purchase Order, Others,
  Point of Sale, Cashier Job Activity, Email, SMS, and Dashboard tabs.

The corresponding files are grouped under
`parity/captures/legacy/sandbox-*.png`.

## Defects and compatibility blockers found

These are evidence-backed findings from the running copy. They are not fixed
in the canonical application or system database.

1. **Legacy startup probe.** `SP_WayToMoon` depends on `xp_cmdshell` and
   `systemab.dll`/`tapi161.dll`. This machine has `xp_cmdshell` disabled and
   those DLLs are not present. The isolated no-op procedure is only a reference
   harness so the UI could be inspected.
2. **SQL Agent metadata compatibility.** Preferences opens after dismissing a
   `Job` warning, but its old query against `msdb.dbo.SysJobs` and
   `SysJobSchedules` references columns (`Name`, `freq_subday_type`,
   `freq_subday_interval`, `active_start_time`) that this SQL Server version
   does not expose. The captured error is
   `parity/captures/legacy/sandbox-preferences-job-error-2026-08-05.png`.
   The temporary Extended Events session used to identify the query was
   stopped and removed; `msdb` was not modified.
3. **Pack Purchase close crash.** Closing the Pack Purchase window raises
   PowerBuilder R0002: null object reference at line 1 in `wf_disconnect` of
   `w_multi_entry`. Evidence:
   `sandbox-pack-purchase-r0002-error-2026-08-05.png`.
4. **Database Integrity close crash.** Running the clone's Check Now executes
   `DBCC CHECKDB` successfully to completion, but closing the monitor raises
   PowerBuilder R0002: null object reference at line 15 in `wf_checkdb` of
   `w_checkdb`. Evidence:
   `sandbox-database-integrity-close-r0002-2026-08-05.png`.
5. **Launch-directory sensitivity.** Starting the copied executable without
   its application directory as the working directory makes it read the wrong
   runtime configuration and produces SQLSTATE 08001. Launching from the
   sandbox application directory reaches the login and shell. Evidence of the
   wrong-directory failure is retained as
   `sandbox-relaunch-database-connection-error-2026-08-05.png`.

## Completion status

The legacy main window is locally accessible and its menu/workflow surface is
now catalogued enough to start parity implementation. **100% workflow and
pixel parity is not yet proven.** Remaining acceptance work includes every leaf
menu workflow, keyboard/focus/validation permutations, all report and print
outputs, hardware integrations (printer, barcode, cash drawer, biometric),
permission/role states, data reconciliation, and the Chrome/Tauri comparison
against this captured baseline. The two R0002 defects and the SQL Agent query
compatibility issue must be resolved or explicitly designed around before any
module can be marked production-complete.
