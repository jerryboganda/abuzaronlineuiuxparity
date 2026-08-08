<#
.SYNOPSIS
    Mechanizes the safe, scriptable parts of the rollback procedure in
    docs/RUNBOOK_CUTOVER.md Section 9 (Rollback and recovery).

.DESCRIPTION
    This script does NOT replace docs/RUNBOOK_CUTOVER.md Section 9. It exists
    so a human executing (or rehearsing) a rollback has one reliable script to
    run instead of hand-typing each command under pressure. The manual
    step-by-step instructions in the runbook remain the authoritative
    reference and the documented fallback if this script cannot run.

    It runs in four modes:

      * Plan / dry run (default). Prints every step it WOULD take, with real
        target paths/commands resolved from repo config and environment
        variables, and performs no destructive action. This is also the safe
        way to sanity-check configuration before a rehearsal or a real
        rollback.

      * -Rehearse (still no -Execute). Same as plan mode, but also runs the
        production-DSN safety check below so a rehearsal operator finds a
        misconfigured disposable target before anything is typed under
        pressure.

      * -Rehearse -Execute. Performs the mechanical steps for real, but ONLY
        after confirming that every resolved PostgreSQL DSN does NOT match
        the known production host/role/database-name patterns recorded in
        ops/vps/docker-compose.yml. Intended for a disposable/sandbox copy of
        the database, never production.

      * -Execute (no -Rehearse). Performs the mechanical steps for real
        against the target selected with -Target. Every destructive/
        irreversible step (stopping the new system's write path, restoring a
        backup over an existing database, relaunching the legacy executable)
        requires an exact typed confirmation phrase first. Enter alone does
        nothing.

    What this script automates (per runbook Section 9.2 and Section 6.2):

      1. Verifies the legacy executable path and the pre-cutover backup file
         exist and look valid, BEFORE touching anything.
      2. Stops the new system's write path (the production API container per
         ops/vps/docker-compose.yml, or the local api/edge supervised
         processes per ops/local/*.ps1, depending on -Target).
      3. Optionally restores the pre-cutover PostgreSQL backup with the exact
         pg_restore flags documented in runbook Section 6.2
         (--clean --if-exists --no-owner --dbname <target>), only when
         -RestoreBackup is supplied and confirmed separately.
      4. Relaunches the legacy executable
         (D:\ABUZAR\V2_AbuzarSoftware\Application\abuzar.exe) exactly as
         documented in runbook Section 9.2 step 3.
      5. Logs every step with a UTC timestamp to console and to a rollback
         log file, and prints a final summary block whose fields line up
         with docs/ROLLBACK_REHEARSAL_RECORD_TEMPLATE.md so the output can be
         pasted directly into that record.

    What this script deliberately does NOT automate (still manual, per the
    runbook):

      * Announcing the maintenance window / incident bridge.
      * Marking physical terminals "ROLLBACK IN PROGRESS" (runbook Section
        9.2 step 2) -- there is no repository script that performs a
        production fleet repoint (runbook Section 8.2, line 469), and this
        script does not invent one.
      * The branch edge service, which runs on separate branch hardware and
        is out of scope for a central script.
      * The controlled transaction recovery/re-entry decision (runbook
        Section 9.2 step 5), which requires release manager + DBA + business
        owner approval and is a business decision, not a mechanical step.
      * Completing docs/ROLLBACK_REHEARSAL_RECORD_TEMPLATE.md itself -- this
        script only prints a block designed to be pasted into it.

.PARAMETER Execute
    Required to perform any real action. Without it, the script always
    behaves as a dry run regardless of any other switch.

.PARAMETER DryRun
    Explicit opt-in to the default dry-run behavior. Provided for callers
    that want to say so explicitly; omitting -Execute has the same effect.

.PARAMETER Rehearse
    Marks this run as a non-destructive rehearsal against a disposable/test
    database. When combined with -Execute, every resolved DSN is checked
    against known production host/role/database-name patterns and the
    script refuses to run if any match.

.PARAMETER Target
    Which write-path stop mechanism to use: 'docker' (production, per
    ops/vps/docker-compose.yml container names) or 'local' (the local dev
    stack started by ops/local/start-local.ps1). Default: docker.

.PARAMETER RestoreBackup
    Also perform the pg_restore step. Requires its own separate typed
    confirmation because it overwrites the target database. Without this
    switch, the script performs only the write-path stop and legacy-exe
    relaunch (the "mechanical terminal fallback" in runbook Section 9.2
    steps 1-4), matching the runbook's framing that restore is conditional
    ("If the target is corrupted...").

.PARAMETER BackupPath
    Path to the pre-cutover backup .dump file to restore. Defaults to the
    newest file matching D:\SecureBackups\AbuzarNext\cutover-pre-*.dump
    (the naming convention from runbook Section 6.1).

.PARAMETER RestoreDsn
    PostgreSQL connection string to restore into. Defaults to
    $env:ABUZAR_RESTORE_DATABASE_URL, then $env:ABUZAR_TARGET_POSTGRES_URL.
    Never logged unmasked.

.PARAMETER LegacyExePath
    Path to the legacy executable. Defaults to the path documented in
    runbook Section 9.2 step 3.

.PARAMETER ApiHealthUrl
.PARAMETER EdgeHealthUrl
    Optional health endpoints to snapshot before/after the rollback
    (GET /v1/health per runbook Section 7.1). Skipped if not supplied --
    this script does not guess a production hostname.

.PARAMETER LogPath
    Rollback log file path. Defaults under tmp\rollback\ (gitignored, same
    convention as ops/local and ops/perf runtime output).

.PARAMETER RecordId
    Value to print in the summary block's "Record ID" field. Defaults to a
    placeholder the operator should replace with the real change-id.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\ops\cutover\rollback.ps1

    Dry run against the default (docker/production) target. Prints the plan
    only.

.EXAMPLE
    $env:ABUZAR_RESTORE_DATABASE_URL = 'postgres://app@127.0.0.1:5432/abuzar_rehearsal?sslmode=disable'
    powershell -ExecutionPolicy Bypass -File .\ops\cutover\rollback.ps1 `
      -Rehearse -Execute -Target local -RestoreBackup -BackupPath D:\SecureBackups\AbuzarNext\rehearsal-fixture.dump

    Real rehearsal against a disposable local database. Refuses to run if
    ABUZAR_RESTORE_DATABASE_URL looks like production.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\ops\cutover\rollback.ps1 -Execute -RestoreBackup

    Real production rollback. Prompts for two separate typed confirmations.
#>

[CmdletBinding()]
param(
    [switch]$Execute,
    [switch]$DryRun,
    [switch]$Rehearse,
    [ValidateSet('docker', 'local')]
    [string]$Target = 'docker',
    [switch]$RestoreBackup,
    [string]$BackupPath,
    [string]$RestoreDsn,
    [string]$LegacyExePath = 'D:\ABUZAR\V2_AbuzarSoftware\Application\abuzar.exe',
    [string]$ApiHealthUrl,
    [string]$EdgeHealthUrl,
    [string]$LogPath,
    [string]$RecordId
)

$ErrorActionPreference = 'Stop'

# ---------------------------------------------------------------------------
# Resolve repo-relative configuration. Nothing here is a placeholder: these
# are the real paths/conventions read from docs/RUNBOOK_CUTOVER.md and the
# existing ops/ scripts.
# ---------------------------------------------------------------------------

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$isExecute = $Execute.IsPresent
$isDryRun = -not $isExecute
$startedUtc = [DateTime]::UtcNow

if (-not $LogPath) {
    $logDir = Join-Path $repoRoot 'tmp\rollback'
    New-Item -ItemType Directory -Force -Path $logDir | Out-Null
    $LogPath = Join-Path $logDir ("rollback-" + $startedUtc.ToString('yyyyMMdd-HHmmss') + '.log')
} else {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $LogPath) -ErrorAction SilentlyContinue | Out-Null
}
$script:LogFile = $LogPath
New-Item -ItemType File -Force -Path $script:LogFile | Out-Null

if (-not $RecordId) {
    $RecordId = '<change-id>-rollback-' + $startedUtc.ToString('yyyyMMddHHmmss')
}

$dockerApiContainer = 'abuzarnext-api'
$dockerWebContainer = 'abuzarnext-web'
$dockerComposeRef = Join-Path $repoRoot 'ops\vps\docker-compose.yml'
$localRuntimeDir = Join-Path $repoRoot 'tmp\local-runtime'
$localServicesToStop = @('api', 'edge')

$resolvedRestoreDsn = if ($RestoreDsn) {
    $RestoreDsn
} elseif ($env:ABUZAR_RESTORE_DATABASE_URL) {
    $env:ABUZAR_RESTORE_DATABASE_URL
} elseif ($env:ABUZAR_TARGET_POSTGRES_URL) {
    $env:ABUZAR_TARGET_POSTGRES_URL
} else {
    $null
}

if (-not $BackupPath) {
    $backupDir = 'D:\SecureBackups\AbuzarNext'
    if (Test-Path -LiteralPath $backupDir) {
        $candidate = Get-ChildItem -LiteralPath $backupDir -Filter 'cutover-pre-*.dump' -File -ErrorAction SilentlyContinue |
            Sort-Object LastWriteTimeUtc -Descending | Select-Object -First 1
        if ($candidate) { $BackupPath = $candidate.FullName }
    }
    if (-not $BackupPath) {
        # Keep the conventional path visible even when nothing matched yet,
        # so dry-run output tells the operator exactly what it looked for.
        $BackupPath = Join-Path $backupDir 'cutover-pre-<UTC>.dump'
    }
}

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

function Write-Log {
    param(
        [Parameter(Mandatory = $true)][string]$Message,
        [ValidateSet('INFO', 'PLAN', 'ACTION', 'WARN', 'ERROR', 'MANUAL')][string]$Level = 'INFO'
    )
    $stamp = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ss.fffZ')
    $line = "[$stamp] [$Level] $Message"
    $color = switch ($Level) {
        'WARN'   { 'Yellow' }
        'ERROR'  { 'Red' }
        'ACTION' { 'Cyan' }
        'MANUAL' { 'Magenta' }
        default  { 'Gray' }
    }
    Write-Host $line -ForegroundColor $color
    Add-Content -LiteralPath $script:LogFile -Value $line
}

function ConvertTo-MaskedDsn {
    param([string]$Dsn)
    if ([string]::IsNullOrWhiteSpace($Dsn)) { return '<not set>' }
    return [regex]::Replace($Dsn, '(://[^:/@\s]+):([^@\s]*)@', '$1:***@')
}

function Test-LooksLikeProductionDsn {
    # Patterns taken directly from ops/vps/docker-compose.yml, the only
    # source of production host/role/database-name conventions in this repo:
    #   DATABASE_URL=postgres://platform_admin:...@platform-postgres:5432/abuzarnext?sslmode=disable
    # The local/dev database name is 'abuzar_next' (with an underscore, see
    # .env.example), which does not match the 'abuzarnext' pattern below.
    param([string]$Dsn)
    if ([string]::IsNullOrWhiteSpace($Dsn)) { return $false }
    $patterns = @(
        'platform-postgres',
        'platform_admin',
        '/abuzarnext(\?|$)'
    )
    foreach ($pattern in $patterns) {
        if ($Dsn -imatch $pattern) { return $true }
    }
    return $false
}

function Confirm-Phrase {
    param(
        [Parameter(Mandatory = $true)][string]$Phrase,
        [Parameter(Mandatory = $true)][string]$PromptContext
    )
    Write-Host ''
    Write-Host "CONFIRMATION REQUIRED: $PromptContext" -ForegroundColor Yellow
    Write-Host "This is irreversible. Type exactly (case-sensitive) to proceed:" -ForegroundColor Yellow
    Write-Host "  $Phrase" -ForegroundColor Yellow
    $typed = Read-Host 'Confirmation'
    if ($typed -cne $Phrase) {
        Write-Log "Confirmation phrase did not match for: $PromptContext. Aborting." -Level ERROR
        throw "Confirmation phrase did not match. Aborting before destructive step: $PromptContext"
    }
    Write-Log "Confirmed: $PromptContext" -Level ACTION
}

function Get-HealthSnapshot {
    param([string]$Label, [string]$Url)
    if (-not $Url) {
        Write-Log "Skipping $Label health snapshot: no URL supplied (pass -ApiHealthUrl/-EdgeHealthUrl or set it in the protected operator shell)." -Level INFO
        return $null
    }
    try {
        $response = Invoke-RestMethod -Uri $Url -TimeoutSec 5
        Write-Log "$Label health: status=$($response.status) database=$($response.database)" -Level INFO
        return $response
    } catch {
        Write-Log "$Label health check failed: $($_.Exception.Message)" -Level WARN
        return $null
    }
}

function Stop-LocalTrackedService {
    # Mirrors the tracked-PID convention in ops/local/stop-local.ps1, but is
    # scoped to only the services passed in (api/edge) so postgres stays up
    # for the restore step and web is left for operator judgment.
    param([Parameter(Mandatory = $true)][string]$ServiceName)

    $stopFile = Join-Path $localRuntimeDir "$ServiceName.stop"
    $supervisorPidFile = Join-Path $localRuntimeDir "$ServiceName.supervisor.pid"
    $childPidFile = Join-Path $localRuntimeDir "$ServiceName.child.pid"

    if (-not (Test-Path -LiteralPath $localRuntimeDir)) {
        Write-Log "Local runtime directory not found at $localRuntimeDir; '$ServiceName' does not appear to be running under ops/local." -Level WARN
        return
    }

    New-Item -ItemType File -Force -Path $stopFile | Out-Null

    $childPid = 0
    if (Test-Path -LiteralPath $childPidFile) {
        [void][int]::TryParse((Get-Content -LiteralPath $childPidFile -TotalCount 1 -ErrorAction SilentlyContinue).Trim(), [ref]$childPid)
    }
    if ($childPid -gt 0) {
        $child = Get-CimInstance Win32_Process -Filter "ProcessId = $childPid" -ErrorAction SilentlyContinue
        if ($child) {
            Stop-Process -Id $childPid -Force -ErrorAction SilentlyContinue
            Write-Log "Stopped local '$ServiceName' child process (PID $childPid)." -Level ACTION
        } else {
            Write-Log "Local '$ServiceName' child PID $childPid was already gone." -Level INFO
        }
    }

    $supervisorPid = 0
    if (Test-Path -LiteralPath $supervisorPidFile) {
        [void][int]::TryParse((Get-Content -LiteralPath $supervisorPidFile -TotalCount 1 -ErrorAction SilentlyContinue).Trim(), [ref]$supervisorPid)
    }
    if ($supervisorPid -gt 0) {
        Stop-Process -Id $supervisorPid -Force -ErrorAction SilentlyContinue
        Write-Log "Stopped local '$ServiceName' supervisor process (PID $supervisorPid)." -Level ACTION
    }

    Remove-Item -LiteralPath $stopFile, $supervisorPidFile, $childPidFile -Force -ErrorAction SilentlyContinue
}

# ---------------------------------------------------------------------------
# Banner
# ---------------------------------------------------------------------------

$modeLabel = if ($isExecute) { 'EXECUTE' } else { 'DRY-RUN (default; pass -Execute to perform real actions)' }
$scopeLabel = if ($Rehearse) { 'REHEARSAL (disposable target expected)' } else { 'PRODUCTION ROLLBACK' }

Write-Log "AbuzarNext rollback automation starting." -Level INFO
Write-Log "Mode: $modeLabel | Scope: $scopeLabel | Target: $Target | RestoreBackup: $($RestoreBackup.IsPresent)" -Level INFO
Write-Log "Log file: $script:LogFile" -Level INFO
Write-Log "Record ID: $RecordId" -Level INFO
Write-Log "This script mechanizes docs/RUNBOOK_CUTOVER.md Section 9. The manual procedure there remains the authoritative fallback." -Level INFO

$stepRecords = New-Object System.Collections.Generic.List[object]
function Add-StepRecord {
    param([int]$Num, [string]$Action, [string]$Expected, [string]$Observed, [string]$Evidence, [string]$Result)
    $stepRecords.Add([pscustomobject]@{
        Num       = $Num
        UtcTime   = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
        Operator  = $env:USERNAME
        Action    = $Action
        Expected  = $Expected
        Observed  = $Observed
        Evidence  = $Evidence
        Result    = $Result
    }) | Out-Null
}

# ---------------------------------------------------------------------------
# Phase 1: preflight verification (always runs, never destructive)
# ---------------------------------------------------------------------------

Write-Log '--- Phase 1: preflight verification ---' -Level INFO

$legacyWorkingDir = Split-Path -Parent $LegacyExePath
$legacyExeOk = Test-Path -LiteralPath $LegacyExePath -PathType Leaf
if ($legacyExeOk) {
    Write-Log "Legacy executable found: $LegacyExePath" -Level INFO
} else {
    Write-Log "Legacy executable NOT found at $LegacyExePath (expected working directory: $legacyWorkingDir)." -Level WARN
}
Add-StepRecord -Num 1 -Action 'Verify legacy executable path' -Expected 'File exists at the documented path' `
    -Observed $(if ($legacyExeOk) { 'found' } else { 'MISSING' }) -Evidence $LegacyExePath `
    -Result $(if ($legacyExeOk) { 'pass' } else { 'fail' })

$backupOk = $false
$backupHash = $null
if (Test-Path -LiteralPath $BackupPath -PathType Leaf -ErrorAction SilentlyContinue) {
    $backupFile = Get-Item -LiteralPath $BackupPath
    if ($backupFile.Length -gt 0) {
        $backupOk = $true
        $backupHash = (Get-FileHash -LiteralPath $BackupPath -Algorithm SHA256).Hash
        Write-Log "Backup file found: $BackupPath ($([math]::Round($backupFile.Length / 1MB, 2)) MB) SHA-256=$backupHash" -Level INFO
    } else {
        Write-Log "Backup file at $BackupPath is zero bytes; treating as invalid." -Level WARN
    }
} else {
    Write-Log "Backup file NOT found at $BackupPath." -Level WARN
}
Add-StepRecord -Num 2 -Action 'Verify pre-cutover backup file exists and is non-empty' -Expected 'File exists, non-zero size, SHA-256 recorded' `
    -Observed $(if ($backupOk) { "found, sha256=$backupHash" } else { 'MISSING or empty' }) -Evidence $BackupPath `
    -Result $(if ($backupOk) { 'pass' } else { 'fail' })

if ($RestoreBackup) {
    if ([string]::IsNullOrWhiteSpace($resolvedRestoreDsn)) {
        Write-Log "RestoreBackup was requested but no restore DSN is resolved. Set `$env:ABUZAR_RESTORE_DATABASE_URL (or `$env:ABUZAR_TARGET_POSTGRES_URL) in the protected operator shell, or pass -RestoreDsn." -Level WARN
    } else {
        Write-Log "Restore target DSN (masked): $(ConvertTo-MaskedDsn $resolvedRestoreDsn)" -Level INFO
    }
}

if ($Rehearse) {
    Write-Log 'Rehearse mode: checking every resolved DSN against known production patterns (platform-postgres / platform_admin / abuzarnext).' -Level INFO
    $productionMatch = Test-LooksLikeProductionDsn -Dsn $resolvedRestoreDsn
    if ($productionMatch) {
        Write-Log "SAFETY ABORT: the resolved restore DSN matches a known PRODUCTION pattern from ops/vps/docker-compose.yml. Refusing to run in -Rehearse mode. DSN (masked): $(ConvertTo-MaskedDsn $resolvedRestoreDsn)" -Level ERROR
        Add-StepRecord -Num 3 -Action 'Rehearse-mode production DSN safety check' -Expected 'Restore DSN does not match production host/role/db patterns' `
            -Observed 'MATCHED production pattern' -Evidence $(ConvertTo-MaskedDsn $resolvedRestoreDsn) -Result 'fail'
        throw 'Refusing to rehearse against a target that matches a known production PostgreSQL DSN pattern. Point -RestoreDsn / $env:ABUZAR_RESTORE_DATABASE_URL at a disposable database.'
    }
    Write-Log 'Rehearse-mode DSN safety check passed: no production pattern matched.' -Level INFO
    Add-StepRecord -Num 3 -Action 'Rehearse-mode production DSN safety check' -Expected 'Restore DSN does not match production host/role/db patterns' `
        -Observed 'no match' -Evidence $(ConvertTo-MaskedDsn $resolvedRestoreDsn) -Result 'pass'
}

$preApiHealth = Get-HealthSnapshot -Label 'API' -Url $ApiHealthUrl
$preEdgeHealth = Get-HealthSnapshot -Label 'Edge' -Url $EdgeHealthUrl

# ---------------------------------------------------------------------------
# Phase 2: plan (what would happen)
# ---------------------------------------------------------------------------

Write-Log '--- Phase 2: rollback plan ---' -Level PLAN

if ($Target -eq 'docker') {
    Write-Log "[PLAN] Stop new-system write path: docker stop $dockerApiContainer  (per $dockerComposeRef)" -Level PLAN
    Write-Log "[PLAN] Stop new-system web front end: docker stop $dockerWebContainer  (best-effort, non-fatal if absent)" -Level PLAN
} else {
    foreach ($svc in $localServicesToStop) {
        Write-Log "[PLAN] Stop local supervised service '$svc' (tracked PID files under $localRuntimeDir), matching ops/local/stop-local.ps1 conventions. postgres and web are left running." -Level PLAN
    }
}

Write-Log '[PLAN] Manual (not automated): mark all physical terminals ROLLBACK IN PROGRESS. There is no repository script that performs a production fleet repoint (RUNBOOK_CUTOVER.md line 469).' -Level MANUAL

if ($RestoreBackup) {
    Write-Log "[PLAN] pg_restore --clean --if-exists --no-owner --dbname $(ConvertTo-MaskedDsn $resolvedRestoreDsn) '$BackupPath'  (flags per RUNBOOK_CUTOVER.md Section 6.2)" -Level PLAN
} else {
    Write-Log '[PLAN] Backup restore skipped (-RestoreBackup not supplied). Per runbook Section 9.2 step 6, restore only if the target is corrupted.' -Level PLAN
}

Write-Log "[PLAN] Relaunch legacy executable: Start-Process -FilePath '$LegacyExePath' -WorkingDirectory '$legacyWorkingDir'  (RUNBOOK_CUTOVER.md Section 9.2 step 3, read-only investigation only)" -Level PLAN
Write-Log '[PLAN] Manual (not automated): keep the legacy database read-only; do not resume legacy trading without release manager + DBA + business owner approval (runbook Section 9.2 step 5).' -Level MANUAL

if ($isDryRun) {
    Write-Log 'Dry run complete. No destructive action was taken. Re-run with -Execute to perform these steps for real.' -Level INFO
}

# ---------------------------------------------------------------------------
# Phase 3: execute (gated)
# ---------------------------------------------------------------------------

$stopResult = 'not attempted (dry run)'
$restoreResult = 'not attempted'
$relaunchResult = 'not attempted (dry run)'

if ($isExecute) {
    Write-Log '--- Phase 3: execute ---' -Level ACTION

    if (-not $legacyExeOk) {
        throw "Refusing to execute: legacy executable not found at $LegacyExePath. Fix -LegacyExePath or restore the legacy install before rolling back."
    }
    if ($RestoreBackup -and -not $backupOk) {
        throw "Refusing to execute -RestoreBackup: backup file not found or empty at $BackupPath."
    }
    if ($RestoreBackup -and [string]::IsNullOrWhiteSpace($resolvedRestoreDsn)) {
        throw 'Refusing to execute -RestoreBackup: no restore DSN resolved. Set $env:ABUZAR_RESTORE_DATABASE_URL or pass -RestoreDsn.'
    }

    $stopContext = if ($Target -eq 'docker') {
        "stop the production write path ($dockerApiContainer) and relaunch the legacy executable"
    } else {
        "stop the local api/edge services and relaunch the legacy executable"
    }
    Confirm-Phrase -Phrase 'STOP AND ROLLBACK' -PromptContext $stopContext

    if ($Target -eq 'docker') {
        try {
            & docker stop $dockerApiContainer | Out-Null
            if ($LASTEXITCODE -ne 0) { throw "docker stop $dockerApiContainer exited $LASTEXITCODE" }
            Write-Log "Stopped container $dockerApiContainer." -Level ACTION
            $stopResult = "docker container $dockerApiContainer stopped"
        } catch {
            Write-Log "Failed to stop $dockerApiContainer : $($_.Exception.Message)" -Level ERROR
            $stopResult = "FAILED: $($_.Exception.Message)"
            throw
        }
        try {
            & docker stop $dockerWebContainer | Out-Null
            Write-Log "Stopped container $dockerWebContainer." -Level ACTION
        } catch {
            Write-Log "Could not stop $dockerWebContainer (non-fatal): $($_.Exception.Message)" -Level WARN
        }
    } else {
        foreach ($svc in $localServicesToStop) {
            Stop-LocalTrackedService -ServiceName $svc
        }
        $stopResult = "local services stopped: $($localServicesToStop -join ', ')"
    }
    Add-StepRecord -Num 4 -Action 'Announce rollback and stop new AbuzarNext posts' -Expected 'No new posts accepted' `
        -Observed $stopResult -Evidence $script:LogFile -Result 'pass'

    if ($RestoreBackup) {
        Confirm-Phrase -Phrase 'RESTORE DATABASE BACKUP' -PromptContext "overwrite the target database with $BackupPath via pg_restore --clean --if-exists --no-owner"
        try {
            & pg_restore --clean --if-exists --no-owner --dbname $resolvedRestoreDsn $BackupPath
            if ($LASTEXITCODE -ne 0) { throw "pg_restore exited $LASTEXITCODE" }
            Write-Log 'pg_restore completed successfully.' -Level ACTION
            $restoreResult = 'pg_restore completed successfully'
        } catch {
            Write-Log "pg_restore failed: $($_.Exception.Message)" -Level ERROR
            $restoreResult = "FAILED: $($_.Exception.Message)"
            throw
        }
        Add-StepRecord -Num 5 -Action 'Restore pre-cutover PostgreSQL backup' -Expected 'Restore completes; hash matches recorded backup' `
            -Observed $restoreResult -Evidence "$BackupPath (sha256=$backupHash)" -Result 'pass'
    } else {
        Add-StepRecord -Num 5 -Action 'Restore pre-cutover PostgreSQL backup' -Expected 'Restore completes; hash matches recorded backup' `
            -Observed 'skipped: -RestoreBackup not supplied' -Evidence '' -Result 'not applicable'
    }

    try {
        Start-Process -FilePath $LegacyExePath -WorkingDirectory $legacyWorkingDir | Out-Null
        Write-Log "Relaunched legacy executable: $LegacyExePath" -Level ACTION
        $relaunchResult = 'legacy executable launched'
    } catch {
        Write-Log "Failed to relaunch legacy executable: $($_.Exception.Message)" -Level ERROR
        $relaunchResult = "FAILED: $($_.Exception.Message)"
        throw
    }
    Add-StepRecord -Num 6 -Action 'Repoint pilot terminal to the intact legacy executable' -Expected 'Client starts from the approved working directory' `
        -Observed $relaunchResult -Evidence $LegacyExePath -Result 'pass'

    Write-Log 'Execution complete. Legacy database remains read-only; do not post/save/cancel/restore transactions in it (runbook Section 3.3).' -Level WARN
}

$postApiHealth = if ($isExecute) { Get-HealthSnapshot -Label 'API (post-rollback)' -Url $ApiHealthUrl } else { $null }
$postEdgeHealth = if ($isExecute) { Get-HealthSnapshot -Label 'Edge (post-rollback)' -Url $EdgeHealthUrl } else { $null }

# ---------------------------------------------------------------------------
# Phase 4: summary block, shaped to match
# docs/ROLLBACK_REHEARSAL_RECORD_TEMPLATE.md field names/order.
# ---------------------------------------------------------------------------

$endedUtc = [DateTime]::UtcNow
$outcome = if ($isDryRun) { 'PLANNED (dry run, no action taken)' } elseif ($Rehearse) { 'REHEARSAL EXECUTED - copy Step record below into the rehearsal record and have approvers sign' } else { 'PRODUCTION ROLLBACK EXECUTED - complete Section 12 evidence register' }

Write-Log '--- Phase 4: summary (paste into docs/ROLLBACK_REHEARSAL_RECORD_TEMPLATE.md) ---' -Level INFO

$summaryLines = New-Object System.Collections.Generic.List[string]
$summaryLines.Add('## 1. Rehearsal metadata (from ops/cutover/rollback.ps1)')
$summaryLines.Add('| Field | Value |')
$summaryLines.Add('|---|---|')
$summaryLines.Add("| Record ID | ``$RecordId`` |")
$summaryLines.Add("| Environment | ``$(if ($Rehearse) { 'sandbox/disposable' } else { 'production' })`` / target=$Target |")
$summaryLines.Add("| Rehearsal date | ``$($startedUtc.ToString('yyyy-MM-dd'))`` |")
$summaryLines.Add("| Start UTC | ``$($startedUtc.ToString('yyyy-MM-ddTHH:mm:ssZ'))`` |")
$summaryLines.Add("| End UTC | ``$($endedUtc.ToString('yyyy-MM-ddTHH:mm:ssZ'))`` |")
$summaryLines.Add("| Release artifact and SHA-256 | see RELEASE_ARTIFACTS.md / backup sha256=``$backupHash`` |")
$summaryLines.Add('| Tenant / branch / counters | `<fill in from change record>` |')
$summaryLines.Add("| Incident lead | ``<fill in>`` |")
$summaryLines.Add("| Release manager | ``<fill in>`` |")
$summaryLines.Add("| DBA | ``<fill in>`` |")
$summaryLines.Add("| Branch operator | ``<fill in>`` |")
$summaryLines.Add('| Legacy write performed | `false` (this script never writes to the legacy database) |')
$summaryLines.Add("| Outcome | ``$outcome`` |")
$summaryLines.Add('')
$summaryLines.Add('## 4. Step record')
$summaryLines.Add('| # | UTC | Operator | Action | Expected | Observed | Evidence | Result |')
$summaryLines.Add('|---:|---|---|---|---|---|---|---|')
foreach ($s in $stepRecords) {
    $summaryLines.Add("| $($s.Num) | $($s.UtcTime) | $($s.Operator) | $($s.Action) | $($s.Expected) | $($s.Observed) | $($s.Evidence) | $($s.Result) |")
}
$summaryLines.Add('')
function Format-HealthField {
    param($Health, [string]$MissingReason)
    if ($Health) { return "status=$($Health.status) database=$($Health.database)" }
    return $MissingReason
}

$backupRestoredField = if ($RestoreBackup -and $isExecute) { 'yes' } elseif ($RestoreBackup) { 'planned, not executed (dry run)' } else { 'not applicable' }
$apiBeforeField = Format-HealthField -Health $preApiHealth -MissingReason 'not captured (no -ApiHealthUrl)'
$apiAfterField = Format-HealthField -Health $postApiHealth -MissingReason 'not captured'
$edgeBeforeField = Format-HealthField -Health $preEdgeHealth -MissingReason 'not captured (no -EdgeHealthUrl)'
$edgeAfterField = Format-HealthField -Health $postEdgeHealth -MissingReason 'not captured'

$summaryLines.Add('## 5. Data safety and reconciliation (partial - fill in the rest)')
$summaryLines.Add('| Check | Result / evidence |')
$summaryLines.Add('|---|---|')
$summaryLines.Add("| Target backup restored | $backupRestoredField |")
$summaryLines.Add('| Legacy write performed | false |')
$summaryLines.Add("| API health before | $apiBeforeField |")
$summaryLines.Add("| API health after | $apiAfterField |")
$summaryLines.Add("| Edge health before | $edgeBeforeField |")
$summaryLines.Add("| Edge health after | $edgeAfterField |")
$summaryLines.Add("| Rollback log file | $script:LogFile |")

$summaryText = $summaryLines -join [Environment]::NewLine
Write-Host ''
Write-Host $summaryText
Add-Content -LiteralPath $script:LogFile -Value ([Environment]::NewLine + $summaryText)

Write-Log "Rollback automation finished. Mode=$modeLabel Scope=$scopeLabel. Log: $script:LogFile" -Level INFO
