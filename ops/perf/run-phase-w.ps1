param(
    [string]$AdminDsn = $env:ABUZAR_PERF_ADMIN_DATABASE_URL,
    [string]$DatabaseName = '',
    [switch]$FullVolume,
    [switch]$AllowSharedCluster,
    [switch]$KeepDatabase,
    [switch]$CleanupOnFailure,
    [int]$Iterations = 15,
    [int]$SeedBatchSize = 50000
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
if ($SeedBatchSize -lt 1000 -or $SeedBatchSize -gt 500000) {
    throw 'SeedBatchSize must be between 1,000 and 500,000.'
}

if ([string]::IsNullOrWhiteSpace($DatabaseName)) {
    $DatabaseName = 'abuzar_phasew_' + (Get-Date -Format 'yyyyMMddHHmmss')
}
if ($DatabaseName -notmatch '^abuzar_phasew_[A-Za-z0-9_]+$') {
    throw 'DatabaseName must use the disposable abuzar_phasew_ prefix.'
}
if ($FullVolume -and -not $AllowSharedCluster) {
    throw 'Full-volume runs require an isolated PostgreSQL cluster. Pass -AllowSharedCluster only for an explicitly isolated CI/service instance.'
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
            $script:phase = 'migration:' + $file.Name
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
$resuming = $false
$phase = 'create_database'
$failure = $null
$artifact = ''
try {
    Set-AdminConnectionEnvironment -Dsn $AdminDsn

    # Resumability: if the caller pointed -DatabaseName at a database that
    # already exists (e.g. one retained by a prior blocked run -- see
    # retainedForInspection in tmp/phase-w-blocker-*.json), skip creation and
    # go straight to migrations/seeding instead of failing on createdb. The
    # seed script itself tracks progress per batch in phase_w_seed_progress
    # and resumes from the last committed window. A resumed run is never
    # auto-dropped in the finally block below, regardless of -KeepDatabase,
    # since the caller explicitly pointed at a pre-existing database.
    $existsCheck = & $psql $AdminDsn -tAc "SELECT 1 FROM pg_database WHERE datname = '$DatabaseName'"
    if ($LASTEXITCODE -ne 0) { throw 'Unable to check whether the disposable database already exists.' }
    if (($existsCheck | Select-Object -First 1) -eq '1') {
        $resuming = $true
        Write-Output "Database $DatabaseName already exists; resuming into it instead of creating a new one."
    } else {
        & $createdb --maintenance-db postgres $DatabaseName
        if ($LASTEXITCODE -ne 0) { throw 'Unable to create the disposable database.' }
        $created = $true
    }

    $phase = 'migrations'
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
    $phase = 'seed:' + $stock.ToString() + '-stock/' + $gl.ToString() + '-gl'
    if ($FullVolume) {
        Write-Output ("Seeding full-volume dataset ({0} stock rows, {1} GL journals) in {2}-row batches." -f $stock, $gl, $SeedBatchSize)
        Write-Output 'This is batched and resumable: the seed script commits and logs progress after every batch (watch for NOTICE lines below).'
        Write-Output 'It can still take a long time at full volume; do not interrupt it unless the underlying Postgres instance is genuinely disposable/isolated.'
    }
    Invoke-Psql -Dsn $targetDsn -Arguments @(
        '--variable', "scale_stock=$stock",
        '--variable', "scale_gl=$gl",
        '--variable', "scale_items=$items",
        '--variable', "scale_batches=$batches",
        '--variable', "batch_size=$SeedBatchSize",
        '--file', $seed
    )

    $phase = 'performance_probes'
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
        resumed = $resuming
        seedBatchSize = $SeedBatchSize
        artifact = $artifact
        retained = [bool]$KeepDatabase
    } | ConvertTo-Json -Depth 4)
}
catch {
    $failure = $_.Exception.Message
    $blocker = Join-Path $root ('tmp\phase-w-blocker-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.json')
    $drive = Get-PSDrive -Name D -ErrorAction SilentlyContinue
    $memory = Get-CimInstance Win32_OperatingSystem -ErrorAction SilentlyContinue
    [pscustomobject]@{
        status = 'blocked'
        phase = $phase
        error = $failure
        database = $DatabaseName
        fullVolumeRequested = [bool]$FullVolume
        retainedForInspection = [bool](-not $CleanupOnFailure)
        diskFreeBytes = if ($drive) { [int64]$drive.Free } else { $null }
        freeMemoryBytes = if ($memory) { [int64]$memory.FreePhysicalMemory * 1KB } else { $null }
        generatedAt = [DateTime]::UtcNow.ToString('o')
        note = 'No acceptance claim is made. Inspect the retained disposable database only if it is explicitly cleaned up later.'
    } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $blocker -Encoding UTF8
    Write-Output "Phase W blocker artifact: $blocker"
    throw
}
finally {
    Remove-Item Env:ABUZAR_ADMIN_DATABASE_URL -ErrorAction SilentlyContinue
    Remove-Item Env:ABUZAR_PERF_DATABASE_URL -ErrorAction SilentlyContinue
    $retain = $KeepDatabase -or ($failure -and -not $CleanupOnFailure)
    if ($created -and -not $retain) {
        # Drop only the validated disposable name; no application database is
        # used. The harness has no external clients while this finally block
        # runs, so a plain drop is sufficient and avoids broad termination.
        Set-AdminConnectionEnvironment -Dsn $AdminDsn
        & $dropdb --if-exists --maintenance-db postgres $DatabaseName
        if ($LASTEXITCODE -ne 0) { throw 'Unable to drop the disposable database.' }
    } elseif ($created -and $failure) {
        Write-Output "Retaining failed disposable database for inspection: $DatabaseName"
    }
}
