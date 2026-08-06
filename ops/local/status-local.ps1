param(
    [switch]$IncludeCommandLine
)

$ErrorActionPreference = 'SilentlyContinue'

$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$tmp = Join-Path $root 'tmp'
$runtimeDir = Join-Path $tmp 'local-runtime'
$pgData = Join-Path $tmp 'pg-test-20260805-complete'

function Get-ProcessByIdSafe {
    param([int]$ProcessId)

    if ($ProcessId -le 0) { return $null }
    return Get-CimInstance Win32_Process -Filter "ProcessId = $ProcessId"
}

function Get-PidFromFile {
    param([string]$Path)

    $number = 0
    if (Test-Path -LiteralPath $Path) {
        [void][int]::TryParse((Get-Content -LiteralPath $Path -TotalCount 1).Trim(), [ref]$number)
    }
    return $number
}

function Get-Health {
    param([string]$Uri)

    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $Uri -TimeoutSec 2
        return [pscustomobject]@{ status = 'healthy'; httpStatus = [int]$response.StatusCode }
    } catch {
        return [pscustomobject]@{ status = 'unhealthy'; httpStatus = $null }
    }
}

function Get-ServiceStatus {
    param(
        [string]$Name,
        [string]$HealthUri
    )

    $supervisorPid = Get-PidFromFile -Path (Join-Path $runtimeDir "$Name.supervisor.pid")
    $childPid = Get-PidFromFile -Path (Join-Path $runtimeDir "$Name.child.pid")
    $supervisor = Get-ProcessByIdSafe -ProcessId $supervisorPid
    $child = Get-ProcessByIdSafe -ProcessId $childPid
    $health = Get-Health -Uri $HealthUri
    $supervisorInfo = if ($supervisor) {
        [pscustomobject]@{ pid = $supervisor.ProcessId; name = $supervisor.Name; running = $true }
    } else {
        [pscustomobject]@{ pid = $null; name = $null; running = $false }
    }
    $childInfo = if ($child) {
        $data = [ordered]@{ pid = $child.ProcessId; name = $child.Name; running = $true }
        if ($IncludeCommandLine) { $data.commandLine = $child.CommandLine }
        [pscustomobject]$data
    } else {
        [pscustomobject]@{ pid = $null; name = $null; running = $false }
    }
    return [pscustomobject]@{
        name = $Name
        health = $health
        supervisor = $supervisorInfo
        child = $childInfo
        stopRequested = Test-Path -LiteralPath (Join-Path $runtimeDir "$Name.stop")
        log = Join-Path $tmp "$Name-localhost.log"
    }
}

$pgctl = (Get-Command pg_ctl.exe -ErrorAction SilentlyContinue).Source
$postgresReady = $false
$postgresPid = Get-PidFromFile -Path (Join-Path $pgData 'postmaster.pid')
if ($pgctl -and (Test-Path -LiteralPath $pgData)) {
    & $pgctl -D $pgData status *> $null
    $postgresReady = $LASTEXITCODE -eq 0
}

$services = @(
    (Get-ServiceStatus -Name 'api' -HealthUri 'http://127.0.0.1:8080/v1/health'),
    (Get-ServiceStatus -Name 'edge' -HealthUri 'http://127.0.0.1:8091/v1/health'),
    (Get-ServiceStatus -Name 'web' -HealthUri 'http://127.0.0.1:5173/login')
)

[pscustomobject]@{
    project = $root
    postgres = [pscustomobject]@{
        health = if ($postgresReady) { 'healthy' } else { 'unhealthy' }
        running = $postgresReady
        pid = if ($postgresReady) { $postgresPid } else { $null }
        dataDirectory = $pgData
    }
    services = $services
} | ConvertTo-Json -Depth 6
