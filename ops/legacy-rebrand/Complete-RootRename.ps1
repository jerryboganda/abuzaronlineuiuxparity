<#
.SYNOPSIS
    Final step of the Bismillah Pharmacy Software rebrand:
    rename the root folder  D:\ABUZAR  ->  D:\BISMILLAH

.DESCRIPTION
    The application binaries, database objects, launcher scripts, desktop
    shortcut and scheduled task have already been rebranded. The only remaining
    trace of the old name is the root folder itself.

    That folder cannot simply be renamed, because it also contains the whole
    SQL Server 2022 instance (MSSQL16.MSSQLSERVER): the engine binaries in
    Binn, the Resource database, master/model/msdb/tempdb and all production
    data files.

    Renaming it therefore means relocating the instance. This script does all
    four things that are required, in the right order:

      A. The three SQL service ImagePath values (Service Control Manager) -
         without this SQL Server can never start again.
      B. Every path stored under the instance registry hive
         (-d/-e/-l startup parameters, ErrorDumpDir, SQLAGENT.OUT,
          WorkingDirectory, BackupDirectory, SQLPath, SQLBinRoot, ...).
      C. The location of master, via the -d and -l startup parameters.
      D. The catalogued location of every other database, via
         ALTER DATABASE ... MODIFY FILE, executed while SQL Server runs in
         single-user, master-only-recovery mode (/f /T3608) so that the still
         stale model/msdb/tempdb paths cannot block startup.

    Safety features:
      * Nothing is changed until a complete snapshot has been taken and every
        captured path has been verified to exist on disk.
      * repoint.sql AND rollback.sql are both generated before any mutation.
      * Any failure is fatal - the script never continues past a failed step.
      * A full recovery procedure is printed if anything goes wrong.

.IMPORTANT
    Microsoft documents how to move system *databases*; it does not document
    relocating an installed instance's program directory. This script performs
    a same-volume folder rename, which keeps every file's position relative to
    the instance root and works in practice, but be aware:

      * Windows Installer still records the original path, so a future
        cumulative update or a repair/uninstall of SQL Server may complain.
      * The fully supported alternative is: back up all databases, uninstall
        SQL Server, rename the folder, reinstall the instance to the new path,
        restore the databases.

    Take a full backup of every database before running this.

.EXAMPLE
    # Preview only - changes nothing:
    & 'D:\ABUZAR\Complete-RootRename.ps1' -WhatIfOnly

.EXAMPLE
    # Perform the rename (ELEVATED prompt, maintenance window):
    Start-Process powershell -Verb RunAs
    & 'D:\ABUZAR\Complete-RootRename.ps1'
#>

[CmdletBinding()]
param(
    [string]$OldRoot   = 'D:\ABUZAR',
    [string]$NewRoot   = 'D:\BISMILLAH',
    [string]$Instance  = 'MSSQL16.MSSQLSERVER',
    [string]$Service   = 'MSSQLSERVER',
    [string[]]$SqlServices = @('MSSQLSERVER','SQLSERVERAGENT','SQLTELEMETRY'),
    [string]$SqlServer = '127.0.0.1',
    [string]$SqlUser   = 'sa',
    [string]$SqlPassword,
    [switch]$WhatIfOnly
)

$ErrorActionPreference = 'Stop'
$LogDir = Join-Path $env:TEMP ("BismillahRootRename_" + (Get-Date -Format 'yyyyMMdd_HHmmss'))
New-Item -ItemType Directory -Path $LogDir -Force | Out-Null

function Say  ($m) { Write-Host "[*] $m" -ForegroundColor Cyan }
function Ok   ($m) { Write-Host "[+] $m" -ForegroundColor Green }
function Warn ($m) { Write-Host "[!] $m" -ForegroundColor Yellow }
function Die  ($m) { Write-Host "[X] $m" -ForegroundColor Red; throw $m }

