param(
    [switch]$OpenBrowser,
    [switch]$SkipWeb
)

$ErrorActionPreference = 'Stop'

$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$tmp = Join-Path $root 'tmp'
$runtimeDir = Join-Path $tmp 'local-runtime'
$pgData = Join-Path $tmp 'pg-test-20260805-complete'
$supervisor = Join-Path $PSScriptRoot 'supervise-local.ps1'
$powershell = Join-Path $PSHOME 'powershell.exe'

New-Item -ItemType Directory -Force -Path $tmp, $runtimeDir | Out-Null

function Quote-ProcessArgument {
    param([string]$Value)

    if ($Value.Length -eq 0) { return '""' }
    if ($Value -notmatch '[\s"]') { return $Value }
    return '"' + $Value.Replace('"', '\"') + '"'
}

function Get-ProcessByIdSafe {
    param([int]$ProcessId)

    if ($ProcessId -le 0) { return $null }
    return Get-CimInstance Win32_Process -Filter "ProcessId = $ProcessId" -ErrorAction SilentlyContinue
}

function Get-TrackedSupervisor {
    param([Parameter(Mandatory = $true)] [string]$ServiceName)

    $pidPath = Join-Path $runtimeDir "$ServiceName.supervisor.pid"
    if (Test-Path -LiteralPath $pidPath) {
        $pidText = (Get-Content -LiteralPath $pidPath -TotalCount 1 -ErrorAction SilentlyContinue).Trim()
        $pidNumber = 0
        if ([int]::TryParse($pidText, [ref]$pidNumber)) {
            $process = Get-ProcessByIdSafe -ProcessId $pidNumber
            if ($process -and $process.CommandLine -like "*supervise-local.ps1*") {
                return $process
            }
        }
        Remove-Item -LiteralPath $pidPath -Force -ErrorAction SilentlyContinue
    }

    $processes = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
        $_.CommandLine -and
        $_.CommandLine -like "*supervise-local.ps1*" -and
        $_.CommandLine -like "*-ServiceName $ServiceName*"
    }
    if ($processes) {
        $process = @($processes)[0]
        Set-Content -LiteralPath $pidPath -Value $process.ProcessId -Encoding ASCII
        return $process
    }
    return $null
}

