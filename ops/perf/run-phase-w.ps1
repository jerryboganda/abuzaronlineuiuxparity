param(
    [string]$AdminDsn = $env:ABUZAR_PERF_ADMIN_DATABASE_URL,
    [string]$DatabaseName = '',
    [switch]$FullVolume,
    [switch]$KeepDatabase,
    [int]$Iterations = 15
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$psql = (Get-Command psql.exe -ErrorAction Stop).Source
$createdb = (Get-Command createdb.exe -ErrorAction Stop).Source
$dropdb = (Get-Command dropdb.exe -ErrorAction Stop).Source
$seed = Join-Path $PSScriptRoot 'seed-scale.sql'
$migrationRoot = Join-Path $root 'db\migrations'

if ([string]::IsNullOrWhiteSpace($AdminDsn)) {
    throw 'Provide -AdminDsn or ABUZAR_PERF_ADMIN_DATABASE_URL. Do not put credentials in this repository.'
}
if ($Iterations -lt 3 -or $Iterations -gt 100) {
    throw 'Iterations must be between 3 and 100.'
}

if ([string]::IsNullOrWhiteSpace($DatabaseName)) {
    $DatabaseName = 'abuzar_phasew_' + (Get-Date -Format 'yyyyMMddHHmmss')
}
if ($DatabaseName -notmatch '^abuzar_phasew_[A-Za-z0-9_]+$') {
    throw 'DatabaseName must use the disposable abuzar_phasew_ prefix.'
}

function Invoke-Psql {
    param([string]$Dsn, [string[]]$Arguments)
    & $psql $Dsn --set ON_ERROR_STOP=1 @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "psql failed with exit code $LASTEXITCODE."
    }
}

function Target-Dsn {
    param([string]$Dsn, [string]$Name)
    $queryIndex = $Dsn.IndexOf('?')
    $suffix = ''
    $base = $Dsn
    if ($queryIndex -ge 0) {
        $suffix = $Dsn.Substring($queryIndex)
        $base = $Dsn.Substring(0, $queryIndex)
    }
    $slash = $base.LastIndexOf('/')
    if ($slash -lt 0) {
        throw 'AdminDsn must be a PostgreSQL URI containing a database path.'
    }
    return $base.Substring(0, $slash + 1) + $Name + $suffix
}

function Set-AdminConnectionEnvironment {
    param([string]$Dsn)
    $uri = [Uri]$Dsn
    $env:PGHOST = $uri.Host
    if ($uri.Port -gt 0) { $env:PGPORT = [string]$uri.Port }
    if ($uri.UserInfo) {
        $parts = $uri.UserInfo.Split(':', 2)
        $env:PGUSER = [Uri]::UnescapeDataString($parts[0])
        if ($parts.Count -gt 1) { $env:PGPASSWORD = [Uri]::UnescapeDataString($parts[1]) }
    }
    foreach ($pair in $uri.Query.TrimStart('?').Split('&')) {
        if ($pair -match '^sslmode=(.+)$') { $env:PGSSLMODE = $Matches[1] }
    }
}

function Apply-HarnessMigrations {
    param([string]$Dsn)
    $files = Get-ChildItem -LiteralPath $migrationRoot -Filter '*.sql' -File |
        Sort-Object Name
    foreach ($file in $files) {
        if ($file.Name -eq '020_historical_migration_wave.sql') {
            # This existing migration has an importer counter FK fixture.
            # Parent rows are created only inside this disposable database.
            Invoke-Psql -Dsn $Dsn -Arguments @(
                '--command', "INSERT INTO tenants (id, legal_name, code) VALUES ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'::uuid, 'Phase E fixture parent', 'phase-e-parent') ON CONFLICT DO NOTHING; INSERT INTO branches (id, tenant_id, code, name) VALUES ('ffffffff-ffff-ffff-ffff-ffffffffffff'::uuid, 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'::uuid, 'IMPORT', 'Phase E fixture branch') ON CONFLICT DO NOTHING;"
            )
        }
        Write-Output "Applying $($file.Name)"
        Invoke-Psql -Dsn $Dsn -Arguments @('--file', $file.FullName)
    }
}

$targetDsn = Target-Dsn -Dsn $AdminDsn -Name $DatabaseName
$created = $false
try {
    Set-AdminConnectionEnvironment -Dsn $AdminDsn
    & $createdb --maintenance-db postgres $DatabaseName
    if ($LASTEXITCODE -ne 0) { throw 'Unable to create the disposable database.' }
    $created = $true

    Apply-HarnessMigrations -Dsn $targetDsn

    $stock = 25000
    $gl = 10000
    $items = 3000
    $batches = 5000
    if ($FullVolume) {
        $stock = 3231846
        $gl = 1040590
        $items = 30050
        $batches = 30050
    }
    Invoke-Psql -Dsn $targetDsn -Arguments @(
        '--variable', "scale_stock=$stock",
        '--variable', "scale_gl=$gl",
        '--variable', "scale_items=$items",
        '--variable', "scale_batches=$batches",
        '--file', $seed
    )

    $artifact = Join-Path $root ('tmp\phase-w-performance-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.json')
    $env:ABUZAR_PERF_DATABASE_URL = $targetDsn
    & go run ./services/api/cmd/perf -iterations $Iterations -output $artifact
    if ($LASTEXITCODE -ne 0) {
        throw 'Performance probes failed.'
    }
    Write-Output ([pscustomobject]@{
        status = 'ok'
        database = $DatabaseName
        fullVolumeRequested = [bool]$FullVolume
        artifact = $artifact
        retained = [bool]$KeepDatabase
    } | ConvertTo-Json -Depth 4)
}
finally {
    Remove-Item Env:ABUZAR_ADMIN_DATABASE_URL -ErrorAction SilentlyContinue
    Remove-Item Env:ABUZAR_PERF_DATABASE_URL -ErrorAction SilentlyContinue
    if ($created -and -not $KeepDatabase) {
        # Drop only the validated disposable name; no application database is
        # used. The harness has no external clients while this finally block
        # runs, so a plain drop is sufficient and avoids broad termination.
        Set-AdminConnectionEnvironment -Dsn $AdminDsn
        & $dropdb --if-exists --maintenance-db postgres $DatabaseName
        if ($LASTEXITCODE -ne 0) { throw 'Unable to drop the disposable database.' }
    }
}