Write-Host ''
Write-Host '=========================================================' -ForegroundColor White
Write-Host '  Bismillah Pharmacy Software - Root Folder Rename'        -ForegroundColor White
Write-Host "  $OldRoot  ->  $NewRoot"                                  -ForegroundColor White
Write-Host '=========================================================' -ForegroundColor White
Write-Host "  Log / rollback directory: $LogDir"
Write-Host ''

# ============================================================== 0. PREFLIGHT ==
$idn = [Security.Principal.WindowsIdentity]::GetCurrent()
$pri = New-Object Security.Principal.WindowsPrincipal($idn)
if (-not $pri.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Die 'Run this from an ELEVATED PowerShell prompt (right-click PowerShell -> Run as administrator).'
}
Ok 'Running elevated.'

if ($OldRoot -match "[`'`[`]%_]") { Die "OldRoot contains a character that is unsafe in T-SQL/LIKE: $OldRoot" }
if (-not (Test-Path -LiteralPath $OldRoot)) { Die "Source root '$OldRoot' does not exist - nothing to do." }
if (Test-Path -LiteralPath $NewRoot)        { Die "Target root '$NewRoot' already exists - remove or rename it first." }
if ((Split-Path $OldRoot -Qualifier) -ne (Split-Path $NewRoot -Qualifier)) {
    Die "OldRoot and NewRoot must be on the same volume (this is a rename, not a copy)."
}

$svc = Get-Service -Name $Service -ErrorAction SilentlyContinue
if (-not $svc) { Die "SQL Server service '$Service' not found (use -Service to override)." }
$svcWasRunningAtStart = ($svc.Status -eq 'Running')
Ok "SQL Server service '$Service' found (currently $($svc.Status))."

$regRoot  = "HKLM:\SOFTWARE\Microsoft\Microsoft SQL Server\$Instance"
$paramKey = Join-Path $regRoot 'MSSQLServer\Parameters'
if (-not (Test-Path $regRoot))  { Die "Instance registry hive not found: $regRoot (use -Instance to override)." }
if (-not (Test-Path $paramKey)) { Die "Startup parameter key not found: $paramKey" }
Ok 'Instance registry hive found.'

$sqlcmd = @(
    'C:\Program Files\Microsoft SQL Server\Client SDK\ODBC\180\Tools\Binn\SQLCMD.EXE',
    'C:\Program Files\Microsoft SQL Server\Client SDK\ODBC\170\Tools\Binn\SQLCMD.EXE',
    'C:\Program Files\Microsoft SQL Server\Client SDK\ODBC\160\Tools\Binn\SQLCMD.EXE'
) | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $sqlcmd) {
    $c = Get-Command sqlcmd.exe -ErrorAction SilentlyContinue
    if ($c) { $sqlcmd = $c.Source }
}
if (-not $sqlcmd) { Die 'sqlcmd.exe not found - install the SQL Server command line utilities.' }
Ok "Using sqlcmd: $sqlcmd"

# --- credentials (never stored in this file) ---------------------------------
if (-not $SqlPassword) {
    $ini = Join-Path $OldRoot 'Preferences.ini'
    if (Test-Path -LiteralPath $ini) {
        $mm = [regex]::Match((Get-Content -LiteralPath $ini -Raw), '(?im)^\s*LogPassword\s*=\s*(.+?)\s*$')
        if ($mm.Success) { $SqlPassword = $mm.Groups[1].Value }
    }
}
if (-not $SqlPassword) {
    $sec = Read-Host "SQL password for login '$SqlUser'" -AsSecureString
    $SqlPassword = [Runtime.InteropServices.Marshal]::PtrToStringAuto(
                       [Runtime.InteropServices.Marshal]::SecureStringToBSTR($sec))
}
if (-not $SqlPassword) { Die "No SQL password available for login '$SqlUser'." }

