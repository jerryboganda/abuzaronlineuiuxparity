param(
    [Parameter(Mandatory = $true)] [string]$ServiceName,
    [Parameter(Mandatory = $true)] [ValidateSet('process', 'postgres')] [string]$Kind,
    [string]$FilePath,
    [Parameter(Mandatory = $true)] [string]$WorkingDirectory,
    [string]$Arguments = '',
    [string]$HealthUri = '',
    [string]$PgCtlPath = '',
    [string]$PgData = '',
    [Parameter(Mandatory = $true)] [string]$LogPath,
    [Parameter(Mandatory = $true)] [string]$RuntimeDirectory,
    [int]$MaxLogBytes = 5242880,
    [int]$LogGenerations = 5
)

$ErrorActionPreference = 'Continue'

New-Item -ItemType Directory -Force -Path $RuntimeDirectory, (Split-Path $LogPath) | Out-Null
$pidPath = Join-Path $RuntimeDirectory "$ServiceName.supervisor.pid"
$childPidPath = Join-Path $RuntimeDirectory "$ServiceName.child.pid"
$stopPath = Join-Path $RuntimeDirectory "$ServiceName.stop"
$logLock = New-Object object

function Rotate-Log {
    param([Parameter(Mandatory = $true)] [string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) { return }
    for ($index = $LogGenerations - 1; $index -ge 1; $index--) {
        $source = "$Path.$index"
        $destination = "$Path.$($index + 1)"
        if (Test-Path -LiteralPath $source) {
            Move-Item -LiteralPath $source -Destination $destination -Force -ErrorAction SilentlyContinue
        }
    }
    Move-Item -LiteralPath $Path -Destination "$Path.1" -Force -ErrorAction SilentlyContinue
}

function Write-LogLine {
    param(
        [Parameter(Mandatory = $true)] [string]$Path,
        [Parameter(Mandatory = $true)] [string]$Line
    )

    [System.Threading.Monitor]::Enter($logLock)
    try {
        if ((Test-Path -LiteralPath $Path) -and ((Get-Item -LiteralPath $Path).Length -ge $MaxLogBytes)) {
            Rotate-Log -Path $Path
        }
        Add-Content -LiteralPath $Path -Value ("{0:u} {1}" -f (Get-Date), $Line) -Encoding UTF8
    } catch {
        # Logging must never take down the service supervisor.
    } finally {
        [System.Threading.Monitor]::Exit($logLock)
    }
}

function Test-Healthy {
    param([string]$Uri)

    if ([string]::IsNullOrWhiteSpace($Uri)) { return $false }
    if ($Uri -eq 'postgres://local') {
        & $PgCtlPath -D $PgData status *> $null
        return $LASTEXITCODE -eq 0
    }
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $Uri -TimeoutSec 2
        return $response.StatusCode -eq 200
    } catch {
        return $false
    }
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

function Stop-Child {
param([System.Diagnostics.Process]$Process)

if ($Process) {
    $descendants = @(Get-DescendantProcessIds -ParentId $Process.Id)
    foreach ($processId in $descendants) {
        Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
    }
    if (-not $Process.HasExited) {
        Stop-Process -Id $Process.Id -Force -ErrorAction SilentlyContinue
        $Process.WaitForExit(5000)
    }
}
}

function Start-ManagedProcess {
    Rotate-Log -Path $LogPath
    $errorLogPath = "$LogPath.stderr"
    Rotate-Log -Path $errorLogPath
    $startParameters = @{
        FilePath = $FilePath
        WorkingDirectory = $WorkingDirectory
        WindowStyle = 'Hidden'
        RedirectStandardOutput = $LogPath
        RedirectStandardError = $errorLogPath
        PassThru = $true
    }
    if (-not [string]::IsNullOrWhiteSpace($Arguments)) {
        $startParameters.ArgumentList = $Arguments
    }
    $process = Start-Process @startParameters
    if (-not $process) { throw "Unable to start $FilePath" }
    Set-Content -LiteralPath $childPidPath -Value $process.Id -Encoding ASCII
    return $process
}

$mutex = New-Object System.Threading.Mutex($false, "Local\AbuzarNext.$ServiceName")
$ownsMutex = $false
$child = $null
try {
    $ownsMutex = $mutex.WaitOne(0)
    if (-not $ownsMutex) { exit 0 }
    Set-Content -LiteralPath $pidPath -Value $PID -Encoding ASCII
    Write-LogLine -Path $LogPath -Line "supervisor started service=$ServiceName kind=$Kind"

    if ($Kind -eq 'postgres') {
        while (-not (Test-Path -LiteralPath $stopPath)) {
            if (-not (Test-Healthy -Uri $HealthUri)) {
                Rotate-Log -Path $LogPath
                & $PgCtlPath -D $PgData -l $LogPath start 2>&1 | ForEach-Object {
                    Write-LogLine -Path $LogPath -Line ([string]$_)
                }
                if ($LASTEXITCODE -ne 0) {
                    Write-LogLine -Path $LogPath -Line "postgres start failed exit=$LASTEXITCODE"
                } else {
                    Write-LogLine -Path $LogPath -Line 'postgres started'
                }
            }
            Start-Sleep -Seconds 2
        }
        if (Test-Healthy -Uri $HealthUri) {
            & $PgCtlPath -D $PgData stop -m fast 2>&1 | ForEach-Object {
                Write-LogLine -Path $LogPath -Line ([string]$_)
            }
        }
    } else {
        $restartDelay = 1
        while (-not (Test-Path -LiteralPath $stopPath)) {
            if (-not $child) {
                if (Test-Healthy -Uri $HealthUri) {
                    Write-LogLine -Path $LogPath -Line "adopting healthy existing process at $HealthUri"
                    while (-not (Test-Path -LiteralPath $stopPath) -and (Test-Healthy -Uri $HealthUri)) {
                        Start-Sleep -Seconds 2
                    }
                    if (Test-Path -LiteralPath $stopPath) { break }
                    Write-LogLine -Path $LogPath -Line 'adopted process became unhealthy; starting managed process'
                }
                try {
                    $child = Start-ManagedProcess
                    Write-LogLine -Path $LogPath -Line "started pid=$($child.Id) file=$FilePath"
                    $restartDelay = 1
                } catch {
                    Write-LogLine -Path $LogPath -Line ("start failed: " + $_.Exception.Message)
                    Start-Sleep -Seconds $restartDelay
                    $restartDelay = [Math]::Min($restartDelay * 2, 30)
                    continue
                }
            }

            while ($child -and -not $child.HasExited -and -not (Test-Path -LiteralPath $stopPath)) {
                Start-Sleep -Milliseconds 500
            }
            if ($child -and $child.HasExited) {
                $exitCode = $child.ExitCode
                $child.WaitForExit()
                Write-LogLine -Path $LogPath -Line "process exited pid=$($child.Id) exit=$exitCode"
                $child.Dispose()
                $child = $null
                Remove-Item -LiteralPath $childPidPath -Force -ErrorAction SilentlyContinue
                if (-not (Test-Path -LiteralPath $stopPath)) {
                    Start-Sleep -Seconds $restartDelay
                    $restartDelay = [Math]::Min($restartDelay * 2, 30)
                }
            }
        }
    }
} catch {
    Write-LogLine -Path $LogPath -Line ("supervisor failure: " + $_.Exception.Message)
} finally {
    Stop-Child -Process $child
    Remove-Item -LiteralPath $childPidPath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $pidPath -Force -ErrorAction SilentlyContinue
    if ($ownsMutex) { [void]$mutex.ReleaseMutex() }
    $mutex.Dispose()
}
