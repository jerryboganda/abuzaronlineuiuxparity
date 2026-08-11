# Legacy Rebrand Tooling

Tooling used to rebrand the **legacy PowerBuilder desktop application** to
**Bismillah Pharmacy Software**.

Full write-up, evidence, and rollback instructions:
[`docs/LEGACY_REBRAND_2026-08-12.md`](../../docs/LEGACY_REBRAND_2026-08-12.md)

> These scripts operate on a **live production machine** (compiled binaries, a
> running SQL Server instance, services, and the registry). Read the runbook
> before running anything, and take a full backup first.

## Contents

| File | Purpose |
|---|---|
| `Complete-RootRename.ps1` | Renames the install root, including SQL instance relocation (35 `sys.master_files` rows, 3 service `ImagePath`s, registry hive). **Requires elevation.** Supports `-WhatIfOnly`. |
| `Rebrand-SandboxDbs.ps1` | Non-destructive per-database rebrand: drop procs → drop views → rename column → rename constraint → recreate views → recreate procs. Supports `-WhatIfOnly`. |
| `ROLLBACK_serverlevel.sql` | Reverts the startup procedure and Extended Events session renames. |
| `rebrand2.py` | Same-length UTF-16LE binary patcher for `.pbd` / `.exe`. |
| `db_rebrand.py` | Database object and data rebrand. |
| `db_full_scan.py` | Server-wide database brand-trace scan. |
| `scan_brand.py` | Filesystem brand-trace scan. |
| `classify_brand.py` | Classifies scan hits into real brand text vs. install-root path noise. |

## Critical constraint

PowerBuilder `.pbd` string-pool entries are length-prefixed
(`length = chars * 2 + 2`, UTF-16LE). **Replacements inside binaries must be
exactly the same byte length** and the file size must never change. This is why
`rebrand2.py` space-pads and refuses non-matching lengths, and why some patched
identifiers look truncated.

Free-form text files have no such constraint and use the full proper names. Do
**not** reuse the length-constrained token map on text files — doing so produces
paths that match neither the old nor the new install root.

## Credentials

No script here contains a credential. They read the password at runtime from the
application preferences file, or prompt with `Read-Host -AsSecureString`.

## Order of operations

1. Back up (binaries + all databases).
2. `scan_brand.py` / `db_full_scan.py` — establish a baseline.
3. `rebrand2.py` — patch binaries.
4. `db_rebrand.py`, then `Rebrand-SandboxDbs.ps1` for sandbox/clone databases.
5. Smoke test the application end to end.
6. `Complete-RootRename.ps1 -WhatIfOnly`, then for real (elevated, maintenance
   window).
7. Re-run the scans and regenerate `environment-report.json`.