# -w 65535 stops sqlcmd wrapping long file paths at its 80-column default.
function Sql {
    param([string]$Query, [string]$Sep = '|')
    $o = & $sqlcmd -S $SqlServer -U $SqlUser -P $SqlPassword -d master -b -h -1 -W -w 65535 -s $Sep -l 30 -Q $Query 2>&1
    [pscustomobject]@{ Output = @($o); ExitCode = $LASTEXITCODE }
}
function SqlFile {
    param([string]$Path)
    $o = & $sqlcmd -S $SqlServer -U $SqlUser -P $SqlPassword -d master -b -w 65535 -l 240 -i $Path 2>&1
    [pscustomobject]@{ Output = @($o); ExitCode = $LASTEXITCODE }
}
function Test-SqlUp {
    param([int]$TimeoutSec = 180)
    $t0 = Get-Date
    while (((Get-Date) - $t0).TotalSeconds -lt $TimeoutSec) {
        $r = Sql 'SELECT 1;'
        if ($r.ExitCode -eq 0) { return $true }
        Start-Sleep -Seconds 3
    }
    return $false
}

# ================================================ 1. SNAPSHOT (for rollback) ==
if ($svc.Status -ne 'Running') {
    Say 'Service is stopped - starting it so the current layout can be captured...'
    Start-Service -Name $Service
    (Get-Service $Service).WaitForStatus('Running','00:03:00')
}
if (-not (Test-SqlUp)) { Die "Cannot connect to $SqlServer as '$SqlUser'. Check the password / instance." }
Ok "Connected to $SqlServer as '$SqlUser'."

