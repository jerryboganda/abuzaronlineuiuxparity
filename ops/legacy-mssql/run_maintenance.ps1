<#
.SYNOPSIS
    Weekly SQL Server maintenance runner for the legacy PowerBuilder database
    (Task Scheduler entry point).

.DESCRIPTION
    Runs weekly_maintenance.sql: smart index rebuild/reorganize plus statistics
    refresh. Keeps query plans healthy so the legacy app does not degrade back
    into full-scan behaviour over time.

.NOTES
    Credentials are NEVER hard-coded. Supply them one of these ways:
      1. Windows auth (recommended):  -UseWindowsAuth
      2. Environment variables:       ABUZAR_SQL_USER / ABUZAR_SQL_PASSWORD
      3. Explicit parameters:         -SqlUser / -SqlPassword

.EXAMPLE
    .\run_maintenance.ps1 -UseWindowsAuth

.EXAMPLE
    $env:ABUZAR_SQL_USER='sa'; $env:ABUZAR_SQL_PASSWORD='***'
    .\run_maintenance.ps1
#>
param(
    [string] $ServerInstance = $(if ($env:ABUZAR_SQL_SERVER)   { $env:ABUZAR_SQL_SERVER }   else { '127.0.0.1' }),
    [string] $Database       = $(if ($env:ABUZAR_SQL_DATABASE) { $env:ABUZAR_SQL_DATABASE } else { 'FazalDinPP19DataBaseV2' }),
    [string] $SqlUser        = $env:ABUZAR_SQL_USER,
    [string] $SqlPassword    = $env:ABUZAR_SQL_PASSWORD,
    [switch] $UseWindowsAuth
)

$ErrorActionPreference = 'Stop'
$log = Join-Path $PSScriptRoot 'maintenance_log.txt'

"=== Run started $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss') ===" | Out-File $log -Append

$sqlArgs = @(
    '-S', $ServerInstance
    '-d', $Database
    '-i', (Join-Path $PSScriptRoot 'weekly_maintenance.sql')
    '-b'                # abort on error so $LASTEXITCODE is meaningful
    '-t', '7200'        # per-statement timeout (index rebuilds on HDD are slow)
)

if ($UseWindowsAuth) {
    $sqlArgs += '-E'
}
elseif ($SqlUser -and $SqlPassword) {
    $sqlArgs += @('-U', $SqlUser, '-P', $SqlPassword)
}
else {
    $msg = 'No credentials supplied. Use -UseWindowsAuth, or set ABUZAR_SQL_USER / ABUZAR_SQL_PASSWORD.'
    $msg | Out-File $log -Append
    Write-Error $msg
    exit 1
}

& sqlcmd @sqlArgs 2>&1 | Out-File $log -Append

"ExitCode: $LASTEXITCODE" | Out-File $log -Append
"=== Run finished $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss') ===" | Out-File $log -Append

# Keep the log bounded
if ((Get-Item $log).Length -gt 1MB) {
    Get-Content $log -Tail 500 | Set-Content $log
}