function Start-Supervisor {
    param(
        [Parameter(Mandatory = $true)] [string]$ServiceName,
        [Parameter(Mandatory = $true)] [ValidateSet('process', 'postgres')] [string]$Kind,
        [string]$FilePath,
        [string]$WorkingDirectory,
        [string]$Arguments = '',
        [string]$HealthUri = '',
        [string]$PgCtlPath = '',
        [string]$PgData = '',
        [Parameter(Mandatory = $true)] [string]$LogPath
    )

    $existing = Get-TrackedSupervisor -ServiceName $ServiceName
    if ($existing) {
        return [pscustomobject]@{ service = $ServiceName; state = 'already-supervised'; pid = $existing.ProcessId }
    }
    Remove-Item -LiteralPath (Join-Path $runtimeDir "$ServiceName.stop") -Force -ErrorAction SilentlyContinue

    $argumentList = @(
        '-NoProfile',
        '-ExecutionPolicy', 'Bypass',
        '-File', $supervisor,
        '-ServiceName', $ServiceName,
        '-Kind', $Kind,
        '-WorkingDirectory', $WorkingDirectory,
        '-Arguments', $Arguments,
        '-HealthUri', $HealthUri,
        '-PgCtlPath', $PgCtlPath,
        '-PgData', $PgData,
        '-LogPath', $LogPath,
        '-RuntimeDirectory', $runtimeDir
    )
    if ($FilePath) { $argumentList += @('-FilePath', $FilePath) }

    $serializedArguments = ($argumentList | ForEach-Object { Quote-ProcessArgument -Value ([string]$_) }) -join ' '
    $started = Start-Process -FilePath $powershell -WorkingDirectory $root `
        -ArgumentList $serializedArguments -WindowStyle Hidden -PassThru

    $deadline = [DateTime]::UtcNow.AddSeconds(5)
    do {
        Start-Sleep -Milliseconds 100
        $tracked = Get-TrackedSupervisor -ServiceName $ServiceName
    } while (-not $tracked -and [DateTime]::UtcNow -lt $deadline)

    if (-not $tracked) {
        if ($started -and -not $started.HasExited) {
            Stop-Process -Id $started.Id -Force -ErrorAction SilentlyContinue
        }
        throw "Unable to start the $ServiceName supervisor."
    }
    return [pscustomobject]@{ service = $ServiceName; state = 'started'; pid = $tracked.ProcessId }
}

function Test-HttpOk {
    param([Parameter(Mandatory = $true)] [string]$Uri)

    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $Uri -TimeoutSec 2
        return $response.StatusCode -eq 200
    } catch {
        return $false
    }
}

function Test-PostgresReady {
    param([Parameter(Mandatory = $true)] [string]$PgCtlPath)

    & $PgCtlPath -D $pgData status *> $null
    return $LASTEXITCODE -eq 0
}

$startMutex = New-Object System.Threading.Mutex($false, 'Local\AbuzarNext.LocalStart')
$hasStartMutex = $false
try {
    $hasStartMutex = $startMutex.WaitOne(30000)
    if (-not $hasStartMutex) { throw 'Another local stack start is already in progress.' }

    $pgctl = (Get-Command pg_ctl.exe -ErrorAction SilentlyContinue).Source
    if ([string]::IsNullOrWhiteSpace($pgctl)) {
        throw 'pg_ctl.exe was not found on PATH. Install PostgreSQL or add its bin directory to PATH.'
    }
    if (-not (Test-Path -LiteralPath $pgData)) {
        throw "PostgreSQL data directory does not exist: $pgData"
    }
    if (-not (Test-Path -LiteralPath $supervisor)) {
        throw "Missing local supervisor: $supervisor"
    }

    $api = Join-Path $tmp 'abuzar-api-localhost.exe'
    $edge = Join-Path $tmp 'abuzar-edge-localhost.exe'
    if (-not (Test-Path -LiteralPath $api)) { throw "Missing API binary: $api" }
    if (-not (Test-Path -LiteralPath $edge)) { throw "Missing edge binary: $edge" }

    $startedServices = @()
    $startedServices += Start-Supervisor -ServiceName 'postgres' -Kind 'postgres' `
        -WorkingDirectory $root -PgCtlPath $pgctl -PgData $pgData `
        -HealthUri 'postgres://local' -LogPath (Join-Path $tmp 'postgres-localhost.log')

    $pgDeadline = [DateTime]::UtcNow.AddSeconds(30)
    while (-not (Test-PostgresReady -PgCtlPath $pgctl) -and [DateTime]::UtcNow -lt $pgDeadline) {
        Start-Sleep -Milliseconds 500
    }
    if (-not (Test-PostgresReady -PgCtlPath $pgctl)) {
        throw 'PostgreSQL did not become ready within 30 seconds.'
    }

    # Provision the non-owner local API role before starting the API. The
    # schema-owner DSN is used only by this setup command and is removed before
    # any service supervisor is started.
    $env:ABUZAR_ADMIN_DATABASE_URL = 'postgres://postgres@127.0.0.1:5432/abuzar_next?sslmode=disable'
    $env:ABUZAR_APP_ROLE = 'abuzar_app_local'
    $provisionRole = Join-Path $root 'ops\postgres\provision-app-role.ps1'
    & $powershell -NoProfile -ExecutionPolicy Bypass -File $provisionRole
    if ($LASTEXITCODE -ne 0) {
        throw 'Unable to provision the local non-owner application role.'
    }
    $env:ABUZAR_APP_DATABASE_URL = 'postgres://abuzar_app_local@127.0.0.1:5432/abuzar_next?sslmode=disable'
    $env:DATABASE_URL = $env:ABUZAR_APP_DATABASE_URL
    Remove-Item Env:ABUZAR_ADMIN_DATABASE_URL -ErrorAction SilentlyContinue
    $env:ABUZAR_API_ADDR = '127.0.0.1:8080'
    $env:ABUZAR_CORS_ORIGINS = 'http://127.0.0.1:5173,http://localhost:5173'
    $env:ABUZAR_COOKIE_SECURE = '0'
    $startedServices += Start-Supervisor -ServiceName 'api' -Kind 'process' `
        -FilePath $api -WorkingDirectory $root -HealthUri 'http://127.0.0.1:8080/v1/health' `
        -LogPath (Join-Path $tmp 'api-localhost.log')

    $env:ABUZAR_EDGE_ADDR = '127.0.0.1:8091'
    $env:ABUZAR_EDGE_DB = (Join-Path $tmp 'data\branch-edge-local.sqlite')
    $startedServices += Start-Supervisor -ServiceName 'edge' -Kind 'process' `
        -FilePath $edge -WorkingDirectory $root -HealthUri 'http://127.0.0.1:8091/v1/health' `
        -LogPath (Join-Path $tmp 'edge-localhost.log')

    if (-not $SkipWeb) {
        $webDirectory = Join-Path $root 'apps\web'
        $cmd = (Get-Command cmd.exe -ErrorAction Stop).Source
        $startedServices += Start-Supervisor -ServiceName 'web' -Kind 'process' `
            -FilePath $cmd -WorkingDirectory $webDirectory `
            -Arguments '/d /c pnpm dev -- --host 127.0.0.1 --port 5173' `
            -HealthUri 'http://127.0.0.1:5173/login' `
            -LogPath (Join-Path $tmp 'web-localhost.log')
    }

    $deadline = [DateTime]::UtcNow.AddSeconds(30)
    do {
        Start-Sleep -Milliseconds 500
        $ready = (Test-HttpOk 'http://127.0.0.1:8080/v1/health') -and
            (Test-HttpOk 'http://127.0.0.1:8091/v1/health')
        if (-not $SkipWeb) { $ready = $ready -and (Test-HttpOk 'http://127.0.0.1:5173/login') }
    } while (-not $ready -and [DateTime]::UtcNow -lt $deadline)
    if (-not $ready) { throw 'The local stack did not become healthy within 30 seconds.' }

    if ($OpenBrowser -and -not $SkipWeb) {
        Start-Process 'http://127.0.0.1:5173/login'
    }

    [pscustomobject]@{
        status = 'ok'
        project = $root
        services = $startedServices
        web = if ($SkipWeb) { 'skipped' } else { 'http://127.0.0.1:5173/login' }
        api = 'http://127.0.0.1:8080/v1/health'
        edge = 'http://127.0.0.1:8091/v1/health'
        postgres = '127.0.0.1:5432'
    } | ConvertTo-Json -Depth 4
} finally {
    if ($hasStartMutex) { [void]$startMutex.ReleaseMutex() }
    $startMutex.Dispose()
}