Say 'Capturing database file layout...'
$r = Sql "SET NOCOUNT ON; SELECT DB_NAME(database_id), name, type_desc, physical_name FROM sys.master_files ORDER BY database_id, file_id;"
if ($r.ExitCode -ne 0) { Die "Could not read sys.master_files: $($r.Output -join "`n")" }

$snapshot = @()
foreach ($line in $r.Output) {
    $s = "$line".Trim()
    if ($s -notmatch '\|') { continue }
    $p = $s -split '\|'
    if ($p.Count -lt 4) { continue }
    $snapshot += [pscustomobject]@{
        Database     = $p[0].Trim()
        LogicalName  = $p[1].Trim()
        Type         = $p[2].Trim()
        PhysicalName = ($p[3..($p.Count-1)] -join '|').Trim()   # path can never contain '|', but be safe
    }
}
if ($snapshot.Count -eq 0) { Die 'Parsed zero rows from sys.master_files - aborting.' }

# Every captured path MUST exist; a truncated/wrapped path would generate a
# valid-looking ALTER DATABASE pointing at a file that does not exist.
$missing = @($snapshot | Where-Object { -not (Test-Path -LiteralPath $_.PhysicalName) })
if ($missing.Count) {
    $missing | ForEach-Object { Warn "  missing: $($_.Database) / $($_.LogicalName) -> $($_.PhysicalName)" }
    Die 'Snapshot contains paths that do not exist on disk (probably truncated). Aborting before any change.'
}
$snapshot | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $LogDir 'db-file-snapshot.json') -Encoding UTF8
$affected = @($snapshot | Where-Object { $_.PhysicalName -like "$OldRoot\*" })
Ok "$($snapshot.Count) database files captured and verified; $($affected.Count) live under $OldRoot."

# ---- service ImagePath values (Service Control Manager) ---------------------
Say 'Capturing SQL service executable paths...'
$svcEdits = @()
foreach ($n in $SqlServices) {
    $k = "HKLM:\SYSTEM\CurrentControlSet\Services\$n"
    if (-not (Test-Path $k)) { continue }
    $ip = (Get-ItemProperty -Path $k -Name ImagePath -ErrorAction SilentlyContinue).ImagePath
    if ($ip -and $ip -like "*$OldRoot*") {
        $svcEdits += [pscustomobject]@{
            Name = $n; Key = $k; Old = $ip
            New  = [regex]::Replace($ip, [regex]::Escape($OldRoot), $NewRoot.Replace('$','$$'),
                                    [Text.RegularExpressions.RegexOptions]::IgnoreCase)
        }
        Write-Host "      $n"
        Write-Host "        $ip"
    }
}
$svcEdits | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $LogDir 'service-imagepaths.json') -Encoding UTF8
Ok "$($svcEdits.Count) service ImagePath value(s) reference $OldRoot."
if ($svcEdits.Count -eq 0) { Warn 'No service ImagePath references the old root - verify -SqlServices is correct.' }

# ---- every registry value under the instance hive (recursive) ---------------
Say 'Capturing instance registry paths (recursive)...'
$regEdits = @()
$hives = @($regRoot, 'HKLM:\SOFTWARE\Microsoft\MSSQLServer')
foreach ($hive in $hives) {
    if (-not (Test-Path $hive)) { continue }
    $keys = @($hive) + @(Get-ChildItem -Path $hive -Recurse -ErrorAction SilentlyContinue | ForEach-Object { $_.PSPath })
    foreach ($kp in $keys) {
        $props = Get-ItemProperty -Path $kp -ErrorAction SilentlyContinue
        if (-not $props) { continue }
        foreach ($pr in $props.PSObject.Properties) {
            if ($pr.Name -match '^PS(Path|ParentPath|ChildName|Drive|Provider)$') { continue }
            $v = $pr.Value
            if ($v -is [string] -and $v -like "*$OldRoot*") {
                $regEdits += [pscustomobject]@{
                    Key = "$kp"; Name = $pr.Name; Old = $v
                    New = [regex]::Replace($v, [regex]::Escape($OldRoot), $NewRoot.Replace('$','$$'),
                                           [Text.RegularExpressions.RegexOptions]::IgnoreCase)
                }
            }
            elseif ($v -is [string[]]) {
                if (@($v | Where-Object { $_ -like "*$OldRoot*" }).Count) {
                    $regEdits += [pscustomobject]@{
                        Key = "$kp"; Name = $pr.Name; Old = ($v -join "`n"); IsMulti = $true
                        New = (($v | ForEach-Object {
                            [regex]::Replace($_, [regex]::Escape($OldRoot), $NewRoot.Replace('$','$$'),
                                             [Text.RegularExpressions.RegexOptions]::IgnoreCase) }) -join "`n")
                    }
                }
            }
        }
    }
}
$regEdits | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $LogDir 'registry-edits.json') -Encoding UTF8
Ok "$($regEdits.Count) registry values reference $OldRoot."
foreach ($e in $regEdits) { Write-Host ("      {0}\{1}" -f (Split-Path $e.Key -Leaf), $e.Name) }

# verify we found exactly one -d, one -e and one -l startup parameter
$startArgs = @($regEdits | Where-Object { $_.Key -like "*MSSQLServer\Parameters" })
foreach ($flag in @('-d','-e','-l')) {
    $n = @($startArgs | Where-Object { $_.Old -like "$flag*" }).Count
    if ($n -ne 1) { Die "Expected exactly one '$flag' startup parameter referencing $OldRoot, found $n. Inspect $paramKey manually." }
}
Ok 'Startup parameters -d, -e and -l all located.'

