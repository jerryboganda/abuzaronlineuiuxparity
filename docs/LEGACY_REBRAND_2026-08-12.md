# Legacy PowerBuilder App Rebrand — "Bismillah Pharmacy Software"

**Date:** 2026-08-12
**Scope:** The **legacy PowerBuilder desktop application** only (installed at the
Windows install root, historically `D:\ABUZAR`). The web parity project in this
repository was explicitly **out of scope** and was not rebranded.

Goal: remove every trace of the previous brand ("Abuzar", "Waseela") from the
legacy desktop product and replace it with **Bismillah Pharmacy Software**.

---

## 1. Why this was unusual

The legacy application has **no source code available** — there is no `.pbw`,
`.pbt`, or `.pbl` anywhere on the machine. What exists is:

- compiled `.exe` and `.pbd` binaries (PowerBuilder 12.5),
- a live ~2.5 GB SQL Server production database,
- loose scripts, shortcuts, a scheduled task, and registry entries.

So the rebrand had to be performed **against compiled binaries**, not source.

### The hard constraint: PBD strings are length-prefixed

PowerBuilder `.pbd` string-pool entries are stored as:

```
[ 4-byte little-endian length ][ UTF-16LE character data ]
length = (character_count * 2) + 2
```

Because the length prefix is not recomputed by anything at runtime, **the byte
length of every replaced string must stay identical**. A single byte of drift
desynchronises the pool and the library fails to load.

Consequence: replacements inside binaries are **same-length, space-padded**, and
the file size never changes. This is why several identifiers below look
truncated — they are length-constrained on purpose.

| Original  | Replacement | Why |
|-----------|-------------|-----|
| `abuzar`  | `bismil`    | 6 chars, must equal 6 |
| `waseela` | `bismila`   | 7 chars, must equal 7 |

Free-form **text** files (`.ps1`, `.sql`, `.md`, `.ini`) have no such constraint
and use the full proper names.

---

## 2. What was changed

### Binaries
- **55 files patched, 1812 string replacements**, all same-length.
- `AbuzarWaseela.exe` → **`BismillahPharmacy.exe`** (including the internal
  library-list entry inside the EXE header).
- Application title literal now renders `BISMILLAH PHARMACY SOFTWARE V3...`.

### Database
- Application database, all objects, module bodies, one column and one default
  constraint rebranded.
- **Server-level objects** (found late — these live outside the app database):
  - `master.dbo.usp_Abuzar_PreWarmCache` → `usp_Bismillah_PreWarmCache`,
    preserving `is_auto_executed = 1` (startup procedure).
  - Extended Events session `abuzar_errors` → `bismillah_errors`
    (XEvent sessions cannot be renamed; dropped and recreated).
- **8 sandbox / performance-clone databases** rebranded non-destructively
  (no database dropped, no row data altered).

### Filesystem / OS integration
- Install subfolder → `V2_BismillahSoftware`.
- Start-menu and desktop shortcuts, the Windows scheduled task
  (`Bismillah-DB-Weekly-Maintenance`), and registry entries.
- Logo relocated to a brand-free, rename-proof path:
  `C:\ProgramData\BismillahPharmacy\Logo1.bmp`.

---

## 3. Identifiers deliberately **kept**

These are *not* brand traces. They are real identifiers that must continue to
match the strings compiled into the patched binaries. Changing them would break
the application.

