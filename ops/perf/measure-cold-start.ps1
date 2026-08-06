param(
    [string]$DatabaseDsn = $env:ABUZAR_PERF_DATABASE_URL,
    [int]$Port = 18080,
    [string]$OutputPath = ''
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
if ([string]::IsNullOrWhiteSpace($DatabaseDsn)) {
    throw 'Provide -DatabaseDsn or ABUZAR_PERF_DATABASE_URL.'
}
if ([string]::IsNullOrWhiteSpace($OutputPath)) {
    $OutputPath = Join-Path $root ('tmp\phase-w-cold-start-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.json')
}

$binary = Join-Path $root 'tmp\phase-w-api.exe'
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $binary) | Out-Null
& go build -o $binary ./services/api/cmd/server
if ($LASTEXITCODE -ne 0) { throw 'Unable to build the API cold-start probe binary.' }

$oldDatabase = $env:DATABASE_URL
$oldAddress = $env:ABUZAR_API_ADDR
$env:DATABASE_URL = $DatabaseDsn
$env:ABUZAR_API_ADDR = "127.0.0.1:$Port"
$process = $null
$watch = [Diagnostics.Stopwatch]::StartNew()
try {
    $process = Start-Process -FilePath $binary -WorkingDirectory $root -WindowStyle Hidden -PassThru
    $healthy = $false
    $deadline = [DateTime]::UtcNow.AddSeconds(30)
    while ([DateTime]::UtcNow -lt $deadline) {
        Start-Sleep -Milliseconds 100
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/v1/health" -TimeoutSec 1
            if ($response.StatusCode -eq 200) {
                $healthy = $true
                break
            }
        } catch {
            if ($process.HasExited) { break }
        }
    }
    $watch.Stop()
    $result = [pscustomobject]@{
        measuredAt = [DateTime]::UtcNow.ToString('o')
        coldStartMs = $watch.Elapsed.TotalMilliseconds
        healthy = $healthy
        budgetMs = 3000
        acceptance = if ($healthy -and $watch.Elapsed.TotalMilliseconds -lt 3000) { 'observed_green_local_probe' } else { 'pending_review' }
        database = 'DSN omitted'
    }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $OutputPath) | Out-Null
    $result | ConvertTo-Json | Set-Content -LiteralPath $OutputPath -Encoding UTF8
    $result | ConvertTo-Json
}
finally {
    if ($process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        $process.WaitForExit()
    }
    if ($null -eq $oldDatabase) { Remove-Item Env:DATABASE_URL -ErrorAction SilentlyContinue } else { $env:DATABASE_URL = $oldDatabase }
    if ($null -eq $oldAddress) { Remove-Item Env:ABUZAR_API_ADDR -ErrorAction SilentlyContinue } else { $env:ABUZAR_API_ADDR = $oldAddress }
    Remove-Item -LiteralPath $binary -Force -ErrorAction SilentlyContinue
}