# ---------- build repoint.sql and rollback.sql BEFORE touching anything ------
$repointSql  = Join-Path $LogDir 'repoint.sql'
$rollbackSql = Join-Path $LogDir 'rollback.sql'
$fwd = @('SET NOCOUNT ON;'); $bak = @('SET NOCOUNT ON;'); $nAlter = 0
foreach ($f in $affected) {
    if ($f.Database -eq 'master') { continue }   # master moves via -d / -l
    $db = $f.Database.Replace(']', ']]')
    $ln = $f.LogicalName.Replace("'", "''")
    $oldFp = $f.PhysicalName.Replace("'", "''")
    $newFp = $f.PhysicalName.Replace($OldRoot, $NewRoot).Replace("'", "''")
    $fwd += "ALTER DATABASE [$db] MODIFY FILE (NAME = N'$ln', FILENAME = N'$newFp');"
    $bak += "ALTER DATABASE [$db] MODIFY FILE (NAME = N'$ln', FILENAME = N'$oldFp');"
    $nAlter++
}
$oldPrefix = ($OldRoot.TrimEnd('\') + '\')
$fwd += "IF EXISTS (SELECT 1 FROM sys.master_files WHERE LEFT(physical_name, LEN(N'$oldPrefix')) = N'$oldPrefix')"
$fwd += "    THROW 50001, 'Old-root database references still remain after repointing.', 1;"
$fwd += "PRINT 'REPOINT OK';"
$fwd -join "`r`n" | Set-Content -LiteralPath $repointSql  -Encoding Unicode
$bak -join "`r`n" | Set-Content -LiteralPath $rollbackSql -Encoding Unicode
Ok "$nAlter ALTER statements written to repoint.sql (rollback.sql also generated)."

if ($WhatIfOnly) {
    if (-not $svcWasRunningAtStart) {
        Say 'Restoring service to its original stopped state...'
        Stop-Service -Name $Service -Force -ErrorAction SilentlyContinue
    }
    Write-Host ''
    Warn 'WhatIfOnly - no changes made.'
    Write-Host "  Would rename   : $OldRoot -> $NewRoot"
    Write-Host "  Would reconfig : $($svcEdits.Count) service ImagePath value(s)"
    Write-Host "  Would rewrite  : $($regEdits.Count) registry values"
    Write-Host "  Would repoint  : $nAlter database files"
    Write-Host "  Scripts staged : $repointSql"
    Write-Host "                   $rollbackSql"
    Write-Host "  Snapshots in   : $LogDir"
    return
}

Write-Host ''
Warn 'SQL Server will be stopped and the root folder renamed.'
Warn 'Confirm the pharmacy application is CLOSED on every workstation,'
Warn 'and that you have a current backup of every database.'
if ((Read-Host 'Type YES to proceed') -ne 'YES') { Warn 'Aborted - no changes made.'; return }

# ====================================================== 2. STOP SQL SERVICES ==
Say 'Stopping SQL Server services...'
$wasRunning = @()
foreach ($n in @('SQLSERVERAGENT', $Service, 'SQLBrowser', 'SQLTELEMETRY')) {
    $s = Get-Service -Name $n -ErrorAction SilentlyContinue
    if ($s -and $s.Status -ne 'Stopped') {
        Stop-Service -Name $n -Force -ErrorAction SilentlyContinue
        $wasRunning += $n
        Ok "  stopped $n"
    }
}
Start-Sleep -Seconds 5
if ((Get-Service $Service).Status -ne 'Stopped') { Die "Could not stop '$Service' - nothing has been changed." }
Ok 'SQL Server stopped.'

# ========================================================== 3. RENAME FOLDER ==
Say "Renaming $OldRoot -> $NewRoot ..."
try {
    Rename-Item -LiteralPath $OldRoot -NewName (Split-Path $NewRoot -Leaf) -ErrorAction Stop
    Ok "Renamed to $NewRoot"
} catch {
    Warn "Rename failed: $($_.Exception.Message)"
    Warn 'A process still holds a handle on the folder. Restarting SQL Server and aborting.'
    foreach ($n in $wasRunning) { Start-Service -Name $n -ErrorAction SilentlyContinue }
    Die 'Root rename failed - system restored to its previous state.'
}

$RecoveryText = @"
RECOVERY PROCEDURE
  Everything needed is in: $LogDir

  1. If any ALTER DATABASE already succeeded, start SQL in recovery mode and
     undo them first:
         net start $Service /f /T3608
         & '$sqlcmd' -S $SqlServer -U $SqlUser -P <password> -d master -b -i '$rollbackSql'
         Stop-Service $Service -Force
  2. Restore the service executable paths (service-imagepaths.json):
         sc.exe config <Name> binPath= "<Old>"
  3. Restore the registry values (registry-edits.json):
         Set-ItemProperty -Path <Key> -Name <Name> -Value <Old>
  4. Rename the folder back:
         Rename-Item -LiteralPath '$NewRoot' -NewName '$(Split-Path $OldRoot -Leaf)'
  5. Start-Service $Service
"@

# ============================== 4. SERVICE ImagePath (Service Control Manager)
Say 'Reconfiguring SQL service executable paths...'
foreach ($e in $svcEdits) {
    $o  = & "$env:SystemRoot\System32\sc.exe" config $e.Name 'binPath=' $e.New 2>&1
    $rc = $LASTEXITCODE
    if ($rc -ne 0) { Warn ($o -join "`n"); Write-Host $RecoveryText; Die "sc.exe config failed for $($e.Name)." }
    $actual = (Get-ItemProperty -Path $e.Key -Name ImagePath).ImagePath
    if ($actual -ne $e.New) { Write-Host $RecoveryText; Die "ImagePath verification failed for $($e.Name): '$actual'" }
    Ok "  $($e.Name)"
    Write-Host "      -> $($e.New)"
}

# ==================================================== 5. REWRITE REGISTRY =====
Say 'Rewriting instance registry paths...'
foreach ($e in $regEdits) {
    try {
        if ($e.PSObject.Properties.Name -contains 'IsMulti' -and $e.IsMulti) {
            Set-ItemProperty -Path $e.Key -Name $e.Name -Value ($e.New -split "`n") -ErrorAction Stop
        } else {
            Set-ItemProperty -Path $e.Key -Name $e.Name -Value $e.New -ErrorAction Stop
        }
        $chk = (Get-ItemProperty -Path $e.Key -Name $e.Name).$($e.Name)
        if (($chk -join "`n") -ne $e.New) { throw "verification mismatch" }
        Ok "  $(Split-Path $e.Key -Leaf)\$($e.Name)"
    } catch {
        Write-Host $RecoveryText
        Die "Could not update $($e.Key)\$($e.Name): $($_.Exception.Message)"
    }
}

# ================== 6. START MASTER-ONLY, SINGLE-USER, RESERVED FOR SQLCMD ====
function Start-Recovery {
    param([switch]$ReserveForSqlcmd)
    $argsList = @('start', $Service, '/f', '/T3608')
    if ($ReserveForSqlcmd) { $argsList += '/mSQLCMD' }
    $o = & "$env:SystemRoot\System32\net.exe" @argsList 2>&1
    $rc = $LASTEXITCODE
    $o | ForEach-Object { if ("$_".Trim()) { Write-Host "      $_" } }
    return ($rc -eq 0 -and (Get-Service $Service).Status -eq 'Running')
}

Say 'Starting SQL Server in single-user, master-only recovery mode...'
if (-not (Start-Recovery -ReserveForSqlcmd)) {
    Write-Host $RecoveryText
    Die 'SQL Server would not start in recovery mode.'
}
Ok 'SQL Server started (/f /T3608 /mSQLCMD).'
Start-Sleep -Seconds 8

# ======================================== 7. REPOINT model/msdb/tempdb/user ===
# Single-user mode allows ONE connection, so this is a single sqlcmd run.
Say "Applying repoint.sql ($nAlter statements, one connection)..."
$res = SqlFile -Path $repointSql
$res.Output | ForEach-Object { if ("$_".Trim()) { Write-Host "      $_" } }

if ($res.ExitCode -ne 0) {
    Warn 'Repoint failed on the reserved connection - retrying without /mSQLCMD...'
    Stop-Service -Name $Service -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 6
    if (-not (Start-Recovery)) { Write-Host $RecoveryText; Die 'SQL Server would not restart in recovery mode.' }
    Start-Sleep -Seconds 8
    $res = SqlFile -Path $repointSql
    $res.Output | ForEach-Object { if ("$_".Trim()) { Write-Host "      $_" } }
}
if ($res.ExitCode -ne 0) {
    Write-Host $RecoveryText
    Die 'ALTER DATABASE repointing failed - NOT starting SQL Server normally.'
}
Ok 'All database file paths repointed and verified inside SQL Server.'

# ============================================== 8. RESTART IN NORMAL MODE =====
Say 'Stopping SQL Server (leaving recovery mode)...'
Stop-Service -Name $Service -Force
Start-Sleep -Seconds 8

Say 'Starting SQL Server normally...'
$o = & "$env:SystemRoot\System32\net.exe" start $Service 2>&1
$o | ForEach-Object { if ("$_".Trim()) { Write-Host "      $_" } }
if ((Get-Service $Service).Status -ne 'Running') {
    Write-Host $RecoveryText
    Die 'SQL Server did not start in normal mode.'
}
if (-not (Test-SqlUp -TimeoutSec 300)) { Write-Host $RecoveryText; Die 'SQL Server is running but not accepting connections.' }
Ok 'SQL Server running normally.'

foreach ($n in $wasRunning) {
    if ($n -eq $Service) { continue }
    try { Start-Service -Name $n -ErrorAction Stop; Ok "  started $n" }
    catch { Warn "  could not start $n - check its log path: $($_.Exception.Message)" }
}

# ================================================================= 9. VERIFY ==
Say 'Verifying database states...'
$bad = @()
$r = Sql "SET NOCOUNT ON; SELECT name, state_desc FROM sys.databases ORDER BY name;"
foreach ($line in $r.Output) {
    $s = "$line".Trim()
    if ($s -notmatch '\|') { continue }
    $p = $s -split '\|'
    Write-Host ("      {0,-72} {1}" -f $p[0].Trim(), $p[1].Trim())
    if ($p[1].Trim() -ne 'ONLINE') { $bad += $p[0].Trim() }
}

Say 'Checking for files still under the old root...'
$r2 = Sql "SET NOCOUNT ON; SELECT COUNT(*) FROM sys.master_files WHERE LEFT(physical_name, LEN(N'$oldPrefix')) = N'$oldPrefix';"
$leftN = ((($r2.Output -join '') -replace '[^\d]','') -as [int])
if ($null -eq $leftN) { $leftN = -1 }

Write-Host ''
Write-Host '=========================================================' -ForegroundColor White
if ($bad.Count -eq 0 -and $leftN -eq 0) {
    Write-Host '  SUCCESS' -ForegroundColor Green
    Write-Host "  $OldRoot  ->  $NewRoot" -ForegroundColor Green
    Write-Host '  Every database is ONLINE; no file references the old root.' -ForegroundColor Green
} else {
    Write-Host '  COMPLETED WITH WARNINGS' -ForegroundColor Yellow
    if ($bad.Count)   { Write-Host "  Not ONLINE: $($bad -join ', ')" -ForegroundColor Yellow }
    if ($leftN -ne 0) { Write-Host "  $leftN file(s) still reference $OldRoot" -ForegroundColor Yellow }
    Write-Host $RecoveryText -ForegroundColor Yellow
}
Write-Host '=========================================================' -ForegroundColor White
Write-Host ''
Write-Host 'Remaining manual steps:' -ForegroundColor Cyan
Write-Host "  1. Recreate the desktop shortcut:  & '$NewRoot\create_shortcut.ps1'"
Write-Host "  2. Launch $NewRoot\V2_BismillahSoftware\Application\BismillahPharmacy.exe"
Write-Host '     and confirm the title bar reads "BISMILLAH PHARMACY SOFTWARE ...".'
Write-Host "  3. Re-check the scheduled task 'Bismillah-DB-Weekly-Maintenance'."
Write-Host "  4. Take a fresh full backup of every database."
Write-Host ''
Write-Host "Log / rollback directory retained: $LogDir"