| Identifier | Reason |
|---|---|
| `bismilapp.pbd` | Actual PBD filename + PowerBuilder library name (length-constrained) |
| `bismil2000`, `c:\bismila_temp\` | Length-constrained runtime identifiers |
| `;APP=Bismila;` | DB connection *Application Name*, patched into the binaries |
| `SP_BismilaMini_*`, `VIEW_BismilaMini_*` | DB objects the patched PBDs call by name |
| `DF_CustomerDeploymentDetail_Bismila`, column `Bismila` | Referenced by compiled SQL |

> This is why the sandbox databases were rebranded `Waseela` → **`Bismila`**
> (length-matched) rather than `Bismillah`.

---

## 4. Verification / evidence

**Static sweep** of the whole install tree (excluding the SQL data directory,
backup archives, and the out-of-scope web project):

```
NAME HITS (files/dirs) : 0
CONTENT HITS           : 3
files where EVERY hit is the install-root path : 3
files with NON-root brand text                 : 0
```

All three remaining files match only the literal install-root path string, which
is genuine until step 6 below is performed.

**Database sweep** across every online database plus server-level objects:

```
Brand traces across all online databases : 0
Databases online                         : 15
Startup proc renamed + still auto-exec   : 1
XEvent session renamed                   : 1
```

**Runtime smoke test (passed end-to-end):**
- login → main window title bar reads `BISMILLAH PHARMACY SOFTWARE V3...`
- a **2,703-page** purchase report rendered
- a **146-page** sale-invoice reprint rendered

---

## 5. Regressions found and fixed during verification

The length-constrained token map (`abuzar`→`bismil`) was correct for binaries but
was **wrongly applied to free-form text files** in an earlier pass. That produced
paths and lookups which matched *neither* the current install root *nor* the
post-rename root:

| Broken artefact | Fix |
|---|---|
| `D:\BISMIL\...` literals | Derived from `$PSScriptRoot` |
| `V2_BismilSoftware` | `V2_BismillahSoftware` |
| `bismil.exe` | `BismillahPharmacy.exe` |
| `Get-Process bismil` | `Get-Process BismillahPharmacy` |
| window title match `*BISMILA*` | `*BISMILLAH*` — note **"BISMILLAH" does not contain "BISMILA"** (after `BISMIL` comes `L`, not `A`), so the old guard never matched |
| hardcoded clone/data paths in `Verify-LegacyDatabase.sql` | derived from `sys.master_files` with a `THROW 50010` guard |

All paths are now derived at runtime, so the diagnostics tooling works **both
now and after** the pending root rename. The environment collector was re-run
and returns `"Status": "PASS"`.

---

## 6. Remaining step (requires an administrator + maintenance window)

The install root folder itself is still named after the old brand. It is not
renamed automatically because **it hosts the SQL Server instance**, so the rename
must also move 35 `sys.master_files` rows across 14 databases, three service
`ImagePath` values, and the instance registry hive.

`ops/legacy-rebrand/Complete-RootRename.ps1` performs this. Always dry-run first:

```powershell
Start-Process powershell -Verb RunAs
& '<INSTALL_ROOT>\Complete-RootRename.ps1' -WhatIfOnly   # inspect the plan
& '<INSTALL_ROOT>\Complete-RootRename.ps1'               # execute
```

It writes its own `rollback.sql`, fails fast, and uses single-user master
recovery (`net start /f /T3608 /mSQLCMD`) with a fallback path.

> ⚠️ Renaming the folder that contains a SQL Server instance is **not a
> Microsoft-supported instance relocation** (Windows Installer metadata, the
> Resource DB, and `Binn` all record the original path). Take a full backup
> first. This trade-off is documented in the script header.

Regenerate `environment-report.json` afterwards.

---

## 7. Deliberate exclusions

1. **The install-root folder** — see step 6; script provided.
2. **`SaleLedger.Remarks` (34 rows)** — the old brand appears there as a
   *person's name* inside financial records. Editing financial history is not
   rebranding, so these were left untouched.
3. **The web parity project in this repository** — explicitly out of scope.
4. **Database backup files** (`.bak` / `.mdf` / `.ldf`, ~52 GB) — excluded from
   byte-patching.

---

## 8. Pre-existing issues (NOT caused by the rebrand)

### SQL error 8127 on Reports ▸ RePrinting ▸ Sale

`saledetail.SaleInvcode` is invalid in the `ORDER BY` because it is not in the
`GROUP BY`. **Proven pre-existing** by extracting every `ORDER BY` clause
referencing that column from the pre-patch backups and comparing against the
patched binaries:

```
267 clauses in patched files
267 byte-identical (same file + same MD5) to the originals
  0 altered      0 only-in-original
```

It **cannot** be fixed by binary patching: the fix requires adding a column to
the `GROUP BY`, which changes the string length and violates the length
constraint. It needs the PowerBuilder source.

### Cleartext `sa` password in `maintenance\run_maintenance.ps1`

Pre-existing; unrelated to the rebrand. Recommend moving to a credential store
or a least-privileged login. None of the tooling committed here contains a
credential — all scripts read it at runtime or prompt via
`Read-Host -AsSecureString`.

---

## 9. Backups / rollback

Everything is reversible from `_RENAME_BACKUP_20260812\` under the install root:

| Path | Contents |
|---|---|
| `Application\` | 467 original files, 788 MB |
| `FULLTREE\` | the 55 patched files, pre-patch |
| `ROOTFILES\` | pre-patch root-level scripts |
| `db\PreRebrand_*.bak` | pre-rebrand database backup |
| `*.ORIGINAL.sql` / `*.PATCHED.sql` | 42 side-by-side module definitions |
| `DIAGFIX\` | the 7 diagnostics source files before the path fixes |

Server-level database objects revert via
`ops/legacy-rebrand/ROLLBACK_serverlevel.sql`.

---

## 10. Tooling committed with this document

Under `ops/legacy-rebrand/`:

| File | Purpose |
|---|---|
| `Complete-RootRename.ps1` | Elevated install-root rename incl. SQL instance relocation |
| `Rebrand-SandboxDbs.ps1` | Non-destructive per-database rebrand (`-WhatIfOnly` supported) |
| `ROLLBACK_serverlevel.sql` | Reverts the startup proc + XEvent session renames |
| `rebrand2.py` | Same-length UTF-16LE binary patcher |
| `db_rebrand.py` | Database object/data rebrand |
| `db_full_scan.py`, `scan_brand.py`, `classify_brand.py` | Verification sweeps |

> `Rebrand-SandboxDbs.ps1` searches for the `waseela` token only — extend it if
> reused for a different token.

### Operational notes discovered the hard way

- Integrated auth is rejected on this instance ("login is from an untrusted
  domain"); only SQL authentication works.
- Concatenating `sys.*` name columns with string literals raises **Msg 451**
  (collation conflict). Return separate columns instead of concatenating.
- `sqlcmd` defaults to 80 columns and **silently wraps** — always pass
  `-w 65535` when printing paths.
- `sp_rename` renames a module but does **not** update `sys.sql_modules.definition`
  — you must `DROP` + `CREATE` to clear the old name from the body.
- Views validate referenced objects at `CREATE` time, procedures use deferred
  name resolution — therefore recreate **views first, then procedures**.
