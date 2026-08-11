# Legacy SQL Server (PowerBuilder app) — performance tuning & DB fixes

Tuning kit for the **legacy** PowerBuilder 12.5 pharmacy application
(`abuzar.exe`, 32-bit, SQLOLEDB) running against SQL Server
(`FazalDinPP19DataBaseV2`).

This is **not** part of the AbuzarNext runtime. It is kept here so the same
fixes can be replayed at client sites and so the parity rewrite inherits the
knowledge of what was actually wrong with the legacy schema.

Client environments are assumed to be **HDD-backed** and often
**multi-PC / multi-branch**, so nothing here depends on SSD storage.

---

## 1. Root causes found

### 1.1 Stale statistics + severe index fragmentation (whole app slow)

| Symptom | Evidence |
| --- | --- |
| Every screen/report took "ages" | Statistics last updated **2022-02-24**, with **7.1M** row modifications since |
| Full scans instead of seeks | Indexes **73–99% fragmented** |
| High physical read latency | DB + tempdb on HDD, **77 ms** average read stall |

### 1.2 `Fn_GetPreference` family was non-SARGable (the big one)

The three preference-lookup scalar UDFs did this:

```sql
WHERE LOWER(Name) = LOWER(@prefname)
```

Wrapping the **column** in `LOWER()` makes the predicate non-SARGable, so
`PK_SoftwarePreferences_Name` could not be used and **every call scanned the
whole table**. These UDFs are invoked *per row* inside report DataWindows.

Measured on a single "Daily Sales Detail" retrieve:

```
SoftwarePreferences | UK_SoftwarePreferences_PrefID | scans:   23,598
SoftwarePreferences | PK_SoftwarePreferences_Name   | lookups: 23,572
```

The `LOWER()` calls were pure waste: the column collation is
`SQL_Latin1_General_CP1_CI_AS` (**case-insensitive**), and
`COUNT(*) = COUNT(DISTINCT LOWER(Name)) = 1363`, so removing them is
behaviour-preserving.

**Result after fix (`fix_01`):**

| Metric | Before | After |
| --- | --- | --- |
| 2,000 UDF calls | 1,499 ms | 50 ms (**30× faster**) |
| Index access for 2,000 calls | 2,000 scans + 2,000 lookups | **2,000 seeks, 0 scans** |
| "Daily Sales Detail" DB time (1 month, 1,654 pages) | ~25 s | **4.4 s** |

### 1.3 Weak page verification

`PAGE_VERIFY` was `TORN_PAGE_DETECTION` (deprecated, misses most corruption).
Set to `CHECKSUM` in `fix_02`.

### 1.4 86 untrusted foreign keys

All FKs were `is_not_trusted = 1`, so the optimizer could not use them for join
elimination. `fix_03` re-validated **all 86 with `WITH CHECK`** — every one
passed, confirming **zero orphaned rows**.

### 1.5 Missing indexes for the real report workload

Added in `fix_02`, derived from `sys.dm_db_missing_index_*` after capturing the
live report SQL via Extended Events:

* `IX_SaleLedger_SaleCat_Date_Cover` — covers `WHERE date BETWEEN … AND salecatcode IN (…)`
* `IX_Item_Active_ICatCode` — 92% estimated gain, tiny
* `IX_PurOrderHeader_Supp_Posted`

---

## 2. NOT a database fault: the "DataBase Error … Row No: 0" dialog

Reported symptom: opening **Report → Daily Reports → Sale → Sale detail**
produced:

```
Error Code: 102
Select Error: SQLSTATE = 42000
Microsoft OLE DB Provider for SQL Server
Incorrect syntax near ')'.
```

Extended Events captured the exact statement the app sent:

```sql
... AND dbo.saleledger.custcode IN (
        SELECT C.CustCode FROM Customer C WHERE C.AreaCode IN ()
    ) ...
```

**The application emits an empty `IN ()` list when no Area is selected** in the
*Specify Retrieval Arguments* dialog. That is invalid T-SQL, hence error 102.
It is a client-side input-validation bug, not data corruption.

**Verified workaround:** tick **All** (or move at least one area into
*Selected Areas*) — the same report then retrieved **1,654 pages** with no error.

**Proper fix** belongs in the PowerBuilder source: when the area list is empty,
omit the `AND custcode IN (…)` predicate entirely rather than emitting `IN ()`.
No source (`.pbl`) is available in this deployment — only compiled `.pbd`
libraries — so it is documented here rather than patched.

---

## 3. Files

| File | Purpose |
| --- | --- |
| `fix_01_preference_udfs.sql` | Removes non-SARGable `LOWER()` from the 3 preference UDFs |
| `fix_02_settings_and_indexes.sql` | `PAGE_VERIFY CHECKSUM` + 3 workload indexes + stats refresh |
| `fix_03_trust_foreign_keys.sql` | Re-validates untrusted FKs; reports (never deletes) orphans |
| `weekly_maintenance.sql` | Smart rebuild (>30%) / reorganize (5–30%) + statistics refresh |
| `run_maintenance.ps1` | Task Scheduler entry point for the weekly job |
| `install_prewarm.sql` | Startup proc that pre-warms hot tables into buffer cache |
| `rollback/prefs_udf_original.sql` | Original UDF definitions, for rollback |

All fix scripts are **idempotent** — safe to re-run.

---

## 4. Applying at a client site

```powershell
# 0. ALWAYS take a verified backup first.
#    BACKUP DATABASE ... WITH COMPRESSION, CHECKSUM;  then RESTORE VERIFYONLY.

$env:ABUZAR_SQL_SERVER   = '127.0.0.1'
$env:ABUZAR_SQL_DATABASE = 'FazalDinPP19DataBaseV2'
$env:ABUZAR_SQL_USER     = '<user>'
$env:ABUZAR_SQL_PASSWORD = '<password>'

sqlcmd -S $env:ABUZAR_SQL_SERVER -d $env:ABUZAR_SQL_DATABASE `
       -U $env:ABUZAR_SQL_USER -P $env:ABUZAR_SQL_PASSWORD -I -b `
       -i fix_01_preference_udfs.sql

# repeat for fix_02_settings_and_indexes.sql and fix_03_trust_foreign_keys.sql
# then install_prewarm.sql (optional), and schedule run_maintenance.ps1 weekly.
```

> `-I` (QUOTED_IDENTIFIER ON) is **required**: `sqlcmd` defaults it OFF, which
> breaks `WITH SCHEMABINDING` functions and XML/XQuery.

### Server / OS settings also applied (not scripted here)

* `cost threshold for parallelism` → 50 (was 5)
* `max degree of parallelism` → 8 (was 12 on 36 cores)
* `max server memory` → 24 GB (was unlimited on a 32 GB box)
* `optimize for ad hoc workloads` → 1
* Data file growth → 256 MB, log growth → 128 MB (were 10%)
* tempdb pre-sized: 8 × 256 MB data + 512 MB log (were 8 MB)
* Windows Defender exclusions for `.mdf/.ldf/.ndf/.bak/.trn`, the data
  directory, `sqlservr.exe` and the app executable
* Windows power plan → **High Performance**

---

## 5. Security note

No credentials are stored in this directory. `run_maintenance.ps1` takes them
from `-UseWindowsAuth`, environment variables, or explicit parameters.
Windows authentication is recommended for the scheduled task.
