$ErrorActionPreference = 'Stop'

$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$tmp = Join-Path $root 'tmp'
$runtimeDir = Join-Path $tmp 'local-runtime'
$pgData = Join-Path $tmp 'pg-test-20260805-complete'
$services = @('web', 'edge', 'api', 'postgres')

function Get-ProcessByIdSafe {
    param([int]$ProcessId)

    if ($ProcessId -le 0) { return $null }
    return Get-CimInstance Win32_Process -Filter "ProcessId = $ProcessId" -ErrorAction SilentlyContinue
}

function Get-DescendantProcessIds {
    param([int]$ParentId)

    $children = @(Get-CimInstance Win32_Process -Filter "ParentProcessId = $ParentId" -ErrorAction SilentlyContinue)
    $ids = @()
    foreach ($childProcess in $children) {
        $ids += Get-DescendantProcessIds -ParentId $childProcess.ProcessId
        $ids += $childProcess.ProcessId
    }
    return $ids
}

function Stop-TrackedService {
    param([Parameter(Mandatory = $true)] [string]$ServiceName)

    $stopFile = Join-Path $runtimeDir "$ServiceName.stop"
    $supervisorPidFile = Join-Path $runtimeDir "$ServiceName.supervisor.pid"
    $childPidFile = Join-Path $runtimeDir "$ServiceName.child.pid"
    New-Item -ItemType File -Force -Path $stopFile | Out-Null

    $supervisorPid = 0
    if (Test-Path -LiteralPath $supervisorPidFile) {
        [void][int]::TryParse((Get-Content -LiteralPath $supervisorPidFile -TotalCount 1 -ErrorAction SilentlyContinue).Trim(), [ref]$supervisorPid)
    }
    $supervisor = Get-ProcessByIdSafe -ProcessId $supervisorPid
    if (-not $supervisor) {
        $supervisor = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
            $_.CommandLine -and
            $_.CommandLine -like '*supervise-local.ps1*' -and
            $_.CommandLine -like "*-ServiceName $ServiceName*"
        }) | Select-Object -First 1
        if ($supervisor) { $supervisorPid = $supervisor.ProcessId }
    }
    if ($supervisor -and $supervisor.CommandLine -like '*supervise-local.ps1*') {
        $deadline = [DateTime]::UtcNow.AddSeconds(8)
        while ((Get-ProcessByIdSafe -ProcessId $supervisorPid) -and [DateTime]::UtcNow -lt $deadline) {
            Start-Sleep -Milliseconds 250
        }
    }

    # The supervisor normally stops its owned child. This fallback is limited
    # to the PID it recorded, so unrelated or legacy processes are untouched.
    $childPid = 0
    if (Test-Path -LiteralPath $childPidFile) {
        [void][int]::TryParse((Get-Content -LiteralPath $childPidFile -TotalCount 1 -ErrorAction SilentlyContinue).Trim(), [ref]$childPid)
    }
    $child = Get-ProcessByIdSafe -ProcessId $childPid
    if ($child) {
        foreach ($processId in @(Get-DescendantProcessIds -ParentId $childPid)) {
            Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
        }
        Stop-Process -Id $childPid -Force -ErrorAction SilentlyContinue
    }

    $supervisor = Get-ProcessByIdSafe -ProcessId $supervisorPid
    if ($supervisor -and $supervisor.CommandLine -like '*supervise-local.ps1*') {
        Stop-Process -Id $supervisorPid -Force -ErrorAction SilentlyContinue
    }

    Remove-Item -LiteralPath $stopFile, $supervisorPidFile, $childPidFile -Force -ErrorAction SilentlyContinue
}

New-Item -ItemType Directory -Force -Path $runtimeDir | Out-Null
foreach ($service in $services) { Stop-TrackedService -ServiceName $service }

$pgctl = (Get-Command pg_ctl.exe -ErrorAction SilentlyContinue).Source
if ($pgctl -and (Test-Path -LiteralPath $pgData)) {
    & $pgctl -D $pgData status *> $null
    if ($LASTEXITCODE -eq 0) {
        & $pgctl -D $pgData stop -m fast | Out-Null
    }
}

[pscustomobject]@{
    status = 'stopped'
    project = $root
} | ConvertTo-Json
